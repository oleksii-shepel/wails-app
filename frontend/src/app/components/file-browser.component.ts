import { Component, OnInit, OnDestroy, ViewChild, ElementRef, HostListener } from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

import { ApiService } from '../services/api.service';
import { WorkspaceService } from '../services/workspace.service';
import { FileType } from '../shared/protocol';
import { WorkspaceConfig } from '../shared/protocol-enhanced';
import { KeyboardHelpComponent } from './keyboard-help.component';
import { WorkspaceSelectorComponent } from './workspace-selector.component';
import { ShortcutCallbacks, ShortcutRegistryService } from '../services/shortcut-registry.service';
import { GlobalSearchComponent } from './global-search.component';
import { PathHistoryService } from '../services/path-history.service';
import { ThemeService } from '../services/theme.service';
import { PathHistoryViewerComponent } from './path-history-viewer.component';
import { FilePaneComponent } from './file-pane.component';
import { SplitDividerComponent } from './split-divider.component';
import { StatusBarComponent } from './status-bar.component';
import { OperationStatusBarComponent } from './operation-status-bar.component';
import { OperationToastStackComponent } from './operation-stack.component';
import { ArchiveService } from '../services/archive.service';

interface CopyMoveItem { source: string; destination: string; }
interface OpResult { failed: number; errors?: Array<{ source: string; error: string }> }

@Component({
  selector: 'app-file-browser',
  standalone: true,
  imports: [
    CommonModule,
    WorkspaceSelectorComponent,
    FilePaneComponent,
    PathHistoryViewerComponent,
    GlobalSearchComponent,
    KeyboardHelpComponent,
    SplitDividerComponent,
    StatusBarComponent,
    OperationToastStackComponent,
    OperationStatusBarComponent
],
  templateUrl: './file-browser.component.html',
  styleUrls: ['./file-browser.component.scss'],
})
export class FileBrowserComponent implements OnInit, OnDestroy {
  @ViewChild(KeyboardHelpComponent) keyboardHelp?: KeyboardHelpComponent;
  @ViewChild(GlobalSearchComponent) globalSearch?: GlobalSearchComponent;
  @ViewChild(PathHistoryViewerComponent) pathHistoryViewer?: PathHistoryViewerComponent;
  @ViewChild('panesContainer') panesContainer?: ElementRef;

  @ViewChild('leftPane') leftPane?: FilePaneComponent;
  @ViewChild('rightPane') rightPane?: FilePaneComponent;
  @ViewChild('toastStack') toastStack?: OperationToastStackComponent;

  activeWorkspace: WorkspaceConfig | null = null;
  workspaceList: WorkspaceConfig[] = [];
  showWorkspaceMenu = false;
  showHidden = false;
  activePaneId: 'left' | 'right' = 'left';

  leftPaneWidth = 50;
  rightPaneWidth = 50;
  isResizing = false;
  toastCollapsed = false;
  
  private startX = 0;
  private startLeftWidth = 0;
  private readonly MIN_PANE_WIDTH = 20;
  private readonly MAX_PANE_WIDTH = 80;

  FileType = FileType;

