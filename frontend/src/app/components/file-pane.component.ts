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
import { ArchiveService, ArchiveListing, isArchive } from '../services/archive.service';

declare const window: any;

interface ArchiveStackFrame {
  archivePath: string;
  innerPath: string;
  listing: ArchiveListing;
}

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
    <div class="file-pane" [class.file-pane--archive]="isInArchive()">
      <app-tab-bar
        [tabs]="tabs"
        [activeTabId]="currentTabId"
        (tabChange)="onTabChange($event)"
        (tabClose)="onTabClose($event)"
        (tabPin)="onTabPin($event)"
        (newTab)="onNewTab()">
      </app-tab-bar>

      <!-- Only render address bar once we have the real platform path,
           preventing a flash of '/' before GetInitialPath() resolves -->
      <app-address-bar
        *ngIf="pathReady"
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
    /* Host element must participate in the panes-container flex layout */
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    .file-pane {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      background: var(--vsc-editor-background);
      overflow: hidden;
    }

    .file-pane--archive {
      background: color-mix(in srgb, var(--vsc-accent-orange) 4%, var(--vsc-editor-background));
    }

    /* All child component host elements must not collapse */
    app-tab-bar,
    app-address-bar,
    app-filter-bar,
    app-action-bar {
      display: block;
      flex-shrink: 0;
    }

    /* File list takes all remaining vertical space */
    app-file-list {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
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

  currentPath = '';
  currentTabId = '';
  pathReady = false;  // guards address bar from rendering with stale '/'
  tabs: TabInfo[] = [];
  entries: FileInfo[] = [];
  filteredEntries: FileInfo[] = [];
  selectedFiles: Set<string> = new Set();
  loading = false;
  error: string | null = null;
  filterText = '';
  filterQuery: FilterQuery = { text: '', criteria: [] };

  archiveStack: ArchiveStackFrame[] = [];
  currentArchiveListing: ArchiveListing | null = null;

  FileType = FileType;
  isArchive = isArchive;

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private filterService: FilterService,
    private pathHistoryService: PathHistoryService,
    private archiveService: ArchiveService
  ) {}

  ngOnInit(): void { this.loadPaneState(); }
  ngOnChanges(): void { if (this.workspace) this.loadPaneState(); }

  private async loadPaneState(): Promise<void> {
    if (!this.workspace) return;
    this.tabs = this.workspaceService.getTabs(this.paneId);
    const activeTab = this.workspaceService.getActiveTab(this.paneId);
    const savedPath = activeTab?.path ?? '';

    // Detect Windows by scanning ALL saved tab paths across both panes.
    // If any saved path looks like a Windows drive path (e.g. "C:/"),
    // we're on Windows and a bare "/" is a stale Unix default.
    const allPaths = this.workspaceService.getAllTabPaths();
    const windowsRoot = allPaths.find(p => /^[A-Za-z]:[/\\]/.test(p));
    const platformRoot = windowsRoot
      ? windowsRoot.replace(/\\/g, '/').replace(/^([A-Za-z]:).*$/, '$1/')
      : '/';

    const isWindows = platformRoot !== '/';
    const pathIsStale = !savedPath || (isWindows && savedPath === '/');
    const pathToLoad = pathIsStale ? platformRoot : savedPath;

    console.log(`[file-pane:${this.paneId}] saved="${savedPath}" root="${platformRoot}" stale=${pathIsStale} → load="${pathToLoad}"`);

    this.currentPath = pathToLoad;
    this.pathReady = true;

    if (activeTab) {
      this.currentTabId = activeTab.id;
    }

    await this.loadDirectory(pathToLoad);
  }

  private async loadDirectory(path: string): Promise<void> {
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

      // Keep Go's per-pane current-directory in sync so NavGoUp works correctly.
      // This is a no-op if the method doesn't exist on the Go side.
      try {
        if (window.go?.main?.App?.SetPaneDirectory) {
          await window.go.main.App.SetPaneDirectory(this.paneId, listing.path);
        }
      } catch { /* optional Go method — ignore if not implemented */ }
    } catch (error: any) {
      this.error = error.message || 'Failed to load directory';
    } finally {
      this.loading = false;
    }
  }

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
    if (newTab) this.loadPaneState();
  }

  onCloseCurrentTab(): void {
    const activeTab = this.tabs.find(t => t.isActive);
    if (activeTab) this.onTabClose(activeTab);
  }

  onNextTab(): void {
    const i = this.tabs.findIndex(t => t.isActive);
    if (i >= 0) this.onTabChange(this.tabs[(i + 1) % this.tabs.length]);
  }

  onPreviousTab(): void {
    const i = this.tabs.findIndex(t => t.isActive);
    if (i >= 0) this.onTabChange(this.tabs[i === 0 ? this.tabs.length - 1 : i - 1]);
  }

  onJumpToTab(index: number): void {
    if (index > 0 && index <= this.tabs.length) this.onTabChange(this.tabs[index - 1]);
  }

  onTogglePinCurrentTab(): void {
    const activeTab = this.tabs.find(t => t.isActive);
    if (activeTab) this.onTabPin(activeTab);
  }

  onPathChange(path: string): void {
    if (path && path !== this.currentPath) {
      this.exitArchive();
      this.loadDirectory(path);
    }
  }

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
      } catch { /* already at root */ }
    }
  }

  onRefresh(): void {
    if (this.isInArchive()) this.refreshArchive();
    else this.loadDirectory(this.currentPath);
  }

  onNavigate(entry: FileInfo): void {
    if (entry.path === '__archive_parent__') { this.navigateArchiveUp(); return; }
    if (this.isInArchive()) {
      if (entry.type === FileType.DIRECTORY) {
        const frame = this.archiveStack[this.archiveStack.length - 1];
        this.loadArchive(frame.archivePath, entry.path);
      }
      return;
    }
    if (entry.type === FileType.DIRECTORY) {
      if (entry.name === '..') this.onNavigateUp();
      else this.loadDirectory(entry.path);
      return;
    }
    if (entry.type === FileType.FILE && isArchive(entry.name)) {
      this.loadArchive(entry.path);
    }
  }

  onOpenSelectedItem(): void {
    const selected = Array.from(this.selectedFiles);
    if (selected.length === 1) {
      const entry = this.entries.find(e => e.path === selected[0]);
      if (entry) this.onNavigate(entry);
    }
  }

  // Archive navigation
  isInArchive(): boolean { return this.archiveStack.length > 0; }

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
    return [{
      name: '..',
      path: '__archive_parent__',
      type: FileType.DIRECTORY,
      size: 0,
      modified: 0,
      permissions: 'drwxr-xr-x',
      isHidden: false,
      created: 0,
      accessed: 0,
    }, ...entries];
  }

  // Filtering
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
    if (!this.filterText?.trim()) {
      this.filteredEntries = this.entries;
    } else {
      this.filteredEntries = this.filterService.filterEntries(this.entries, this.filterQuery);
    }
  }

  // Selection
  onSelectionChange(entry: FileInfo): void {
    if (this.selectedFiles.has(entry.path)) this.selectedFiles.delete(entry.path);
    else this.selectedFiles.add(entry.path);
    this.selectedFiles = new Set(this.selectedFiles);
  }

  onSelectAll(): void {
    this.filteredEntries.forEach(e => this.selectedFiles.add(e.path));
    this.selectedFiles = new Set(this.selectedFiles);
  }

  onDeselectAll(): void {
    this.selectedFiles.clear();
    this.selectedFiles = new Set(this.selectedFiles);
  }

  // File Operations
  async onCreateFolder(): Promise<void> {
    if (this.isInArchive()) { alert('Cannot create folders inside an archive.'); return; }
    const folderName = prompt('Enter folder name:');
    if (!folderName) return;
    try {
      const newPath: string = await window.go.main.App.JoinPaths([this.currentPath, folderName]);
      await this.apiService.createDirectory(newPath, false);
      await this.loadDirectory(this.currentPath);
    } catch (error: any) {
      alert(`Failed to create folder: ${error.message}`);
    }
  }

  async onDeleteSelected(): Promise<void> {
    if (this.isInArchive()) { alert('Cannot delete files inside an archive.'); return; }
    if (this.selectedFiles.size === 0) return;
    if (!confirm(`Delete ${this.selectedFiles.size} item(s)?`)) return;
    for (const path of Array.from(this.selectedFiles)) {
      try {
        await this.apiService.deleteFile(path, true);
      } catch (error: any) {
        alert(`Failed to delete ${path}: ${error.message}`);
      }
    }
    await this.loadDirectory(this.currentPath);
  }

  onCopyToOther(): void {
    if (this.selectedFiles.size > 0) this.copyToOther.emit(Array.from(this.selectedFiles));
  }

  onMoveToOther(): void {
    if (this.selectedFiles.size > 0) this.moveToOther.emit(Array.from(this.selectedFiles));
  }

  focusFilter(): void {
    setTimeout(() => {
      const el = document.querySelector(
        `app-file-pane[paneId="${this.paneId}"] .filter-input`
      ) as HTMLInputElement;
      el?.focus();
    });
  }

  focusAddressBar(): void {
    setTimeout(() => {
      const el = document.querySelector(
        `app-file-pane[paneId="${this.paneId}"] .ab-edit-input`
      ) as HTMLInputElement;
      el?.focus();
      el?.select();
    });
  }
}