import { Component, Input, Output, EventEmitter, OnInit, OnChanges } from '@angular/core';
import { CommonModule } from '@angular/common';
import { TabBarComponent } from './tab-bar.component';
import { AddressBarComponent } from './address-bar.component';
import { FilterBarComponent } from './filter-bar.component';
import { ActionBarComponent } from './action-bar.component';
import { FileListComponent } from './file-list.component';
import { FileInfo, FileType } from '../shared/protocol';
import { TabInfo, WorkspaceConfig, FilterQuery } from '../shared/protocol-enhanced';
import { ApiService } from '../services/api.service';
import { WorkspaceService } from '../services/workspace.service';
import { FilterService } from '../services/filter.service';
import { PathHistoryService } from '../services/path-history.service';

// Archive types
import { ArchiveService, ArchiveEntry, ArchiveListing, isArchive, archiveFormat } from '../services/archive.service';

declare const window: any;

/** One level in the archive navigation stack. */
interface ArchiveStackFrame {
  archivePath: string;
  innerPath: string;
  listing: ArchiveListing;
}

/**
 * File Pane Component
 * Complete reusable file browser pane
 * Combines tabs, address bar, filter, actions, and file list
 */
@Component({
  selector: 'app-file-pane',
  standalone: true,
  imports: [
    CommonModule,
    TabBarComponent,
    AddressBarComponent,
    FilterBarComponent,
    ActionBarComponent,
    FileListComponent
  ],
  template: `
    <div class="pane" [class.archive-mode]="isInArchive()">
      <app-tab-bar
        [tabs]="tabs"
        [activeTabId]="currentTabId"
        (tabChange)="onTabChange($event)"
        (tabClose)="onTabClose($event)"
        (tabPin)="onTabPin($event)"
        (newTab)="onNewTab()">
      </app-tab-bar>

      <app-address-bar
        [initialPath]="currentPath"
        [paneId]="paneId"
        (pathChange)="onPathChange($event)"
        (navigateUpClicked)="onNavigateUp()"
        (refreshClicked)="onRefresh()">
      </app-address-bar>

      <app-filter-bar
        [totalCount]="entries.length"
        [filteredCount]="filteredEntries.length"
        (filterChange)="onFilterChange($event)"
        (clear)="onClearFilter()">
      </app-filter-bar>

      <app-action-bar
        [selectedCount]="selectedFiles.size"
        [panePosition]="paneId"
        (createFolder)="onCreateFolder()"
        (deleteSelected)="onDeleteSelected()"
        (copyToOther)="onCopyToOther()"
        (moveToOther)="onMoveToOther()">
      </app-action-bar>

      <app-file-list
        [entries]="filteredEntries"
        [selectedFiles]="selectedFiles"
        [loading]="loading"
        [error]="error"
        (selectionChange)="onSelectionChange($event)"
        (navigate)="onNavigate($event)">
      </app-file-list>
    </div>
  `,
  styles: [`
    .pane {
      flex: 1;
      display: flex;
      flex-direction: column;
      background: var(--vsc-editor-background);
      overflow: hidden;
      min-height: 0;
      min-width: 0;
      width: 100%;
    }
    .pane.archive-mode {
      background: var(--vsc-editor-background, #1e1e1e);
    }
  `]
})
export class FilePaneComponent implements OnInit, OnChanges {
  @Input() paneId: 'left' | 'right' = 'left';
  @Input() workspace: WorkspaceConfig | null = null;
  @Input() showHidden = false;

  @Output() pathChange = new EventEmitter<string>();
  @Output() copyToOther = new EventEmitter<string[]>();
  @Output() moveToOther = new EventEmitter<string[]>();

  // State
  currentPath = '';
  currentTabId = '';
  tabs: TabInfo[] = [];
  entries: FileInfo[] = [];
  filteredEntries: FileInfo[] = [];
  selectedFiles: Set<string> = new Set();
  loading = false;
  error: string | null = null;
  filterText = '';
  filterQuery: FilterQuery = { text: '', criteria: [] };

  // Archive state
  archiveStack: ArchiveStackFrame[] = [];
  currentArchiveListing: ArchiveListing | null = null;

