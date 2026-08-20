import type { SourceCapabilities } from '../services/source-capabilities';
import type { FileInfo } from '../shared/protocol';

export type DirectoryView = 'list' | 'grid' | 'details' | 'tree';
export type SortField = 'name' | 'size' | 'modified' | 'type';
export type SortDirection = 'asc' | 'desc';

export interface PanePresentation {
  readonly viewMode: DirectoryView;
  readonly sortField: SortField;
  readonly sortDirection: SortDirection;
  readonly directoriesFirst: boolean;
  readonly filterText: string;
  readonly filterVisible: boolean;
}

export type PaneViewportSnapshot = {
  topName: string | null;
  ratio: number;
};

export interface PaneState {
  readonly path: string;
  readonly entries: FileInfo[];
  readonly tabs: readonly unknown[];
  readonly activeTab: any;
  readonly activeViewerTab: any;
  readonly activeSurfaceId: string;
  readonly viewMode: string;
  readonly visibleEntries: readonly FileInfo[];
  readonly sourceCapabilities: SourceCapabilities;
  readonly activePanelSource: any;
  readonly mountedViewer: any;
  readonly viewerArchiveContext: any;
  readonly currentArchiveFrame: any;
  readonly pinnableLocation: string;
  readonly pinnableSurfaceId: string;
  readonly pinnableTitle: string;
  readonly hasSelection: boolean;
  readonly canCreate: boolean;
  readonly canRename: boolean;
  readonly canDelete: boolean;
  readonly canCut: boolean;
  readonly canPaste: boolean;
  readonly isInArchive: boolean;
  readonly isDirectoryTab: boolean;
  lastAction: string;
}

export interface PaneNavigation {
  refresh(): Promise<void>;
  focusContent(): void;
  focusLocation(): void;
  focusFilter(): void;
  goBack(): void;
  goForward(): void;
  navigateUp(): void;
  navigateTo(path: string): void | Promise<void>;
  navigateLinkedLocation(path: string, options?: unknown): Promise<any>;
  canNavigateToLocation(path: string): boolean | Promise<boolean>;
  currentLocationAvailable(): Promise<boolean>;
  jumpNavigationHistory(path: string): Promise<any>;
}

export interface PaneSelection {
  readonly cursorEntry: FileInfo | null;
  readonly cursorPath: string | null;
  readonly selectedPaths: ReadonlySet<string>;
  readonly selectedEntries: FileInfo[];
  moveCursor(delta: number, extend?: boolean): void;
  moveCursorHome(extend?: boolean): void;
  moveCursorEnd(extend?: boolean): void;
  moveCursorPage(direction: -1 | 1, extend?: boolean): void;
  selectAll(): void;
  clearSelection(): void;
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
  selectPaths(paths: readonly string[]): void;
  selectComparisonItem(path: string): void;
  recoverVisibleCursor(): void;
  calculateSelectedSize(): Promise<void>;
  selectPeerByName(name: string): boolean;
}

export interface PanePresentationPort {
  panePresentation(): PanePresentation;
  viewportSnapshot(): PaneViewportSnapshot;
  syncViewportToPeer(snapshot: PaneViewportSnapshot): void;
  applyMirroredPresentation(presentation: PanePresentation, options?: unknown): void;
}

export interface PaneTabs {
  closeActiveTab(): void;
  newTab(): void;
  nextTab(): void;
  previousTab(): void;
  jumpToTab(index: number): void;
  closeOtherTabs(): void;
  closeTabsToRight(): void;
  toggleActiveTabPin(): void;
  exportActiveTabId(): string | null;
  exportTabCount(): number;
  restoreWorkspaceState(): Promise<void>;
  reloadWorkspaceTabs(preferredTabId?: string): Promise<any>;
}

export interface PaneViewerCommands {
  openEntry(entry: FileInfo): void | Promise<void>;
  openViewer(entry: FileInfo, viewerId?: string | null, autoFollow?: boolean): any;
  openSelectedWithViewer(viewerId: string): Promise<void>;
  openQuickView(entry: FileInfo): void;
  openActiveViewerExternally(): Promise<void>;
  returnToViewerSource(): Promise<void>;
  openResultsPanel(title: string, entries: readonly FileInfo[], metadata?: unknown): Promise<any>;
  openRecentFilesPanel(): Promise<void>;
  openSurfaceTab(surfaceId: string, title?: string, location?: string): Promise<void>;
}

export interface PaneCommands {
  copySelected(): void;
  cutSelected(): void;
  paste(): Promise<void>;
  deleteSelected(permanent?: boolean): Promise<void>;
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
  transferArchiveSelectionTo(destinationDir: string, operation: 'copy' | 'move'): Promise<any>;
  treeLeft(): void;
  treeRight(): void;
}

export interface PaneComponent
  extends PaneState,
    PaneNavigation,
    PaneSelection,
    PanePresentationPort,
    PaneTabs,
    PaneViewerCommands,
    PaneCommands {}
