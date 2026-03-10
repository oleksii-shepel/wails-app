import { Component, OnInit, OnDestroy, AfterViewInit, ViewChild, ElementRef, HostListener, NgZone, ChangeDetectorRef } from '@angular/core';
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
import { FileBrowserClipboardController, type FileBrowserClipboardHost } from './file-browser-clipboard.controller';
import { FileBrowserDropController, type FileBrowserDropHost } from './file-browser-drop.controller';
import { FileBrowserPathHelper } from './file-browser-path.helper';
import { FileBrowserPreviewController, type FileBrowserPreviewHost } from './file-browser-preview.controller';
import { FileBrowserRuntimeController, type FileBrowserRuntimeHost } from './file-browser-runtime.controller';
import { FileBrowserWorkspaceController, type FileBrowserWorkspaceHost } from './file-browser-workspace.controller';
import { FilePaneComponent } from './file-pane.component';
import { FileBrowserTransferController, type FileBrowserTransferHost } from './file-browser-transfer.controller';
import { SplitDividerComponent } from './split-divider.component';
import { StatusBarComponent } from './status-bar.component';
import { OperationStatusBarComponent } from './operation-status-bar.component';
import { OperationToastStackComponent } from './operation-stack.component';
import { ArchiveService } from '../services/archive.service';
import { ElevationStatusPillComponent } from './elevation-status-pill.component';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { TabInfo } from '../shared/protocol-enhanced';

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
    OperationStatusBarComponent,
    ElevationStatusPillComponent
],
  templateUrl: './file-browser.component.html',
  styleUrls: ['./file-browser.component.scss'],
})
export class FileBrowserComponent implements OnInit, AfterViewInit, OnDestroy {
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
  elevationActive = false;
  elevationBusy = false;
  
  private startX = 0;
  private startLeftWidth = 0;
  private readonly MIN_PANE_WIDTH = 20;
  private readonly MAX_PANE_WIDTH = 80;

  FileType = FileType;
  private readonly clipboardController = new FileBrowserClipboardController();
  private readonly dropController = new FileBrowserDropController();
  private readonly pathHelper: FileBrowserPathHelper;
  private readonly previewController = new FileBrowserPreviewController();
  private readonly runtimeController = new FileBrowserRuntimeController();
  private readonly transferController = new FileBrowserTransferController();
  private readonly workspaceController = new FileBrowserWorkspaceController();

