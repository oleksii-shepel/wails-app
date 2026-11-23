import type { FileInfo } from '../files/index';
import type { SourceCapabilities } from '../sources/index';

export type DirectoryView = 'list' | 'grid' | 'details' | 'tree';
export type SortField = 'name' | 'size' | 'modified' | 'type';
export type SortDirection = 'asc' | 'desc';

export interface PanePresentation {
  readonly viewMode: DirectoryView;
  readonly sortField: SortField;
  readonly sortDirection: SortDirection;
  readonly directoriesFirst: boolean;
  readonly showHidden?: boolean;
  readonly filterText: string;
  readonly filterVisible: boolean;
  readonly treeContentMode?: 'all' | 'folders';
}

export interface PaneViewportSnapshot {
  readonly topName: string | null;
  readonly ratio: number;
}

export interface QuickViewSource {
  readonly path?: string;
  readonly fileList?: ReadonlyArray<{ path: string; name: string }>;
  readonly sourcePane: 'left' | 'right';
  readonly sourceTabId: string;
  readonly sourceDir: string;
}

export interface ArchiveTransferResult {
  readonly succeeded: number;
  readonly total: number;
  readonly failed: number;
  readonly sourceRemoved?: boolean;
  readonly success?: boolean;
  readonly message?: string;
  readonly affectedPaths?: string[];
}

export interface PaneState {
  readonly paneId: 'left' | 'right';
  readonly path: string;
  readonly entries: readonly FileInfo[];
  readonly visibleEntries: readonly FileInfo[];
  readonly selectedPaths: ReadonlySet<string>;
  readonly cursorPath: string | null;
  readonly cursorEntry: FileInfo | null;
  readonly selectedEntries: readonly FileInfo[];
  readonly loading: boolean;
  readonly error: string;
  readonly viewMode: DirectoryView;
  readonly activeSurfaceId: string;
  readonly pinnableSurfaceId: string;
  readonly pinnableLocation: string;
  readonly pinnableTitle: string;
  readonly tabs: readonly any[];
  readonly activeTab: any;
  readonly activeViewerTab: any;
  readonly mountedViewer: any;
  readonly viewerArchiveContext: any;
  readonly currentArchiveFrame: any;
  readonly activePanelSource: any;
  readonly sourceCapabilities: SourceCapabilities;
  readonly isDirectoryTab?: boolean;
  readonly isInArchive?: boolean;
  readonly hasSelection: boolean;
  readonly canCreate?: boolean;
  readonly canRename?: boolean;
  readonly canDelete?: boolean;
  readonly canCut?: boolean;
  readonly canPaste?: boolean;
}

export interface PaneNavigation {
  navigateTo(path: string, options?: any): Promise<any>;
  refresh(): Promise<void>;
  goBack(): Promise<void> | void;
  goForward(): Promise<void> | void;
  goUp?(): Promise<void> | void;
  navigateUp(): void;
  focusContent(): void;
  focusLocation(): void;
  focusFilter(): void;
  canGoBack(): boolean;
  canGoForward(): boolean;
  currentLocationAvailable(): Promise<boolean>;
  jumpNavigationHistory(path: string): Promise<boolean>;
  canNavigateToLocation(path: string): boolean;
  navigateLinkedLocation(path: string, options?: any): Promise<boolean>;
}

export interface PaneSelection {
  readonly selectedPaths: ReadonlySet<string>;
  readonly cursorPath: string | null;
  readonly cursorEntry: FileInfo | null;
  readonly selectedEntries: readonly FileInfo[];
  clearSelection(): void;
  selectAll(): void;
  selectPaths(paths: readonly string[]): void;
  selectPeerByName(name: string): boolean;
  selectFromPointer?(entry: FileInfo, event: MouseEvent): void;
  selectComparisonItem(target: string): void;
  recoverVisibleCursor(): void;
  moveCursor(delta: number, extend?: boolean): void;
  moveCursorHome(extend?: boolean): void;
  moveCursorEnd(extend?: boolean): void;
  moveCursorPage(direction: -1 | 1, extend?: boolean): void;
  toggleCurrentSelection(): void;
  extendSelection(delta: number): void;
  invertSelection(): void;
  selectSameExtension(): void;
  selectSameType(): void;
  selectFiles(): void;
  selectFolders(): void;
  unselectFiles(): void;
  unselectFolders(): void;
  selectByMaskPrompt(): Promise<void>;
  unselectByMaskPrompt(): Promise<void>;
  calculateSelectedSize(): Promise<void>;
  reconcileAddedEntries?(entries: readonly FileInfo[]): void;
  reconcileRemovedPaths?(paths: readonly string[]): void;
}

export interface PanePresentationPort {
  setViewMode?(mode: DirectoryView): void;
  setSort?(field: SortField, direction?: SortDirection): void;
  toggleHidden?(): void;
  setFilterVisible?(visible: boolean): void;
  panePresentation(): PanePresentation;
  applyMirroredPresentation(presentation: PanePresentation, options?: any): void;
}

export interface PaneViewportPort {
  getViewportSnapshot?(): PaneViewportSnapshot;
  restoreViewportSnapshot?(snapshot: PaneViewportSnapshot): void;
  viewportSnapshot(): PaneViewportSnapshot;
  syncViewportToPeer(snapshot: PaneViewportSnapshot): void;
  focusPane?(): void;
}