  private destroy$ = createSubject<void>();
  private clipboard: { operation: 'copy' | 'cut'; paths: string[] } | null = null;

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private shortcutRegistry: ShortcutRegistryService,
    private pathHistoryService: PathHistoryService,
    public theme: ThemeService,
    private archiveService: ArchiveService,
  ) {}

  ngOnInit(): void {
    this.workspaceService.activeWorkspace$
      .pipe(takeUntil(this.destroy$))
      .subscribe(workspace => {
        this.activeWorkspace = workspace;
        if (workspace) {
          this.showHidden = workspace.showHidden;
        }
      });

    this.workspaceService.workspaces$
      .pipe(takeUntil(this.destroy$))
      .subscribe(workspaces => {
        this.workspaceList = workspaces.workspaces;
      });

    this.checkServerConnection();
    this.registerKeyboardShortcuts();
    this.loadPaneWidths();
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
    this.apiService.disconnectWebSocket();
    this.cleanupResizeListeners();
  }

  copyToOtherPane(source: FilePaneComponent | undefined, target: FilePaneComponent | undefined, paths?: string[]): void {
    if (!source || !target) return;
    const selected = paths ?? Array.from(source.selectedFiles);
    if (selected.length === 0) return;
    if (source === this.leftPane && target === this.rightPane) {
      this.onCopyFromLeft(selected);
    } else if (source === this.rightPane && target === this.leftPane) {
      this.onCopyFromRight(selected);
    }
  }

  moveToOtherPane(source: FilePaneComponent | undefined, target: FilePaneComponent | undefined, paths?: string[]): void {
    if (!source || !target) return;
    const selected = paths ?? Array.from(source.selectedFiles);
    if (selected.length === 0) return;
    if (source === this.leftPane && target === this.rightPane) {
      this.onMoveFromLeft(selected);
    } else if (source === this.rightPane && target === this.leftPane) {
      this.onMoveFromRight(selected);
    }
  }

  private getActivePane(): FilePaneComponent | undefined {
    return this.activePaneId === 'left' ? this.leftPane : this.rightPane;
  }

  private getOtherPane(): FilePaneComponent | undefined {
    return this.activePaneId === 'left' ? this.rightPane : this.leftPane;
  }

  private archiveError(result: any): string | null {
    if (!result?.failed) return null;
    return (result.errors ?? []).map((e: any) => e.error).join('\n');
  }

  private bothArchive(src?: FilePaneComponent, dst?: FilePaneComponent): boolean {
    return !!src?.isInArchive() && !!dst?.isInArchive();
  }

  // ============================================================================
  // Server Connection
  // ============================================================================

  async checkServerConnection(): Promise<void> {
    const isHealthy = await this.apiService.checkHealth();
    if (!isHealthy) {
      console.error('Cannot connect to server');
    }
  }

  // ============================================================================
  // Workspace Management
  // ============================================================================

  async createNewWorkspace(): Promise<void> {
    const name = prompt('Enter workspace name:');
    if (!name) return;
    const workspace = await this.workspaceService.createWorkspace(name);
    this.workspaceService.switchWorkspace(workspace.id);
    this.showWorkspaceMenu = false;
  }

  switchWorkspace(workspaceId: string): void {
    this.workspaceService.switchWorkspace(workspaceId);
    this.showWorkspaceMenu = false;
  }

  toggleWorkspaceMenu(event?: Event): void {
    if (event) event.stopPropagation();
    this.showWorkspaceMenu = !this.showWorkspaceMenu;
  }

  closeWorkspaceMenu(): void {
    this.showWorkspaceMenu = false;
  }

  deleteCurrentWorkspace(): void {
    if (!this.activeWorkspace) return;
    if (!confirm(`Delete workspace "${this.activeWorkspace.name}"?`)) return;
    this.workspaceService.deleteWorkspace(this.activeWorkspace.id);
  }

  renameCurrentWorkspace(): void {
    if (!this.activeWorkspace) return;
    const name = prompt('Enter new workspace name:', this.activeWorkspace.name);
    if (!name) return;
    this.workspaceService.updateWorkspace(this.activeWorkspace.id, { name });
  }

  async toggleShowHidden(): Promise<void> {
    this.showHidden = !this.showHidden;
    if (this.activeWorkspace) {
      this.workspaceService.updateWorkspace(this.activeWorkspace.id, {
        showHidden: this.showHidden
      });
    }
    await Promise.all([
      this.leftPane?.onRefresh(),
      this.rightPane?.onRefresh()
    ]);
  }

  // ============================================================================
  // Cross-Pane Operations - Archive-Aware Implementations
  // ============================================================================

  /**
   * Helper to build proper destination path
   * If destination is a directory, append the source filename
   * Otherwise use destination as-is
   */
  private async buildDestinationPath(sourcePath: string, targetDir: string): Promise<string> {
    try {
      // Check if target is a directory
      const isTargetDir = await this.apiService.isDirectory(targetDir);
      
      if (isTargetDir) {
        // Get filename from source path
        const fileName = await this.getFileName(sourcePath);
        // Join with target directory
        return await this.apiService.joinPaths([targetDir, fileName]);
      }
      
      // Target is not a directory (shouldn't happen in normal usage, but handle it)
      return targetDir;
    } catch (error) {
      console.warn('Error building destination path, falling back to target dir:', error);
      return targetDir;
    }
  }

  /**
   * Extract filename from a path
   */
  private async getFileName(path: string): Promise<string> {
    try {
      // Try using the API first
      const splitResult = await this.apiService.splitPath(path);
      if (splitResult.base) {
        return splitResult.base;
      }
    } catch (error) {
      console.warn('Error splitting path via API, falling back to manual split:', error);
    }
    
    // Fallback: manual split
    const normalized = path.replace(/\\/g, '/');
    const parts = normalized.split('/').filter(p => p.length > 0);
    return parts.length > 0 ? parts[parts.length - 1] : 'file';
  }

  async onCopyFromLeft(paths: string[]): Promise<void> {
    const src = this.leftPane;
    const dst = this.rightPane;
    if (!src || !dst || paths.length === 0) return;

    if (src.isInArchive()) {
      if (this.bothArchive(src, dst)) {
        alert('Cannot copy between two archive panes. Open a real folder in one pane first.');
        return;
      }
      const frame = src.archiveStack[src.archiveStack.length - 1];
      const result = await this.archiveService.copy(frame, paths, dst.currentPath);
      const err = this.archiveError(result);
      if (err) alert(`Copy completed with errors:\n${err}`);
      dst.onRefresh();
      return;
    }

    // Filesystem copy — unchanged
    const items = await Promise.all(
      paths.map(async srcPath => ({ 
        source: srcPath, 
        destination: await this.buildDestinationPath(srcPath, dst.currentPath) 
      }))
    );
    const result = await this.apiService.batchCopy(items, true);
    const err = this.archiveError(result);
    if (err) alert(`Copy errors:\n${err}`);
    dst.onRefresh();
  }

  async onCopyFromRight(paths: string[]): Promise<void> {
    const src = this.rightPane;
    const dst = this.leftPane;
    if (!src || !dst || paths.length === 0) return;

    if (src.isInArchive()) {
      if (this.bothArchive(src, dst)) {
        alert('Cannot copy between two archive panes. Open a real folder in one pane first.');
        return;
      }
      const frame = src.archiveStack[src.archiveStack.length - 1];
      const result = await this.archiveService.copy(frame, paths, dst.currentPath);
      const err = this.archiveError(result);
      if (err) alert(`Copy completed with errors:\n${err}`);
      dst.onRefresh();
      return;
    }

    const items = await Promise.all(
      paths.map(async srcPath => ({ 
        source: srcPath, 
        destination: await this.buildDestinationPath(srcPath, dst.currentPath) 
      }))
    );
    const result = await this.apiService.batchCopy(items, true);
    const err = this.archiveError(result);
    if (err) alert(`Copy errors:\n${err}`);
    dst.onRefresh();
  }

  async onMoveFromLeft(paths: string[]): Promise<void> {
    const src = this.leftPane;
    const dst = this.rightPane;
    if (!src || !dst || paths.length === 0) return;

    if (src.isInArchive()) {
      if (this.bothArchive(src, dst)) {
        alert('Cannot move between two archive panes. Open a real folder in one pane first.');
        return;
      }
      // Delegate: pane shows warning dialog → user confirms → pane calls
      // archiveOps.move() → operation events → toast progress shown automatically
      src.initiatePendingMove(paths, dst.currentPath);
      return;
    }

    // Filesystem move — unchanged
    const items = await Promise.all(
      paths.map(async srcPath => ({ 
        source: srcPath, 
        destination: await this.buildDestinationPath(srcPath, dst.currentPath) 
      }))
    );
    const result = await this.apiService.batchMove(items, true);
    const err = this.archiveError(result);
    if (err) alert(`Move errors:\n${err}`);
    src.onRefresh();
    dst.onRefresh();
  }

  async onMoveFromRight(paths: string[]): Promise<void> {
    const src = this.rightPane;
    const dst = this.leftPane;
    if (!src || !dst || paths.length === 0) return;

    if (src.isInArchive()) {
      if (this.bothArchive(src, dst)) {
        alert('Cannot move between two archive panes. Open a real folder in one pane first.');
        return;
      }
      src.initiatePendingMove(paths, dst.currentPath);
      return;
    }

    const items = await Promise.all(
      paths.map(async srcPath => ({ 
        source: srcPath, 
        destination: await this.buildDestinationPath(srcPath, dst.currentPath) 
      }))
    );
    const result = await this.apiService.batchMove(items, true);
    const err = this.archiveError(result);
    if (err) alert(`Move errors:\n${err}`);
    src.onRefresh();
    dst.onRefresh();
  }

  // ============================================================================
  // Clipboard Operations - FIXED with proper destination path handling
  // ============================================================================

  private copySelectedToClipboard(): void {
    const pane = this.getActivePane();
    if (pane?.selectedFiles.size) {
      this.clipboard = {
        operation: 'copy',
        paths: Array.from(pane.selectedFiles)
      };
      console.log('Copied to clipboard:', this.clipboard);
    }
  }

  private cutSelectedToClipboard(): void {
    const pane = this.getActivePane();
    if (pane?.selectedFiles.size) {
      this.clipboard = {
        operation: 'cut',
        paths: Array.from(pane.selectedFiles)
      };
      console.log('Cut to clipboard:', this.clipboard);
    }
  }

  private async pasteFromClipboard(): Promise<void> {
    if (!this.clipboard) return;

    const targetPane = this.getActivePane();
    if (!targetPane) return;

    const { operation, paths } = this.clipboard;
    console.log(`Pasting ${operation} operation:`, paths);

    try {
      const items = await Promise.all(paths.map(async sourcePath => {
        const destination = await this.buildDestinationPath(sourcePath, targetPane.currentPath);
        return {
          source: sourcePath,
          destination: destination
        };
      }));

      let result;
      if (operation === 'copy') {
        result = await this.apiService.batchCopy(items, true);
      } else {
        result = await this.apiService.batchMove(items, true);
      }
      
      console.log('Paste result:', result);
      
      if (result.failed > 0) {
        const errorMsg = result.errors?.map(e => `${e.source}: ${e.error}`).join('\n');
        alert(`Paste completed with ${result.failed} errors:\n${errorMsg}`);
      }
    } catch (error: any) {
      console.error('Paste error:', error);
      alert(`Paste error: ${error.message}`);
    }

    if (operation === 'cut') {
      this.clipboard = null;
    }

    await targetPane.onRefresh();
  }

  private copyToOtherPaneCurrent(): void {
    const src = this.getActivePane();
    const paths = Array.from(src?.selectedFiles || []);
    if (paths.length === 0) return;
    
    console.log('Copy to other pane current:', paths);
    if (this.activePaneId === 'left') {
      this.onCopyFromLeft(paths);
    } else {
      this.onCopyFromRight(paths);
    }
  }

  private moveToOtherPaneCurrent(): void {
    const src = this.getActivePane();
    const paths = Array.from(src?.selectedFiles || []);
    if (paths.length === 0) return;
    
    console.log('Move to other pane current:', paths);
    if (this.activePaneId === 'left') {
      this.onMoveFromLeft(paths);
    } else {
      this.onMoveFromRight(paths);
    }
  }

  onPanePathChange(paneId: string, path: string): void {
    this.pathHistoryService.addPath(path, paneId);
  }

  // ============================================================================
  // Global Search
  // ============================================================================

  onGlobalSearchResult(result: any): void {
    const pane = this.getActivePane();
    if (!pane) return;

    if (result.file.type === FileType.DIRECTORY) {
      pane.onPathChange(result.file.path);
    } else {
      const parentPath = this.getParentPath(result.file.path);
      pane.onPathChange(parentPath);
    }
  }

  private getParentPath(path: string): string {
    if (path === '/' || path === '') return '/';
    const parts = path.split('/').filter(p => p.length > 0);
    if (parts.length === 0) return '/';
    parts.pop();
    return '/' + parts.join('/');
  }

  // ============================================================================
  // Keyboard Shortcuts
  // ============================================================================

  private registerKeyboardShortcuts(): void {
    const callbacks: ShortcutCallbacks = {
      openGlobalSearch:      () => this.globalSearch?.show(),
      showPathHistory:       () => this.pathHistoryViewer?.show(),
      quickPathJump:         () => console.log('Quick path jump - Phase 2'),
      navigateUp:            () => this.getActivePane()?.onNavigateUp(),
      refresh:               () => this.getActivePane()?.onRefresh(),
      showKeyboardHelp:      () => this.keyboardHelp?.show(),
      newTab:                () => this.getActivePane()?.onNewTab(),
      closeTab:              () => this.getActivePane()?.onCloseCurrentTab(),
      nextTab:               () => this.getActivePane()?.onNextTab(),
      previousTab:           () => this.getActivePane()?.onPreviousTab(),
      jumpToTab:             (index) => this.getActivePane()?.onJumpToTab(index),
      togglePinTab:          () => this.getActivePane()?.onTogglePinCurrentTab(),
      quickView:             () => console.log('Quick view - Phase 3'),
      openItem:              () => this.getActivePane()?.onOpenSelectedItem(),
      deleteSelected:        () => this.getActivePane()?.onDeleteSelected(),
      copySelected:          () => this.copySelectedToClipboard(),
      cutSelected:           () => this.cutSelectedToClipboard(),
      paste:                 () => this.pasteFromClipboard(),
      selectAll:             () => this.getActivePane()?.onSelectAll(),
      deselectAll:           () => this.getActivePane()?.onDeselectAll(),
      newFolder:             () => this.getActivePane()?.onCreateFolder(),
      renameSelected:        () => this.getActivePane()?.onRenameSelected(),
      toggleCurrentAndMoveDown: () => this.getActivePane()?.onToggleCurrentAndMoveDown(),
      selectCurrentAndMoveDown: () => this.getActivePane()?.onSelectCurrentAndMoveDown(),
      extendSelectionUp:     () => this.getActivePane()?.onExtendSelection(-1),
      extendSelectionDown:   () => this.getActivePane()?.onExtendSelection(1),
      selectByMask:          () => this.getActivePane()?.onSelectByMask(),
      unselectByMask:        () => this.getActivePane()?.onUnselectByMask(),
      invertSelection:       () => this.getActivePane()?.onInvertSelection(),
      selectSameExtension:   () => this.getActivePane()?.onSelectSameExtension(),
      focusFilter:           () => this.getActivePane()?.focusFilter(),
      advancedFilter:        () => console.log('Advanced filter - to be implemented'),
      clearFilter:           () => this.getActivePane()?.onClearFilter(),
      showWorkspaceSwitcher: () => this.toggleWorkspaceMenu(),
      newWorkspace:          () => this.createNewWorkspace(),
      toggleHiddenFiles:     () => this.toggleShowHidden(),
      switchPane:            () => this.switchActivePaneInternal(),
      copyToOtherPane:       () => this.copyToOtherPaneCurrent(),
      moveToOtherPane:       () => this.moveToOtherPaneCurrent(),
      focusAddressBar:       () => this.getActivePane()?.focusAddressBar(),
    };
    this.shortcutRegistry.registerAllShortcuts(callbacks);
  }

  private switchActivePaneInternal(): void {
    this.activePaneId = this.activePaneId === 'left' ? 'right' : 'left';
  }

  // ============================================================================
  // Resizable Panels
  // ============================================================================

  private loadPaneWidths(): void {
    const savedLeftWidth = localStorage.getItem('fileManager_leftPaneWidth');
    const savedRightWidth = localStorage.getItem('fileManager_rightPaneWidth');

    if (savedLeftWidth && savedRightWidth) {
      this.leftPaneWidth = parseFloat(savedLeftWidth);
      this.rightPaneWidth = parseFloat(savedRightWidth);
    }
  }

  private savePaneWidths(): void {
    localStorage.setItem('fileManager_leftPaneWidth', this.leftPaneWidth.toString());
    localStorage.setItem('fileManager_rightPaneWidth', this.rightPaneWidth.toString());
  }

  startResize(event: MouseEvent): void {
    event.preventDefault();
    this.isResizing = true;
    this.startX = event.clientX;
    this.startLeftWidth = this.leftPaneWidth;

    document.addEventListener('mousemove', this.onResize);
    document.addEventListener('mouseup', this.stopResize);
    document.body.classList.add('resizing');
  }

  private onResize = (event: MouseEvent): void => {
    if (!this.isResizing || !this.panesContainer) return;

    event.preventDefault();

    const containerWidth = this.panesContainer.nativeElement.clientWidth;
    const deltaX = event.clientX - this.startX;
    const deltaPercent = (deltaX / containerWidth) * 100;

    let newLeftWidth = this.startLeftWidth + deltaPercent;
    newLeftWidth = Math.max(this.MIN_PANE_WIDTH, Math.min(this.MAX_PANE_WIDTH, newLeftWidth));

    this.leftPaneWidth = newLeftWidth;
    this.rightPaneWidth = 100 - newLeftWidth;
  };

  private stopResize = (): void => {
    if (this.isResizing) {
      this.isResizing = false;
      document.removeEventListener('mousemove', this.onResize);
      document.removeEventListener('mouseup', this.stopResize);
      document.body.classList.remove('resizing');
      this.savePaneWidths();
    }
  };

  resetPaneWidths(): void {
    this.leftPaneWidth = 50;
    this.rightPaneWidth = 50;
    this.savePaneWidths();
  }

  private cleanupResizeListeners(): void {
    if (this.isResizing) {
      document.removeEventListener('mousemove', this.onResize);
      document.removeEventListener('mouseup', this.stopResize);
    }
  }

  @HostListener('window:beforeunload')
  onBeforeUnload(): void {
    this.cleanupResizeListeners();
  }

  @HostListener('window:keydown.arrowleft', ['$event'])
  onArrowLeft(event: any): void {
    if (document.activeElement?.classList.contains('pane-divider')) {
      event.preventDefault();
      this.leftPaneWidth = Math.max(this.MIN_PANE_WIDTH, this.leftPaneWidth - 5);
      this.rightPaneWidth = 100 - this.leftPaneWidth;
      this.savePaneWidths();
    }
  }

  @HostListener('window:keydown.arrowright', ['$event'])
  onArrowRight(event: any): void {
    if (document.activeElement?.classList.contains('pane-divider')) {
      event.preventDefault();
      this.leftPaneWidth = Math.min(this.MAX_PANE_WIDTH, this.leftPaneWidth + 5);
      this.rightPaneWidth = 100 - this.leftPaneWidth;
      this.savePaneWidths();
    }
  }

  statusText: string = 'Ready';
}