  private destroy$ = createSubject<void>();
  private clipboard: { operation: 'copy' | 'cut'; paths: string[] } | null = null;

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private shortcutRegistry: ShortcutRegistryService,
    private pathHistoryService: PathHistoryService,
    public theme: ThemeService,
    private archiveService: ArchiveService,
    private confirmDialog: ConfirmDialogService,
    private zone: NgZone,
    private cdr: ChangeDetectorRef,
  ) {
    this.pathHelper = new FileBrowserPathHelper(this.apiService);
  }

  ngOnInit(): void {
    this.workspaceService.activeWorkspace$
      .pipe(takeUntil(this.destroy$))
      .subscribe(workspace => {
        // Some Wails/bridge async callbacks can land outside Angular's zone.
        // Ensure state updates trigger a render without requiring a user click.
        this.zone.run(() => {
          this.activeWorkspace = workspace;
          if (workspace) {
            this.showHidden = workspace.showHidden;
          }
          this.cdr.markForCheck();
        });
      });

    this.workspaceService.workspaces$
      .pipe(takeUntil(this.destroy$))
      .subscribe(workspaces => {
        this.zone.run(() => {
          this.workspaceList = workspaces.workspaces;
          this.cdr.markForCheck();
        });
      });

    this.registerKeyboardShortcuts();
    this.loadPaneWidths();
    void this.runtimeController.initialize(this.runtimeHost);
  }



  ngAfterViewInit(): void {
    // File drop handling is done via native Wails event (wails:file-drop)
    // No need to attach browser drag-drop listeners
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
    this.apiService.disconnectWebSocket();
    this.cleanupResizeListeners();

    this.runtimeController.dispose();
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

  private get transferHost(): FileBrowserTransferHost {
    return {
      leftPane: this.leftPane,
      rightPane: this.rightPane,
      apiService: this.apiService,
      archiveService: this.archiveService,
      confirmDialog: this.confirmDialog,
      buildDestinationPath: (sourcePath: string, targetDir: string) => this.buildDestinationPath(sourcePath, targetDir),
    };
  }

  private get clipboardHost(): FileBrowserClipboardHost {
    return {
      getActivePane: () => this.getActivePane(),
      getActivePaneId: () => this.activePaneId,
      apiService: this.apiService,
      confirmDialog: this.confirmDialog,
      buildDestinationPath: (sourcePath: string, targetDir: string) => this.buildDestinationPath(sourcePath, targetDir),
      runCrossPaneCopy: (sourceSide: 'left' | 'right', paths: string[]) => this.transferController.copy(this.transferHost, sourceSide, paths),
      runCrossPaneMove: (sourceSide: 'left' | 'right', paths: string[]) => this.transferController.move(this.transferHost, sourceSide, paths),
    };
  }

  private get dropHost(): FileBrowserDropHost {
    return {
      getActivePane: () => this.getActivePane(),
      getLeftPane: () => this.leftPane,
      getRightPane: () => this.rightPane,
      getPanesContainer: () => this.panesContainer?.nativeElement as HTMLElement | undefined,
      getLeftPaneWidth: () => this.leftPaneWidth,
      apiService: this.apiService,
      confirmDialog: this.confirmDialog,
      buildDestinationPath: (sourcePath: string, targetDir: string) => this.buildDestinationPath(sourcePath, targetDir),
    };
  }

  private get workspaceHost(): FileBrowserWorkspaceHost {
    return {
      workspaceService: this.workspaceService,
      confirmDialog: this.confirmDialog,
      getActiveWorkspace: () => this.activeWorkspace,
      getShowHidden: () => this.showHidden,
      getShowWorkspaceMenu: () => this.showWorkspaceMenu,
      setShowHidden: (showHidden: boolean) => {
        this.showHidden = showHidden;
      },
      setShowWorkspaceMenu: (show: boolean) => {
        this.showWorkspaceMenu = show;
      },
      refreshPanes: async () => {
        await Promise.all([
          this.leftPane?.onRefresh(),
          this.rightPane?.onRefresh(),
        ]);
      },
    };
  }

  private get runtimeHost(): FileBrowserRuntimeHost {
    return {
      apiService: this.apiService,
      confirmDialog: this.confirmDialog,
      getElevationActive: () => this.elevationActive,
      getElevationBusy: () => this.elevationBusy,
      setElevationActive: (active: boolean) => {
        this.elevationActive = active;
      },
      setElevationBusy: (busy: boolean) => {
        this.elevationBusy = busy;
      },
      handleNativeFileDrop: (x: number, y: number, paths: string[]) => this.dropController.handleNativeFileDrop(this.dropHost, x, y, paths),
    };
  }

  private get previewHost(): FileBrowserPreviewHost {
    return {
      workspaceService: this.workspaceService,
      getActivePaneId: () => this.activePaneId,
      setActivePaneId: (paneId: 'left' | 'right') => {
        this.activePaneId = paneId;
      },
      getActivePane: () => this.getActivePane(),
      getLeftPane: () => this.leftPane,
      getRightPane: () => this.rightPane,
    };
  }

  // ============================================================================
  // Workspace Management
  // ============================================================================

  async createNewWorkspace(): Promise<void> {
    await this.workspaceController.createNewWorkspace(this.workspaceHost);
  }

  switchWorkspace(workspaceId: string): void {
    this.workspaceController.switchWorkspace(this.workspaceHost, workspaceId);
  }

  toggleWorkspaceMenu(event?: Event): void {
    this.workspaceController.toggleWorkspaceMenu(this.workspaceHost, event);
  }

  closeWorkspaceMenu(): void {
    this.workspaceController.closeWorkspaceMenu(this.workspaceHost);
  }

  async deleteCurrentWorkspace(): Promise<void> {
    await this.workspaceController.deleteCurrentWorkspace(this.workspaceHost);
  }

  renameCurrentWorkspace(): void {
    this.workspaceController.renameCurrentWorkspace(this.workspaceHost);
  }

  async toggleShowHidden(): Promise<void> {
    await this.workspaceController.toggleShowHidden(this.workspaceHost);
  }

  async toggleElevation(): Promise<void> {
    await this.runtimeController.toggleElevation(this.runtimeHost);
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
    return await this.pathHelper.buildDestinationPath(sourcePath, targetDir);
  }

  async onCopyFromLeft(paths: string[]): Promise<void> {
    await this.transferController.copy(this.transferHost, 'left', paths);
  }

  async onCopyFromRight(paths: string[]): Promise<void> {
    await this.transferController.copy(this.transferHost, 'right', paths);
  }

  async onMoveFromLeft(paths: string[]): Promise<void> {
    await this.transferController.move(this.transferHost, 'left', paths);
  }

  async onMoveFromRight(paths: string[]): Promise<void> {
    await this.transferController.move(this.transferHost, 'right', paths);
  }

  // ============================================================================
  // Clipboard Operations - FIXED with proper destination path handling
  // ============================================================================

  private copySelectedToClipboard(): void {
    this.clipboard = this.clipboardController.copySelectedToClipboard(this.clipboardHost);
  }

  private cutSelectedToClipboard(): void {
    this.clipboard = this.clipboardController.cutSelectedToClipboard(this.clipboardHost);
  }

  private async pasteFromClipboard(): Promise<void> {
    this.clipboard = await this.clipboardController.pasteFromClipboard(this.clipboardHost, this.clipboard);
  }

  private async copyToOtherPaneCurrent(): Promise<void> {
    await this.clipboardController.copyToOtherPaneCurrent(this.clipboardHost);
  }

  private async moveToOtherPaneCurrent(): Promise<void> {
    await this.clipboardController.moveToOtherPaneCurrent(this.clipboardHost);
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
      switchPane:            () => this.previewController.switchActivePane(this.previewHost),
      copyToOtherPane:       () => this.copyToOtherPaneCurrent(),
      moveToOtherPane:       () => this.moveToOtherPaneCurrent(),
      focusAddressBar:       () => this.getActivePane()?.focusAddressBar(),
      moveCursorUp:          () => this.getActivePane()?.onMoveCursorUp(),
      moveCursorDown:        () => this.getActivePane()?.onMoveCursorDown(),
      moveCursorLeft:        () => this.getActivePane()?.onMoveCursorLeft(),
      moveCursorRight:       () => this.getActivePane()?.onMoveCursorRight(),
      moveCursorHome:        () => this.getActivePane()?.onMoveCursorHome(),
      moveCursorEnd:         () => this.getActivePane()?.onMoveCursorEnd(),
      moveCursorPageUp:      () => this.getActivePane()?.onMoveCursorPageUp(),
      moveCursorPageDown:    () => this.getActivePane()?.onMoveCursorPageDown(),
      markCurrentItem:       () => this.getActivePane()?.onMarkCurrentItem(),
      quickView:             () => this.previewController.openFrozenPreviewInOtherPane(this.previewHost),
    };
    this.shortcutRegistry.registerAllShortcuts(callbacks);
  }

  onPanePreviewTarget(fromPane: 'left' | 'right', targetPath: string): void {
    this.previewController.onPanePreviewTarget(this.previewHost, fromPane, targetPath);
  }

  onPaneActiveTabChanged(fromPane: 'left' | 'right', tab: TabInfo): void {
    this.previewController.onPaneActiveTabChanged(this.previewHost, fromPane, tab);
  }

  // NOTE: we intentionally do not auto-create live preview tabs on cursor move.

  // ============================================================================
  // Resizable Panels
  // ============================================================================

  private loadPaneWidths(): void {
    const savedSplit = localStorage.getItem('split-pos');
    const savedLeftWidth = localStorage.getItem('fileManager_leftPaneWidth');

    const raw = savedSplit ?? savedLeftWidth;
    if (!raw) return;
    const parsed = parseFloat(raw);
    if (!Number.isFinite(parsed)) return;

    const clampedLeft = Math.max(this.MIN_PANE_WIDTH, Math.min(this.MAX_PANE_WIDTH, parsed));
    this.leftPaneWidth = clampedLeft;
    this.rightPaneWidth = 100 - clampedLeft;
  }

  private savePaneWidths(): void {
    localStorage.setItem('fileManager_leftPaneWidth', this.leftPaneWidth.toString());
    localStorage.setItem('fileManager_rightPaneWidth', this.rightPaneWidth.toString());
    // Keep SplitDividerComponent restore key in sync.
    localStorage.setItem('split-pos', this.leftPaneWidth.toString());
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