  // Expose to template
  FileType = FileType;
  isArchive = isArchive;

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private filterService: FilterService,
    private pathHistoryService: PathHistoryService,
    private archiveService: ArchiveService
  ) {}

  ngOnInit(): void {
    this.loadPaneState();
  }

  ngOnChanges(): void {
    if (this.workspace) {
      this.loadPaneState();
    }
  }

  // ============================================================================
  // Initialization
  // ============================================================================

  private async loadPaneState(): Promise<void> {
    if (!this.workspace) return;

    this.tabs = this.workspaceService.getTabs(this.paneId);
    const activeTab = this.workspaceService.getActiveTab(this.paneId);

    if (activeTab && activeTab.path) {
      this.currentTabId = activeTab.id;
      this.currentPath = activeTab.path;
      await this.loadDirectory(activeTab.path);
    } else {
      // No saved tab — ask Go for the platform-correct initial path
      try {
        const initialPath: string = await window.go.main.App.GetInitialPath();
        this.currentPath = initialPath;
        await this.loadDirectory(initialPath);
      } catch (err) {
        console.error('Failed to get initial path:', err);
      }
    }
  }

  // ============================================================================
  // Directory Loading
  // ============================================================================

  private async loadDirectory(path: string): Promise<void> {
    // Entering a real directory always exits any archive context
    this.exitArchive();

    this.loading = true;
    this.error = null;

    try {
      const listing = await this.apiService.listDirectory(path, this.showHidden);
      this.currentPath = listing.path;
      this.entries = listing.entries;
      this.selectedFiles.clear();

      this.applyFilter();
      this.workspaceService.updateTabPath(this.paneId, this.currentTabId, listing.path);
      this.pathHistoryService.addPath(listing.path, this.paneId);
      this.pathChange.emit(listing.path);
    } catch (error: any) {
      this.error = error.message || 'Failed to load directory';
      console.error('Failed to load directory:', error);
    } finally {
      this.loading = false;
    }
  }

  // ============================================================================
  // Tab Management
  // ============================================================================

  onTabChange(tab: TabInfo): void {
    this.workspaceService.switchTab(this.paneId, tab.id);
    this.currentTabId = tab.id;
    this.currentPath = tab.path;
    this.exitArchive();
    this.loadDirectory(tab.path);
  }

  onTabClose(tab: TabInfo): void {
    this.workspaceService.closeTab(this.paneId, tab.id);
    this.loadPaneState();
  }

  onTabPin(tab: TabInfo): void {
    this.workspaceService.toggleTabPin(this.paneId, tab.id);
    this.loadPaneState();
  }

  onNewTab(): void {
    const newTab = this.workspaceService.addTab(this.paneId, this.currentPath, true);
    if (newTab) {
      this.loadPaneState();
    }
  }

  // Tab navigation for keyboard shortcuts
  onCloseCurrentTab(): void {
    const activeTab = this.tabs.find(t => t.isActive);
    if (activeTab) {
      this.onTabClose(activeTab);
    }
  }

  onNextTab(): void {
    const i = this.tabs.findIndex(t => t.isActive);
    if (i >= 0) {
      const nextIndex = (i + 1) % this.tabs.length;
      this.onTabChange(this.tabs[nextIndex]);
    }
  }

  onPreviousTab(): void {
    const i = this.tabs.findIndex(t => t.isActive);
    if (i >= 0) {
      const prevIndex = i === 0 ? this.tabs.length - 1 : i - 1;
      this.onTabChange(this.tabs[prevIndex]);
    }
  }

  onJumpToTab(index: number): void {
    if (index > 0 && index <= this.tabs.length) {
      this.onTabChange(this.tabs[index - 1]);
    }
  }

  onTogglePinCurrentTab(): void {
    const activeTab = this.tabs.find(t => t.isActive);
    if (activeTab) {
      this.onTabPin(activeTab);
    }
  }

  // ============================================================================
  // Navigation
  // ============================================================================

  onPathChange(path: string): void {
    if (path && path !== this.currentPath) {
      this.exitArchive();
      this.loadDirectory(path);
    }
  }

  /**
   * Navigate up to parent directory.
   * Delegates entirely to Go (NavGoUp) so path logic is platform-correct
   * for both Unix "/" and Windows "C:\" roots.
   */
  async onNavigateUp(): Promise<void> {
    if (this.isInArchive()) {
      await this.navigateArchiveUp();
    } else {
      try {
        const listing = await window.go.main.App.NavGoUp(this.paneId);
        this.currentPath = listing.path;
        this.entries = listing.entries;
        this.selectedFiles.clear();
        this.applyFilter();
        this.workspaceService.updateTabPath(this.paneId, this.currentTabId, listing.path);
        this.pathHistoryService.addPath(listing.path, this.paneId);
        this.pathChange.emit(listing.path);
      } catch {
        // Already at root — silently ignore
      }
    }
  }

  onRefresh(): void {
    if (this.isInArchive()) {
      this.refreshArchive();
    } else {
      this.loadDirectory(this.currentPath);
    }
  }

  onNavigate(entry: FileInfo): void {
    // Special sentinel: '..' inside an archive
    if (entry.path === '__archive_parent__') {
      this.navigateArchiveUp();
      return;
    }

    // Inside an archive
    if (this.isInArchive()) {
      const frame = this.archiveStack[this.archiveStack.length - 1];
      if (entry.type === FileType.DIRECTORY) {
        this.loadArchive(frame.archivePath, entry.path);
      }
      // Files inside archives are read-only — no action
      return;
    }

    // Normal filesystem: directory
    if (entry.type === FileType.DIRECTORY) {
      if (entry.name === '..') {
        this.onNavigateUp();
      } else {
        this.loadDirectory(entry.path);
      }
      return;
    }

    // Normal filesystem: archive file → open as folder
    if (entry.type === FileType.FILE && isArchive(entry.name)) {
      this.loadArchive(entry.path);
      return;
    }
  }

  onOpenSelectedItem(): void {
    const selected = Array.from(this.selectedFiles);
    if (selected.length === 1) {
      const entry = this.entries.find(e => e.path === selected[0]);
      if (entry) {
        this.onNavigate(entry);
      }
    }
  }

  // ============================================================================
  // Archive Navigation
  // ============================================================================

  isInArchive(): boolean {
    return this.archiveStack.length > 0;
  }

  async loadArchive(archivePath: string, innerPath = ''): Promise<void> {
    this.loading = true;
    this.error = null;

    try {
      const listing = await this.archiveService.listArchive(archivePath, innerPath);
      if (!listing) throw new Error('Empty response from server');

      const frame: ArchiveStackFrame = { archivePath, innerPath, listing };
      this.archiveStack.push(frame);
      this.currentArchiveListing = listing;

      this.entries = this.archiveListingToFileInfos(listing);
      this.selectedFiles.clear();
      this.applyFilter();
    } catch (error: any) {
      this.error = error.message || 'Failed to open archive';
    } finally {
      this.loading = false;
    }
  }

  async navigateArchiveUp(): Promise<void> {
    this.archiveStack.pop();

    if (this.archiveStack.length === 0) {
      // Back on the real filesystem
      this.currentArchiveListing = null;
      await this.loadDirectory(this.currentPath);
      return;
    }

    const frame = this.archiveStack[this.archiveStack.length - 1];
    this.currentArchiveListing = frame.listing;
    this.entries = this.archiveListingToFileInfos(frame.listing);
    this.selectedFiles.clear();
    this.applyFilter();
  }

  exitArchive(): void {
    this.archiveStack = [];
    this.currentArchiveListing = null;
  }

  private async refreshArchive(): Promise<void> {
    const frame = this.archiveStack[this.archiveStack.length - 1];
    this.archiveStack.pop();
    await this.loadArchive(frame.archivePath, frame.innerPath);
  }

  getArchiveBreadcrumb(): { label: string; frameIndex: number }[] {
    return this.archiveStack.map((frame, i) => {
      const archiveName = frame.archivePath.split(/[\\/]/).pop() ?? frame.archivePath;
      if (i === 0) {
        return { label: archiveName, frameIndex: i };
      }
      const segment = frame.innerPath.split('/').pop() ?? frame.innerPath;
      return { label: segment, frameIndex: i };
    });
  }

  async jumpToArchiveFrame(frameIndex: number): Promise<void> {
    this.archiveStack = this.archiveStack.slice(0, frameIndex + 1);
    const frame = this.archiveStack[this.archiveStack.length - 1];
    this.currentArchiveListing = frame.listing;
    this.entries = this.archiveListingToFileInfos(frame.listing);
    this.selectedFiles.clear();
    this.applyFilter();
  }

  getCurrentArchiveFormat(): string | null {
    return this.currentArchiveListing?.format ?? null;
  }

  getCompressionRatio(entry: FileInfo): string | null {
    if (!this.isInArchive() || !this.currentArchiveListing) return null;
    const archiveEntry = this.currentArchiveListing.entries.find(e => e.innerPath === entry.path);
    if (!archiveEntry || archiveEntry.size === 0 || archiveEntry.compressedSize === 0) return null;
    const ratio = Math.round((1 - archiveEntry.compressedSize / archiveEntry.size) * 100);
    return ratio > 0 ? `${ratio}%` : null;
  }

  getCompressionMethod(entry: FileInfo): string | null {
    if (!this.isInArchive() || !this.currentArchiveListing) return null;
    const archiveEntry = this.currentArchiveListing.entries.find(e => e.innerPath === entry.path);
    return archiveEntry?.compression ?? null;
  }

  async onExtractSelected(): Promise<void> {
    if (!this.isInArchive()) return;

    const frame = this.archiveStack[this.archiveStack.length - 1];
    const destination = prompt('Extract to directory:', this.currentPath);
    if (!destination) return;

    const selected = Array.from(this.selectedFiles);
    const innerPaths = selected.length > 0 ? selected : [];

    this.loading = true;
    try {
      await this.archiveService.extractArchive(frame.archivePath, destination, innerPaths);
      alert(`Extracted successfully to ${destination}`);
    } catch (error: any) {
      alert(`Extraction failed: ${error.message}`);
    } finally {
      this.loading = false;
    }
  }

  private archiveListingToFileInfos(listing: ArchiveListing): FileInfo[] {
    const entries: FileInfo[] = listing.entries.map(e => ({
      name: e.name,
      path: e.innerPath,
      type: e.type === 'DIRECTORY' ? FileType.DIRECTORY : FileType.FILE,
      size: e.size,
      modified: e.modified,
      permissions: e.type === 'DIRECTORY' ? 'drwxr-xr-x' : '-rw-r--r--',
      isHidden: e.name.startsWith('.'),
      created: e.modified,
      accessed: e.modified,
    }));

    // Add '..' entry
    const parentEntry: FileInfo = {
      name: '..',
      path: '__archive_parent__',
      type: FileType.DIRECTORY,
      size: 0,
      modified: 0,
      permissions: 'drwxr-xr-x',
      isHidden: false,
      created: 0,
      accessed: 0,
    };

    return [parentEntry, ...entries];
  }

  // ============================================================================
  // Filtering
  // ============================================================================

  onFilterChange(state: any): void {
    if (state && typeof state.combined === 'string') {
      this.filterText = state.combined;
      this.filterQuery = this.filterService.parseFilterText(state.combined);
      this.applyFilter();
    }
  }

  onClearFilter(): void {
    this.filterText = '';
    this.filterQuery = { text: '', criteria: [] };
    this.applyFilter();
  }

  private applyFilter(): void {
    if (!this.filterText || this.filterText.trim() === '') {
      this.filteredEntries = this.entries;
    } else {
      this.filteredEntries = this.filterService.filterEntries(this.entries, this.filterQuery);
    }
  }

  // ============================================================================
  // Selection
  // ============================================================================

  onSelectionChange(entry: FileInfo): void {
    if (this.selectedFiles.has(entry.path)) {
      this.selectedFiles.delete(entry.path);
    } else {
      this.selectedFiles.add(entry.path);
    }
    this.selectedFiles = new Set(this.selectedFiles); // trigger change detection
  }

  onSelectAll(): void {
    this.filteredEntries.forEach(e => this.selectedFiles.add(e.path));
    this.selectedFiles = new Set(this.selectedFiles);
  }

  onDeselectAll(): void {
    this.selectedFiles.clear();
    this.selectedFiles = new Set(this.selectedFiles);
  }

  // ============================================================================
  // File Operations
  // ============================================================================

  async onCreateFolder(): Promise<void> {
    if (this.isInArchive()) {
      alert('Cannot create folders inside an archive.');
      return;
    }

    const folderName = prompt('Enter folder name:');
    if (!folderName) return;

    try {
      const newPath: string = await window.go.main.App.JoinPaths(
        [this.currentPath, folderName]
      );
      await this.apiService.createDirectory(newPath, false);
      await this.loadDirectory(this.currentPath);
    } catch (error: any) {
      alert(`Failed to create folder: ${error.message}`);
    }
  }

  async onDeleteSelected(): Promise<void> {
    if (this.isInArchive()) {
      alert('Cannot delete files inside an archive.');
      return;
    }

    if (this.selectedFiles.size === 0) return;
    if (!confirm(`Delete ${this.selectedFiles.size} item(s)?`)) return;

    for (const path of Array.from(this.selectedFiles)) {
      try {
        await this.apiService.deleteFile(path, true);
      } catch (error: any) {
        console.error(`Failed to delete ${path}:`, error);
        alert(`Failed to delete ${path}: ${error.message}`);
      }
    }

    await this.loadDirectory(this.currentPath);
  }

  onCopyToOther(): void {
    if (this.selectedFiles.size > 0) {
      this.copyToOther.emit(Array.from(this.selectedFiles));
    }
  }

  onMoveToOther(): void {
    if (this.selectedFiles.size > 0) {
      this.moveToOther.emit(Array.from(this.selectedFiles));
    }
  }

  // ============================================================================
  // Focus Methods for Keyboard Shortcuts
  // ============================================================================

  focusFilter(): void {
    setTimeout(() => {
      const el = document.querySelector(
        `.pane:nth-child(${this.paneId === 'left' ? 1 : 2}) .filter-input`
      ) as HTMLInputElement;
      el?.focus();
    });
  }

  focusAddressBar(): void {
    setTimeout(() => {
      const el = document.querySelector(
        `.pane:nth-child(${this.paneId === 'left' ? 1 : 2}) .address-input`
      ) as HTMLInputElement;
      el?.focus();
      el?.select();
    });
  }
}