export interface PaneTabs {
  openTab?(path?: string): void;
  closeTab?(tabId?: string): void;
  newTab(): void;
  closeActiveTab(): void;
  nextTab(): void;
  previousTab(): void;
  prevTab?(): void;
  jumpToTab(index: number): void;
  closeOtherTabs(): void;
  closeTabsToRight(): void;
  toggleActiveTabPin(): void;
  exportActiveTabId(): string | null;
  exportTabCount(): number;
  restoreWorkspaceState(): Promise<void>;
  reloadWorkspaceTabs(preferredTabId?: string): Promise<void>;
}

export interface PaneViewerCommands {
  openEntry(entry: FileInfo): Promise<void> | void;
  openViewer(entry: FileInfo, viewerId?: string | null, autoFollow?: boolean): any;
  openSelectedWithViewer(viewerId: string): Promise<void>;
  openQuickView(entry: FileInfo, source?: QuickViewSource): void;
  closeQuickView?(): void;
  openActiveViewerExternally(): Promise<void>;
  returnToViewerSource(): Promise<void>;
  openResultsPanel(title: string, entries: readonly FileInfo[], metadata?: unknown): Promise<void>;
  openRecentFilesPanel(): Promise<void>;
  openSurfaceTab(surfaceId: string, title?: string, location?: string): Promise<void>;
}

export interface PaneCommands {
  copyTo?(targetDir: string): Promise<ArchiveTransferResult | null | void>;
  moveTo?(targetDir: string): Promise<ArchiveTransferResult | null | void>;
  copySelected(): void;
  cutSelected(): void;
  paste(): Promise<void>;
  deleteSelected(useTrash?: boolean): Promise<void>;
  createFolder?(): Promise<void>;
  renameSelected(): Promise<void>;
  newFolder(): Promise<void>;
  newFile(): Promise<void>;
  newDocumentTemplate(kind: string): Promise<void>;
  moveSelectionToNewFolder(): Promise<void>;
  duplicateSelected(): Promise<void>;
  revealSelectedInFileManager(): Promise<void>;
  touchSelectedModifiedTime(): Promise<void>;
  editSelectedAttributes(): Promise<void>;
  showSelectedProperties(): Promise<void>;
  showSelectedChecksum(): Promise<void>;
  copySelectedPaths(): Promise<void>;
  copySelectedRelativePaths(): Promise<void>;
  copySelectedFileUris(): Promise<void>;
  copySelectedNames(): Promise<void>;
  createLinkForSelected(kind: 'symbolic' | 'hard'): Promise<void>;
  createShortcutForSelected(): Promise<void>;
  createZipFromSelection(): Promise<void>;
  extractSelectedTo(destinationDir: string, flattenPaths?: boolean): Promise<boolean>;
  transferArchiveSelectionTo(destinationDir: string, operation: 'copy' | 'move'): Promise<ArchiveTransferResult | null>;
  treeLeft(): void;
  treeRight(): void;
}

export interface PaneComponent
  extends PaneState,
    PaneNavigation,
    PaneSelection,
    PanePresentationPort,
    PaneViewportPort,
    PaneTabs,
    PaneViewerCommands,
    PaneCommands {}

export interface PaneContextState {
  readonly path: string;
  readonly entries: readonly FileInfo[];
  readonly visibleEntries: readonly FileInfo[];
  readonly activeSurfaceId: string;
  readonly viewMode: DirectoryView;
  readonly sourceCapabilities: SourceCapabilities;
  readonly activePanelSource: any;
  readonly currentArchiveFrame: any;
  readonly isDirectoryTab: boolean;
  readonly isInArchive: boolean;
}

export interface PaneTabState {
  readonly tabs: readonly any[];
  readonly activeTab: any;
  readonly activeViewerTab: any;
}

export interface PaneViewerState {
  readonly mountedViewer: any;
  readonly viewerArchiveContext: any;
}

export interface PanePinState {
  readonly location: string;
  readonly surfaceId: string;
  readonly title: string;
}

export interface PaneMutationState {
  readonly hasSelection: boolean;
  readonly canCreate: boolean;
  readonly canRename: boolean;
  readonly canDelete: boolean;
  readonly canCut: boolean;
  readonly canPaste: boolean;
}

export interface PaneStatePort {
  readonly contextState: PaneContextState;
  readonly tabState: PaneTabState;
  readonly viewerState: PaneViewerState;
  readonly pinState: PanePinState;
  readonly mutationState: PaneMutationState;
}

export interface PanePeerState {
  readonly path: string;
  readonly entries: readonly FileInfo[];
  readonly cursorEntry: FileInfo | null;
  readonly selectedPaths: ReadonlySet<string>;
  readonly sourceCapabilities: SourceCapabilities;
  readonly isDirectoryTab: boolean;
  readonly isInArchive: boolean;
}

export interface PanePeerNavigation {
  refresh(): Promise<void>;
  canNavigateToLocation(path: string): boolean;
  navigateLinkedLocation(path: string, options?: any): Promise<boolean>;
}

export interface PanePeerSelection {
  selectPeerByName(name: string): boolean;
  selectPaths(paths: readonly string[]): void;
}

export interface PanePeerPort {
  readonly peerState: PanePeerState;
  readonly peerNavigation: PanePeerNavigation;
  readonly peerSelection: PanePeerSelection;
}
