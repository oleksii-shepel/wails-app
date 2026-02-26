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
    OperationToastStackComponent
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

  activeWorkspace: WorkspaceConfig | null = null;
  workspaceList: WorkspaceConfig[] = [];
  showWorkspaceMenu = false;
  showHidden = false;
  activePaneId: 'left' | 'right' = 'left';

  leftPaneWidth = 50;
  rightPaneWidth = 50;
  isResizing = false;
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
  // Cross-Pane Operations
  //
  // All four methods now call the *WithProgress variants so the backend emits
  // operation:start / operation:progress / operation:complete events, which
  // the OperationService picks up and renders as toast notifications.
  //
  // We no longer await each file sequentially and then refresh — the copy runs
  // in the background and the pane refreshes once all files are done.
  // ============================================================================

  async onCopyFromLeft(paths: string[]): Promise<void> {
    const source = this.leftPane;
    const target = this.rightPane;
    if (!source || !target || paths.length === 0) return;

    const errors: string[] = [];

    await Promise.all(paths.map(async sourcePath => {
      try {
        const splitResult = await (window as any).go.main.App.SplitPath(sourcePath);
        const fileName = splitResult['base'] || 'file';
        const destPath: string = await (window as any).go.main.App.JoinPaths([target.currentPath, fileName]);
        // ↓ progress-aware copy — fires toast notifications automatically
        await this.apiService.copyFileWithProgress(sourcePath, destPath, true);
      } catch (error: any) {
        errors.push(`${sourcePath}: ${error.message}`);
      }
    }));

    if (errors.length) alert(`Copy errors:\n${errors.join('\n')}`);
    await target.onRefresh();
  }

  async onMoveFromLeft(paths: string[]): Promise<void> {
    const source = this.leftPane;
    const target = this.rightPane;
    if (!source || !target || paths.length === 0) return;

    const errors: string[] = [];

    await Promise.all(paths.map(async sourcePath => {
      try {
        const splitResult = await (window as any).go.main.App.SplitPath(sourcePath);
        const fileName = splitResult['base'] || 'file';
        const destPath: string = await (window as any).go.main.App.JoinPaths([target.currentPath, fileName]);
        // ↓ progress-aware move
        await this.apiService.moveFileWithProgress(sourcePath, destPath);
      } catch (error: any) {
        errors.push(`${sourcePath}: ${error.message}`);
      }
    }));

    if (errors.length) alert(`Move errors:\n${errors.join('\n')}`);
    await Promise.all([source.onRefresh(), target.onRefresh()]);
  }

  async onCopyFromRight(paths: string[]): Promise<void> {
    const source = this.rightPane;
    const target = this.leftPane;
    if (!source || !target || paths.length === 0) return;

    const errors: string[] = [];

    await Promise.all(paths.map(async sourcePath => {
      try {
        const splitResult = await (window as any).go.main.App.SplitPath(sourcePath);
        const fileName = splitResult['base'] || 'file';
        const destPath: string = await (window as any).go.main.App.JoinPaths([target.currentPath, fileName]);
        await this.apiService.copyFileWithProgress(sourcePath, destPath, true);
      } catch (error: any) {
        errors.push(`${sourcePath}: ${error.message}`);
      }
    }));

    if (errors.length) alert(`Copy errors:\n${errors.join('\n')}`);
    await target.onRefresh();
  }

  async onMoveFromRight(paths: string[]): Promise<void> {
    const source = this.rightPane;
    const target = this.leftPane;
    if (!source || !target || paths.length === 0) return;

    const errors: string[] = [];

    await Promise.all(paths.map(async sourcePath => {
      try {
        const splitResult = await (window as any).go.main.App.SplitPath(sourcePath);
        const fileName = splitResult['base'] || 'file';
        const destPath: string = await (window as any).go.main.App.JoinPaths([target.currentPath, fileName]);
        await this.apiService.moveFileWithProgress(sourcePath, destPath);
      } catch (error: any) {
        errors.push(`${sourcePath}: ${error.message}`);
      }
    }));

    if (errors.length) alert(`Move errors:\n${errors.join('\n')}`);
    await Promise.all([source.onRefresh(), target.onRefresh()]);
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
      renameSelected:        () => console.log('Rename - to be implemented'),
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
  // Clipboard Operations
  // ============================================================================

  private copySelectedToClipboard(): void {
    const pane = this.getActivePane();
    if (pane?.selectedFiles.size) {
      this.clipboard = {
        operation: 'copy',
        paths: Array.from(pane.selectedFiles)
      };
    }
  }

  private cutSelectedToClipboard(): void {
    const pane = this.getActivePane();
    if (pane?.selectedFiles.size) {
      this.clipboard = {
        operation: 'cut',
        paths: Array.from(pane.selectedFiles)
      };
    }
  }

  private async pasteFromClipboard(): Promise<void> {
    if (!this.clipboard) return;

    const targetPane = this.getActivePane();
    if (!targetPane) return;

    const { operation, paths } = this.clipboard;
    const errors: string[] = [];

    await Promise.all(paths.map(async sourcePath => {
      try {
        const splitResult = await (window as any).go.main.App.SplitPath(sourcePath);
        const fileName = splitResult['base'] || 'file';
        const destPath: string = await (window as any).go.main.App.JoinPaths([targetPane.currentPath, fileName]);

        if (operation === 'copy') {
          await this.apiService.copyFileWithProgress(sourcePath, destPath, true);
        } else {
          await this.apiService.moveFileWithProgress(sourcePath, destPath);
        }
      } catch (error: any) {
        errors.push(`${sourcePath}: ${error.message}`);
      }
    }));

    if (errors.length) alert(`Paste errors:\n${errors.join('\n')}`);

    if (operation === 'cut') {
      this.clipboard = null;
    }

    await targetPane.onRefresh();
  }

  private copyToOtherPaneCurrent(): void {
    const src = this.getActivePane();
    if (this.activePaneId === 'left') {
      this.onCopyFromLeft(Array.from(src?.selectedFiles || []));
    } else {
      this.onCopyFromRight(Array.from(src?.selectedFiles || []));
    }
  }

  private moveToOtherPaneCurrent(): void {
    const src = this.getActivePane();
    if (this.activePaneId === 'left') {
      this.onMoveFromLeft(Array.from(src?.selectedFiles || []));
    } else {
      this.onMoveFromRight(Array.from(src?.selectedFiles || []));
    }
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