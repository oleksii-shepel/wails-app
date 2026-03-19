import { Component, OnInit, OnDestroy, AfterViewInit, ViewChild, ElementRef, HostListener, NgZone, ChangeDetectorRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import { ApiService } from '../services/api.service';
import { WorkspaceService } from '../services/workspace.service';
import { FileType } from '../shared/protocol';
import { WorkspaceConfig } from '../shared/protocol-enhanced';
import { KeyboardHelpComponent } from './keyboard-help.component';
import { WorkspaceSelectorComponent } from './workspace-selector.component';
import { FileBrowserCommandHandlers, registerFileBrowserCommands } from './file-browser-command-registry';
import { GlobalSearchComponent } from './global-search.component';
import { PathHistoryService } from '../services/path-history.service';
import { ThemeService } from '../services/theme.service';
import { PathHistoryViewerComponent } from './path-history-viewer.component';
import { FileBrowserClipboardController, type FileBrowserClipboardHost } from './file-browser-clipboard.controller';
import { FileBrowserDropController, type FileBrowserDropHost } from './file-browser-drop.controller';
import { FileBrowserLayoutController, type FileBrowserLayoutHost } from './file-browser-layout.controller';
import { FileBrowserPathHelper } from './file-browser-path.helper';
import { FileBrowserPreviewController, type FileBrowserPreviewHost } from './file-browser-preview.controller';
import { FileBrowserRuntimeController, type FileBrowserRuntimeHost } from './file-browser-runtime.controller';
import { FileBrowserSearchController, type FileBrowserSearchHost } from './file-browser-search.controller';
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
import { InputEngineService } from '../services/input-engine.service';
import { TabInfo } from '../shared/protocol-enhanced';
import type { InputBindingDefinition } from '../services/input-system.types';

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
  private static readonly mouseCommandOwner = 'file-browser-mouse';
  private static readonly mouseBindingIds = [
    'global.mouse.fileEntry.doubleClickOpen',
    'global.mouse.viewer.ctrlWheelUp',
    'global.mouse.viewer.ctrlWheelDown',
  ] as const;

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
  toastCollapsed = false;
  elevationActive = false;
  elevationBusy = false;
  readonly pendingChord$ = this.inputEngine.pendingChord$;

  FileType = FileType;
  private readonly clipboardController = new FileBrowserClipboardController();
  private readonly dropController = new FileBrowserDropController();
  private readonly layoutController = new FileBrowserLayoutController();
  private readonly pathHelper: FileBrowserPathHelper;
  private readonly previewController = new FileBrowserPreviewController();
  private readonly runtimeController = new FileBrowserRuntimeController();
  private readonly searchController = new FileBrowserSearchController();
  private readonly transferController = new FileBrowserTransferController();
  private readonly workspaceController = new FileBrowserWorkspaceController();

  private destroy$ = createSubject<void>();
  private clipboard: { operation: 'copy' | 'cut'; paths: string[] } | null = null;

  private browserCommandCleanup: (() => void) | null = null;

  constructor(
    private apiService: ApiService,
    private workspaceService: WorkspaceService,
    private pathHistoryService: PathHistoryService,
    public theme: ThemeService,
    private archiveService: ArchiveService,
    private confirmDialog: ConfirmDialogService,
    private inputEngine: InputEngineService,
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

    this.browserCommandCleanup = this.setupBrowserCommands();
    this.registerMouseBindings();
    this.layoutController.loadPaneWidths(this.layoutHost);
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

    this.browserCommandCleanup?.();
    this.unregisterMouseBindings();
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

  private getPane(paneId: 'left' | 'right'): FilePaneComponent | undefined {
    return paneId === 'left' ? this.leftPane : this.rightPane;
  }

  private getOppositePaneId(paneId: 'left' | 'right'): 'left' | 'right' {
    return paneId === 'left' ? 'right' : 'left';
  }

  private getPaneTabKind(pane: FilePaneComponent | undefined): 'dir' | 'viewer' | 'size-manager' {
    const tab = pane?.tabs.find((candidate) => candidate.id === pane.currentTabId);
    return (tab?.kind ?? 'dir') as 'dir' | 'viewer' | 'size-manager';
  }

  private syncSizeManagerPane(targetPaneId: 'left' | 'right', path?: string): void {
    const targetPane = this.getPane(targetPaneId);
    if (this.getPaneTabKind(targetPane) !== 'size-manager') return;
    const sourcePane = this.getPane(this.getOppositePaneId(targetPaneId));
    const targetPath = path ?? sourcePane?.currentPath;
    if (!targetPath) return;
    targetPane?.followSizeManagerNavigation(targetPath);
  }

  // Status bar data — read from the active pane reactively via template bindings
  get statusFilteredEntries() { return this.getActivePane()?.filteredEntries ?? []; }
  get statusAllEntries()      { return this.getActivePane()?.entries ?? []; }
  get statusSelectedPaths()   { return this.getActivePane()?.selectedFiles ?? new Set<string>(); }

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

  private get layoutHost(): FileBrowserLayoutHost {
    return {
      getLeftPaneWidth: () => this.leftPaneWidth,
      getRightPaneWidth: () => this.rightPaneWidth,
      setPaneWidths: (left: number, right: number) => {
        this.leftPaneWidth = left;
        this.rightPaneWidth = right;
      },
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
          this.leftPane?.browserCommands.navigation.refresh(),
          this.rightPane?.browserCommands.navigation.refresh(),
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

  private get searchHost(): FileBrowserSearchHost {
	return {
	  pathHistoryService: this.pathHistoryService,
    apiService: this.apiService,
	  getActivePane: () => this.getActivePane(),
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
    this.searchController.onPanePathChange(this.searchHost, paneId, path);
    this.syncSizeManagerPane(this.getOppositePaneId(paneId as 'left' | 'right'), path);
  }

  async onGlobalSearchResult(result: any): Promise<void> {
    await this.searchController.onGlobalSearchResult(this.searchHost, result);
  }

  async onGlobalSearchFolder(query: string): Promise<void> {
    await this.searchController.openGlobalSearchAsFolder(this.searchHost, query);
  }

  // ============================================================================
  // Keyboard Shortcuts
  // ============================================================================

  private setupBrowserCommands(): () => void {
    const handlers: FileBrowserCommandHandlers = {
      openGlobalSearch:      () => this.globalSearch?.show(),
      showPathHistory:       () => this.pathHistoryViewer?.show(),
      quickPathJump:         () => console.log('Quick path jump - to be implemented'),
      navigateUp:            () => this.getActivePane()?.browserCommands.navigation.navigateUp(),
      refresh:               () => this.getActivePane()?.browserCommands.navigation.refresh(),
      showKeyboardHelp:      () => this.keyboardHelp?.show(),
      cursorUp:              () => this.getActivePane()?.shortcuts.selection.moveCursorUp(),
      cursorDown:            () => this.getActivePane()?.shortcuts.selection.moveCursorDown(),
      cursorLeft:            () => this.getActivePane()?.shortcuts.selection.moveCursorLeft(),
      cursorRight:           () => this.getActivePane()?.shortcuts.selection.moveCursorRight(),
      cursorHome:            () => this.getActivePane()?.shortcuts.selection.moveCursorHome(),
      cursorEnd:             () => this.getActivePane()?.shortcuts.selection.moveCursorEnd(),
      cursorPageUp:          () => this.getActivePane()?.shortcuts.selection.moveCursorPageUp(),
      cursorPageDown:        () => this.getActivePane()?.shortcuts.selection.moveCursorPageDown(),
      newTab:                () => this.getActivePane()?.onNewTab(),
      closeTab:              () => this.getActivePane()?.shortcuts.tabs.closeCurrentTab(),
      nextTab:               () => this.getActivePane()?.shortcuts.tabs.nextTab(),
      prevTab:               () => this.getActivePane()?.shortcuts.tabs.previousTab(),
      jumpToTab:             (index) => this.getActivePane()?.shortcuts.tabs.jumpToTab(index),
      togglePinTab:          () => this.getActivePane()?.shortcuts.tabs.togglePinCurrentTab(),
      openItem:              () => this.getActivePane()?.browserCommands.navigation.openSelectedItem(),
      quickView:             () => this.previewController.openFrozenPreviewInOtherPane(this.previewHost),
      copy:                  () => this.copySelectedToClipboard(),
      cut:                   () => this.cutSelectedToClipboard(),
      paste:                 () => this.pasteFromClipboard(),
      deleteSelected:        () => this.getActivePane()?.browserCommands.selection.deleteSelected(),
      selectAll:             () => this.getActivePane()?.shortcuts.selection.selectAll(),
      deselectAll:           () => this.getActivePane()?.shortcuts.selection.deselectAll(),
      newFolder:             () => this.getActivePane()?.browserCommands.selection.createFolder(),
      rename:                () => this.getActivePane()?.browserCommands.selection.renameSelected(),
      toggleCurrent:         () => this.getActivePane()?.shortcuts.selection.toggleCurrentAndMoveDown(),
      inlineDirectorySize:   () => this.getActivePane()?.measureCurrentDirectorySizeAndMoveDown(),
      extendSelUp:           () => this.getActivePane()?.shortcuts.selection.extendSelection(-1),
      extendSelDown:         () => this.getActivePane()?.shortcuts.selection.extendSelection(1),
      selectByMask:          () => this.getActivePane()?.shortcuts.selection.selectByMask(),
      unselectByMask:        () => this.getActivePane()?.shortcuts.selection.unselectByMask(),
      invertSelection:       () => this.getActivePane()?.shortcuts.selection.invertSelection(),
      selectSameExt:         () => this.getActivePane()?.shortcuts.selection.selectSameExtension(),
      focusFilter:           () => this.getActivePane()?.shortcuts.focus.focusFilter(),
      advancedFilter:        () => console.log('Advanced filter - to be implemented'),
      clearFilter:           () => this.getActivePane()?.shortcuts.focus.clearFilter(),
      showWorkspaceSwitcher: () => this.toggleWorkspaceMenu(),
      newWorkspace:          () => this.createNewWorkspace(),
      toggleHidden:          () => this.toggleShowHidden(),
      switchPane:            () => this.previewController.switchActivePane(this.previewHost),
      copyToOtherPane:       () => this.copyToOtherPaneCurrent(),
      moveToOtherPane:       () => this.moveToOtherPaneCurrent(),
      focusAddressBar:       () => this.getActivePane()?.shortcuts.focus.focusAddressBar(),
      diskUsage:             () => {
        const sourcePane = this.getActivePane();
        const sourcePath = sourcePane?.currentPath;
        const targetPane = this.getOtherPane();
        targetPane?.openDiskUsageAnalyzer(sourcePath ?? undefined);
      },
    };
    return registerFileBrowserCommands(this.inputEngine, handlers, 'file-browser');
  }

  private registerMouseBindings(): void {
    this.inputEngine.registerCommand('global.mouse.fileEntry.openSelected', () => {
      this.getActivePane()?.browserCommands.navigation.openSelectedItem();
    }, FileBrowserComponent.mouseCommandOwner);

    this.inputEngine.registerCommand('global.mouse.viewer.zoomIn', () => {
      this.getActivePane()?.browserCommands.preview.zoomInActiveViewer();
    }, FileBrowserComponent.mouseCommandOwner);

    this.inputEngine.registerCommand('global.mouse.viewer.zoomOut', () => {
      this.getActivePane()?.browserCommands.preview.zoomOutActiveViewer();
    }, FileBrowserComponent.mouseCommandOwner);

    const bindings: InputBindingDefinition[] = [
      {
        id: 'global.mouse.fileEntry.doubleClickOpen',
        contextId: 'global',
        commandId: 'global.mouse.fileEntry.openSelected',
        source: 'default',
        description: 'Open the currently selected entry on double click',
        when: 'fileEntry',
        trigger: {
          device: 'mouse',
          gesture: { type: 'doubleClick', button: 'left' },
        },
      },
      {
        id: 'global.mouse.viewer.ctrlWheelUp',
        contextId: 'global',
        commandId: 'global.mouse.viewer.zoomIn',
        source: 'default',
        description: 'Zoom the active viewer in with Ctrl+wheel up',
        when: 'viewerSurface',
        trigger: {
          device: 'mouse',
          gesture: { type: 'wheel', direction: 'up', modifiers: ['ctrl'] },
        },
      },
      {
        id: 'global.mouse.viewer.ctrlWheelDown',
        contextId: 'global',
        commandId: 'global.mouse.viewer.zoomOut',
        source: 'default',
        description: 'Zoom the active viewer out with Ctrl+wheel down',
        when: 'viewerSurface',
        trigger: {
          device: 'mouse',
          gesture: { type: 'wheel', direction: 'down', modifiers: ['ctrl'] },
        },
      },
    ];

    for (const binding of bindings) {
      this.inputEngine.registerBinding(binding);
    }
  }

  private unregisterMouseBindings(): void {
    this.inputEngine.unregisterCommand('global.mouse.fileEntry.openSelected', FileBrowserComponent.mouseCommandOwner);
    this.inputEngine.unregisterCommand('global.mouse.viewer.zoomIn', FileBrowserComponent.mouseCommandOwner);
    this.inputEngine.unregisterCommand('global.mouse.viewer.zoomOut', FileBrowserComponent.mouseCommandOwner);

    for (const bindingId of FileBrowserComponent.mouseBindingIds) {
      this.inputEngine.unregisterBinding(bindingId);
    }
  }

  formatPendingChord(steps: unknown): string {
    const sequence = Array.isArray((steps as { steps?: unknown } | null | undefined)?.steps)
      ? (steps as { steps: unknown[] }).steps
      : (Array.isArray(steps) ? steps : null);

    if (!sequence) {
      return '';
    }

    return sequence.map((step) => {
      const record = (step && typeof step === 'object') ? step as { key?: unknown; modifiers?: unknown } : {};
      const key = typeof record.key === 'string' ? record.key : '';
      const modifiers = Array.isArray(record.modifiers)
        ? record.modifiers.filter((value: unknown): value is string => typeof value === 'string')
        : [];
      const parts = [...modifiers.map((modifier: string) => {
        switch (modifier) {
          case 'ctrl': return 'Ctrl';
          case 'shift': return 'Shift';
          case 'alt': return 'Alt';
          case 'meta': return 'Win';
          default: return modifier;
        }
      })];

      const keyMap: Record<string, string> = {
        arrowup: 'Up',
        arrowdown: 'Down',
        arrowleft: 'Left',
        arrowright: 'Right',
        escape: 'Esc',
        space: 'Space',
      };

      parts.push(keyMap[key] ?? key.toUpperCase());
      return parts.join('+');
    }).join(' ');
  }

  onPanePreviewTarget(fromPane: 'left' | 'right', targetPath: string): void {
    this.previewController.onPanePreviewTarget(this.previewHost, fromPane, targetPath);
  }

  onPaneActiveTabChanged(fromPane: 'left' | 'right', tab: TabInfo): void {
    this.previewController.onPaneActiveTabChanged(this.previewHost, fromPane, tab);
    if ((tab.kind ?? 'dir') === 'size-manager') {
      this.syncSizeManagerPane(fromPane);
      return;
    }
    this.syncSizeManagerPane(this.getOppositePaneId(fromPane));
  }

  // NOTE: we intentionally do not auto-create live preview tabs on cursor move.

  // ============================================================================
  // Resizable Panels
  // ============================================================================

  onPaneDividerPositionChange(position: number): void {
    this.layoutController.syncPaneWidths(this.layoutHost, position);
  }

  @HostListener('window:keydown.arrowleft', ['$event'])
  onArrowLeft(event: any): void {
    if (!document.activeElement?.classList.contains('pane-divider')) return;
    event.preventDefault();
    this.layoutController.nudgeDividerLeft(this.layoutHost);
  }

  @HostListener('window:keydown.arrowright', ['$event'])
  onArrowRight(event: any): void {
    if (!document.activeElement?.classList.contains('pane-divider')) return;
    event.preventDefault();
    this.layoutController.nudgeDividerRight(this.layoutHost);
  }

  statusText: string = 'Ready';
}
