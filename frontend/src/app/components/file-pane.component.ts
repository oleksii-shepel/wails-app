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
import {
  ArchiveService, ArchiveFrame, isArchive, buildArchiveDisplayPath, buildArchiveTabTitle
} from '../services/archive.service';

declare const window: any;

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
        [inArchive]="isInArchive()"
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
        [archiveStack]="archiveStack"
        [fsDirPath]="fsDirPath"
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

    app-tab-bar,
    app-address-bar,
    app-filter-bar,
    app-action-bar {
      display: block;
      flex-shrink: 0;
    }

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
  /** Full path to the archive file currently open (e.g. C:/Desktop.zip). Used by address bar. */
  fsDirPath = '';
  /** The real FS directory containing the archive — restored when exiting. */
  private fsParentDir = '';
  currentTabId = '';
  pathReady = false;
  tabs: TabInfo[] = [];
  entries: FileInfo[] = [];
  filteredEntries: FileInfo[] = [];
  selectedFiles: Set<string> = new Set();
  loading = false;
  error: string | null = null;
  filterText = '';
  filterQuery: FilterQuery = { text: '', criteria: [] };

  archiveStack: ArchiveFrame[] = [];

  FileType = FileType;

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

    const allPaths = this.workspaceService.getAllTabPaths();
    const windowsRoot = allPaths.find(p => /^[A-Za-z]:[/\\]/.test(p));
    const platformRoot = windowsRoot
      ? windowsRoot.replace(/\\/g, '/').replace(/^([A-Za-z]:).*$/, '$1/')
      : '/';

    const isWindows = platformRoot !== '/';
    const pathIsStale = !savedPath || (isWindows && savedPath === '/');
    const pathToLoad = pathIsStale ? platformRoot : savedPath;

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
      this.fsDirPath = listing.path;
      this.fsParentDir = listing.path;  // remember the FS dir for archive exit
      this.entries = listing.entries;
      this.selectedFiles.clear();
      this.applyFilter();
      this.workspaceService.updateTabPath(this.paneId, this.currentTabId, listing.path);
      this.pathHistoryService.addPath(listing.path, this.paneId);
      this.pathChange.emit(listing.path);

      try {
        if (window.go?.main?.App?.SetPaneDirectory) {
          await window.go.main.App.SetPaneDirectory(this.paneId, listing.path);
        }
      } catch { /* optional — ignore */ }
    } catch (error: any) {
      this.error = error.message || 'Failed to load directory';
    } finally {
      this.loading = false;
    }
  }

  // ── Tab management ────────────────────────────────────────────────────────

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

  // ── Address bar / path ────────────────────────────────────────────────────

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

  // ── Navigation ────────────────────────────────────────────────────────────

  onNavigate(entry: FileInfo): void {
    // Special sentinel for the '..' row inside archives
    if (entry.path === '__archive_parent__') {
      this.navigateArchiveUp();
      return;
    }

    if (this.isInArchive()) {
      if (entry.type === FileType.DIRECTORY) {
        // Navigate deeper into a folder within the current archive level.
        // entry.path holds the full innerPath as returned by Go.
        this.archiveNavigateInto(entry.path);
      } else if (entry.type === FileType.ARCHIVE) {
        // Open a nested archive. Go put the correct nestedChain on the entry.
        const chain: string[] = (entry as any).nestedChain;
        if (chain?.length) {
          this.openNestedArchive(chain);
        }
      }
      return;
    }

    // Normal filesystem navigation
    if (entry.type === FileType.DIRECTORY) {
      if (entry.name === '..') this.onNavigateUp();
      else this.loadDirectory(entry.path);
      return;
    }

    // ARCHIVE type or a file whose name looks like an archive
    if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
      this.openRootArchive(entry.path);
    }
  }

  onOpenSelectedItem(): void {
    const selected = Array.from(this.selectedFiles);
    if (selected.length === 1) {
      const entry = this.entries.find(e => e.path === selected[0]);
      if (entry) this.onNavigate(entry);
    }
  }

  // ── Archive navigation ────────────────────────────────────────────────────

  isInArchive(): boolean { return this.archiveStack.length > 0; }

  /** Open a top-level (filesystem) archive. */
  async openRootArchive(fsPath: string): Promise<void> {
    this.loading = true;
    this.error = null;
    try {
      const frame = await this.archiveService.openRootArchive(fsPath);
      this.archiveStack = [frame];
      // fsParentDir = the directory we came from (for restoring on exit)
      // fsDirPath   = the archive file path itself (for address bar / breadcrumbs)
      this.fsParentDir = this.fsDirPath;
      this.fsDirPath = fsPath;
      this.renderArchiveFrame(frame);
    } catch (err: any) {
      this.error = err.message || 'Failed to open archive';
    } finally {
      this.loading = false;
    }
  }

  /**
   * Navigate into a sub-directory within the current archive frame.
   * `folderInnerPath` is the full inner path from the Go FileInfo.path field.
   */
  private async archiveNavigateInto(folderInnerPath: string): Promise<void> {
    const currentFrame = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error = null;
    try {
      const newFrame = await this.archiveService.navigateInto(currentFrame, folderInnerPath);
      // New array reference so Angular OnChanges fires on [archiveStack] input
      this.archiveStack = [...this.archiveStack.slice(0, -1), newFrame];
      this.renderArchiveFrame(newFrame);
    } catch (err: any) {
      this.error = err.message || 'Failed to navigate into folder';
    } finally {
      this.loading = false;
    }
  }

  /** Open a nested archive using the chain provided by the Go backend. */
  private async openNestedArchive(nestedChain: string[]): Promise<void> {
    const currentFrame = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error = null;
    try {
      const newFrame = await this.archiveService.openNestedArchive(currentFrame, nestedChain);
      this.archiveStack = [...this.archiveStack, newFrame];  // new reference ✓
      this.renderArchiveFrame(newFrame);
    } catch (err: any) {
      this.error = err.message || 'Failed to open nested archive';
    } finally {
      this.loading = false;
    }
  }

  /** Navigate up one level: parent folder → archive root → previous archive level → filesystem. */
  async navigateArchiveUp(): Promise<void> {
    const currentFrame = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error = null;
    try {
      // Try navigating to the parent folder within the same archive level.
      const parentFrame = await this.archiveService.navigateUp(currentFrame);
      if (parentFrame !== null) {
        this.archiveStack = [...this.archiveStack.slice(0, -1), parentFrame]; // new reference ✓
        this.renderArchiveFrame(parentFrame);
        return;
      }

      // At the root of this archive level — pop the stack.
      const poppedStack = this.archiveStack.slice(0, -1);

      if (poppedStack.length === 0) {
        this.archiveStack = [];                               // new reference ✓
        await this.loadDirectory(this.fsParentDir || this.fsDirPath);
        return;
      }

      this.archiveStack = poppedStack;                       // new reference ✓
      this.renderArchiveFrame(poppedStack[poppedStack.length - 1]);
    } catch (err: any) {
      this.error = err.message || 'Failed to navigate up';
    } finally {
      this.loading = false;
    }
  }

  exitArchive(): void {
    this.archiveStack = [];
    const restorePath = this.fsParentDir || this.fsDirPath;
    if (restorePath) {
      this.currentPath = restorePath;
      this.fsDirPath = restorePath;
      const dirName = restorePath.split('/').filter(Boolean).pop() ?? restorePath;
      this.workspaceService.updateTabPathAndTitle(this.paneId, this.currentTabId, restorePath, dirName);
      this.tabs = this.workspaceService.getTabs(this.paneId);
    }
  }

  private async refreshArchive(): Promise<void> {
    const frame = this.archiveStack[this.archiveStack.length - 1];
    this.loading = true;
    this.error = null;
    try {
      const refreshed = await this.archiveService.refreshFrame(frame);
      this.archiveStack = [...this.archiveStack.slice(0, -1), refreshed]; // new reference ✓
      this.renderArchiveFrame(refreshed);
    } catch (err: any) {
      this.error = err.message || 'Failed to refresh archive';
    } finally {
      this.loading = false;
    }
  }

  /** Convert the current archive frame to FileInfo rows and update the address bar + tab. */
  private renderArchiveFrame(frame: ArchiveFrame): void {
    this.entries = this.archiveService.listingToFileInfos(frame.listing);
    this.selectedFiles.clear();
    this.applyFilter();
    // Both address bar and tab show the same virtual path: C:/Desktop.zip/src
    const virtualPath = buildArchiveDisplayPath(this.fsDirPath, this.archiveStack);
    this.currentPath = virtualPath;
    const title = buildArchiveTabTitle(this.fsDirPath, this.archiveStack);
    this.workspaceService.updateTabPathAndTitle(this.paneId, this.currentTabId, virtualPath, title);
    this.tabs = this.workspaceService.getTabs(this.paneId);
  }

  // ── Filtering ─────────────────────────────────────────────────────────────

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

  // ── Selection ─────────────────────────────────────────────────────────────

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

  // ── File operations ───────────────────────────────────────────────────────

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