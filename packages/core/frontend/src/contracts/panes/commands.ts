import type { FileInfo } from '../files';
import type { PanePeerState, PanePresentation, PaneState, PaneViewportSnapshot } from './state';

export interface ArchiveTransferResult {
  readonly operation?: 'copy' | 'move';
  readonly succeeded: number;
  readonly total: number;
  readonly failed: number;
  readonly sourceRemoved?: boolean;
  readonly transferredCount?: number;
  readonly destinations?: readonly string[];
}

export interface PaneNavigation {
  refresh(): Promise<void>;
  navigateTo(path: string): Promise<void>;
  navigateUp(): void;
  goBack(): void;
  goForward(): void;
  canGoBack(): boolean;
  canGoForward(): boolean;
  canNavigateToLocation(path: string): boolean;
  navigateLinkedLocation(path: string, options?: unknown): Promise<boolean>;
  currentLocationAvailable(): Promise<boolean>;
  jumpNavigationHistory(path: string): Promise<boolean>;
  focusContent(): void;
  focusLocation(): void;
  focusFilter(): void;
}

export interface PaneSelection {
  readonly selectedEntries: readonly FileInfo[];
  readonly selectedPaths: ReadonlySet<string>;
  readonly cursorPath: string | null;
  readonly cursorEntry: FileInfo | null;
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
  reconcileAddedEntries?(entries: readonly FileInfo[], selectAdded?: boolean): void;
  reconcileRemovedPaths?(paths: readonly string[]): void;
}

export interface PanePresentationPort {
  panePresentation(): PanePresentation;
  applyMirroredPresentation(presentation: PanePresentation, options?: unknown): void;
}

export interface PaneViewportPort {
  viewportSnapshot(): PaneViewportSnapshot;
  syncViewportToPeer(snapshot: PaneViewportSnapshot): void;
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
  reloadWorkspaceTabs(preferredTabId?: string): Promise<void>;
}

export interface PaneViewerCommands {
  openEntry(entry: FileInfo): Promise<void>;
  openViewer(entry: FileInfo, viewerId?: string | null, autoFollow?: boolean): any;
  openSelectedWithViewer(viewerId: string): Promise<void>;
  openQuickView(entry: FileInfo, options?: unknown): void;
  openActiveViewerExternally(): Promise<void>;
  returnToViewerSource(): Promise<void>;
  openResultsPanel(title: string, entries: readonly FileInfo[], metadata?: unknown): Promise<void>;
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

export interface PanePeerNavigation {
  refresh(): Promise<void>;
  canNavigateToLocation(path: string): boolean;
  navigateLinkedLocation(path: string, options?: unknown): Promise<boolean>;
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
