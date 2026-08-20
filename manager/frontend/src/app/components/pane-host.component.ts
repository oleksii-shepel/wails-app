import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output, ViewChild } from '@angular/core';

import { DirectoryPaneComponent, type PanePresentation } from './directory-pane.component';
import type { PresentedCommand } from './presented-command';
import type { PaneComparisonStatus } from '../services/pane-comparison.service';
import type { FileInfo } from '../shared/protocol';
import { getSourceCapabilities, type SourceCapabilities } from '../services/source-capabilities';

@Component({
  selector: 'app-pane-host',
  standalone: true,
  imports: [CommonModule, DirectoryPaneComponent],
  template: `
    <app-directory-pane
      #component
      [paneId]="paneId"
      [active]="active"
      [showHidden]="showHidden"
      [currentPathPinned]="currentPathPinned"
      [pluginToolbarActions]="pluginToolbarActions"
      [comparisonStatuses]="comparisonStatuses"
      (activateRequested)="activateRequested.emit()"
      (navigationTrailRequested)="navigationTrailRequested.emit($event)"
      (remoteRecoveryRequested)="remoteRecoveryRequested.emit()"
      (togglePinRequested)="togglePinRequested.emit()"
      (locationChange)="locationChange.emit($event)"
      (cursorChange)="cursorChange.emit($event)"
      (presentationChange)="presentationChange.emit($event)"
      (viewportChange)="viewportChange.emit($event)"
      (entryContextMenu)="entryContextMenu.emit($event)"
      (backgroundContextMenu)="backgroundContextMenu.emit($event)"
      (internalDrop)="internalDrop.emit($event)">
    </app-directory-pane>
  `,
})
export class PaneHostComponent {
  @ViewChild('component') private inner?: DirectoryPaneComponent;

  @Input() paneId: 'left' | 'right' = 'left';
  @Input() active = false;
  @Input() showHidden = false;
  @Input() currentPathPinned = false;
  @Input() pluginToolbarActions: readonly PresentedCommand[] = [];
  @Input() comparisonStatuses: ReadonlyMap<string, PaneComparisonStatus> =
    new Map<string, PaneComparisonStatus>();

  @Output() readonly activateRequested = new EventEmitter<void>();
  @Output() readonly navigationTrailRequested = new EventEmitter<{
    direction: 'back' | 'forward';
    event: MouseEvent;
    entries: readonly string[];
  }>();
  @Output() readonly remoteRecoveryRequested = new EventEmitter<void>();
  @Output() readonly togglePinRequested = new EventEmitter<void>();
  @Output() readonly locationChange = new EventEmitter<string>();
  @Output() readonly cursorChange = new EventEmitter<FileInfo | null>();
  @Output() readonly presentationChange = new EventEmitter<PanePresentation>();
  @Output() readonly viewportChange = new EventEmitter<{
    topName: string | null;
    ratio: number;
  }>();
  @Output() readonly entryContextMenu = new EventEmitter<{
    event: MouseEvent;
    entry: FileInfo;
  }>();
  @Output() readonly backgroundContextMenu = new EventEmitter<MouseEvent>();
  @Output() readonly internalDrop = new EventEmitter<{
    event: DragEvent;
    targetPath: string;
    targetEntry: FileInfo | null;
  }>();

  get component(): DirectoryPaneComponent | undefined { return this.inner; }
  get path(): string { return this.inner?.path ?? ''; }
  get entries(): FileInfo[] { return this.inner?.entries ?? []; }
  get tabs() { return this.inner?.tabs ?? []; }
  get activeTab() { return this.inner?.activeTab; }
  get activeViewerTab() { return this.inner?.activeViewerTab; }
  get activeSurfaceId(): string { return this.inner?.activeSurfaceId ?? ''; }
  get viewMode() { return this.inner?.viewMode ?? 'list'; }
  get visibleEntries(): readonly FileInfo[] { return this.inner?.visibleEntries ?? []; }
  get cursorEntry(): FileInfo | null { return this.inner?.cursorEntry ?? null; }
  get cursorPath(): string | null { return this.inner?.cursorPath ?? null; }
  get selectedPaths(): ReadonlySet<string> { return this.inner?.selectedPaths ?? new Set<string>(); }
  get selectedEntries(): FileInfo[] { return this.inner?.selectedEntries ?? []; }
  get sourceCapabilities(): SourceCapabilities {
    return this.inner?.sourceCapabilities ?? getSourceCapabilities(this.path);
  }
  get activePanelSource(): any { return this.inner?.activePanelSource; }
  get mountedViewer() { return this.inner?.mountedViewer; }
  get viewerArchiveContext() { return this.inner?.viewerArchiveContext ?? null; }
  get currentArchiveFrame() { return (this.inner as any)?.currentArchiveFrame ?? null; }
  get pinnableLocation(): string { return this.inner?.pinnableLocation ?? ''; }
  get pinnableSurfaceId(): string { return this.inner?.pinnableSurfaceId ?? 'dir'; }
  get pinnableTitle(): string { return this.inner?.pinnableTitle ?? ''; }
  get hasSelection(): boolean { return !!this.inner?.hasSelection; }
  get canCreate(): boolean { return !!this.inner?.canCreate; }
  get canRename(): boolean { return !!this.inner?.canRename; }
  get canDelete(): boolean { return !!this.inner?.canDelete; }
  get canCut(): boolean { return !!this.inner?.canCut; }
  get canPaste(): boolean { return !!this.inner?.canPaste; }
  get isInArchive(): boolean { return !!this.inner?.isInArchive; }
  get isDirectoryTab(): boolean { return !!this.inner?.isDirectoryTab; }

  get lastAction(): string { return this.inner?.lastAction ?? ''; }
  set lastAction(value: string) {
    if (this.inner) this.inner.lastAction = value;
  }

  refresh(): Promise<void> { return this.inner?.refresh() ?? Promise.resolve(); }
  focusContent(): void { this.inner?.focusContent(); }
  focusLocation(): void { this.inner?.focusLocation(); }
  focusFilter(): void { this.inner?.focusFilter(); }
  goBack(): void { this.inner?.goBack(); }
  goForward(): void { this.inner?.goForward(); }
  navigateUp(): void { this.inner?.navigateUp(); }
  navigateTo(path: string): Promise<void> { return this.inner?.navigateTo(path) ?? Promise.resolve(); }
  navigateLinkedLocation(path: string, options?: unknown): Promise<any> { return (this.inner as any)?.navigateLinkedLocation?.(path, options) ?? Promise.resolve(false); }
  canNavigateToLocation(path: string): boolean { return (this.inner as any)?.canNavigateToLocation?.(path) ?? false; }
  currentLocationAvailable(): Promise<boolean> { return (this.inner as any)?.currentLocationAvailable?.() ?? Promise.resolve(false); }
  jumpNavigationHistory(path: string): Promise<any> { return (this.inner as any)?.jumpNavigationHistory?.(path) ?? Promise.resolve(false); }
  panePresentation(): PanePresentation { return (this.inner as any)?.panePresentation?.() ?? { viewMode: 'list', sortField: 'name', sortDirection: 'asc', directoriesFirst: true, filterText: '', filterVisible: false }; }
  viewportSnapshot(): { topName: string | null; ratio: number } { return (this.inner as any)?.viewportSnapshot?.() ?? { topName: null, ratio: 0 }; }
  syncViewportToPeer(snapshot: { topName: string | null; ratio: number }): void { (this.inner as any)?.syncViewportToPeer?.(snapshot); }
  applyMirroredPresentation(presentation: PanePresentation, options?: unknown): void { (this.inner as any)?.applyMirroredPresentation?.(presentation, options); }
  moveCursor(delta: number, extend = false): void { (this.inner as any)?.moveCursor?.(delta, extend); }
  moveCursorHome(extend = false): void { (this.inner as any)?.moveCursorHome?.(extend); }
  moveCursorEnd(extend = false): void { (this.inner as any)?.moveCursorEnd?.(extend); }
  moveCursorPage(direction: -1 | 1, extend = false): void { (this.inner as any)?.moveCursorPage?.(direction, extend); }
  openEntry(entry: FileInfo): Promise<void> { return (this.inner as any)?.openEntry?.(entry) ?? Promise.resolve(); }
  openViewer(entry: FileInfo, viewerId?: string | null, autoFollow?: boolean): any { return (this.inner as any)?.openViewer?.(entry, viewerId, autoFollow); }
  openSelectedWithViewer(viewerId: string): Promise<void> { return (this.inner as any)?.openSelectedWithViewer?.(viewerId) ?? Promise.resolve(); }
  openQuickView(entry: FileInfo): void { (this.inner as any)?.openQuickView?.(entry); }
  openActiveViewerExternally(): Promise<void> { return (this.inner as any)?.openActiveViewerExternally?.() ?? Promise.resolve(); }
  returnToViewerSource(): Promise<void> { return (this.inner as any)?.returnToViewerSource?.() ?? Promise.resolve(); }
  closeActiveTab(): void { (this.inner as any)?.closeActiveTab?.(); }
  newTab(): void { (this.inner as any)?.newTab?.(); }
  nextTab(): void { (this.inner as any)?.nextTab?.(); }
  previousTab(): void { (this.inner as any)?.previousTab?.(); }
  jumpToTab(index: number): void { (this.inner as any)?.jumpToTab?.(index); }
  closeOtherTabs(): void { (this.inner as any)?.closeOtherTabs?.(); }
  closeTabsToRight(): void { (this.inner as any)?.closeTabsToRight?.(); }
  toggleActiveTabPin(): void { (this.inner as any)?.toggleActiveTabPin?.(); }
  exportActiveTabId(): string | null { return (this.inner as any)?.exportActiveTabId?.() ?? null; }
  exportTabCount(): number { return (this.inner as any)?.exportTabCount?.() ?? 0; }
  restoreWorkspaceState(): Promise<void> { return (this.inner as any)?.restoreWorkspaceState?.() ?? Promise.resolve(); }
  reloadWorkspaceTabs(preferredTabId?: string): Promise<any> { return (this.inner as any)?.reloadWorkspaceTabs?.(preferredTabId) ?? Promise.resolve(); }
  selectAll(): void { (this.inner as any)?.selectAll?.(); }
  clearSelection(): void { (this.inner as any)?.clearSelection?.(); }
  toggleCurrentSelection(): void { (this.inner as any)?.toggleCurrentSelection?.(); }
  extendSelection(delta: number): void { (this.inner as any)?.extendSelection?.(delta); }
  invertSelection(): void { (this.inner as any)?.invertSelection?.(); }
  selectSameExtension(): void { (this.inner as any)?.selectSameExtension?.(); }
  selectSameType(): void { (this.inner as any)?.selectSameType?.(); }
  selectFiles(): void { (this.inner as any)?.selectFiles?.(); }
  selectFolders(): void { (this.inner as any)?.selectFolders?.(); }
  unselectFiles(): void { (this.inner as any)?.unselectFiles?.(); }
  unselectFolders(): void { (this.inner as any)?.unselectFolders?.(); }
  selectByMaskPrompt(): Promise<void> { return (this.inner as any)?.selectByMaskPrompt?.() ?? Promise.resolve(); }
  unselectByMaskPrompt(): Promise<void> { return (this.inner as any)?.unselectByMaskPrompt?.() ?? Promise.resolve(); }
  selectPaths(paths: readonly string[]): void { (this.inner as any)?.selectPaths?.(paths); }
  selectComparisonItem(path: string): void { (this.inner as any)?.selectComparisonItem?.(path); }
  recoverVisibleCursor(): void { (this.inner as any)?.recoverVisibleCursor?.(); }
  calculateSelectedSize(): Promise<void> { return (this.inner as any)?.calculateSelectedSize?.() ?? Promise.resolve(); }
  copySelected(): void { (this.inner as any)?.copySelected?.(); }
  cutSelected(): void { (this.inner as any)?.cutSelected?.(); }
  paste(): Promise<void> { return (this.inner as any)?.paste?.() ?? Promise.resolve(); }
  deleteSelected(permanent?: boolean): Promise<void> { return (this.inner as any)?.deleteSelected?.(permanent) ?? Promise.resolve(); }
  renameSelected(): Promise<void> { return (this.inner as any)?.renameSelected?.() ?? Promise.resolve(); }
  newFolder(): Promise<void> { return (this.inner as any)?.newFolder?.() ?? Promise.resolve(); }
  newFile(): Promise<void> { return (this.inner as any)?.newFile?.() ?? Promise.resolve(); }
  newDocumentTemplate(kind: string): Promise<void> { return (this.inner as any)?.newDocumentTemplate?.(kind) ?? Promise.resolve(); }
  moveSelectionToNewFolder(): Promise<void> { return (this.inner as any)?.moveSelectionToNewFolder?.() ?? Promise.resolve(); }
  duplicateSelected(): Promise<void> { return (this.inner as any)?.duplicateSelected?.() ?? Promise.resolve(); }
  revealSelectedInFileManager(): Promise<void> { return (this.inner as any)?.revealSelectedInFileManager?.() ?? Promise.resolve(); }
  touchSelectedModifiedTime(): Promise<void> { return (this.inner as any)?.touchSelectedModifiedTime?.() ?? Promise.resolve(); }
  editSelectedAttributes(): Promise<void> { return (this.inner as any)?.editSelectedAttributes?.() ?? Promise.resolve(); }
  showSelectedProperties(): Promise<void> { return (this.inner as any)?.showSelectedProperties?.() ?? Promise.resolve(); }
  showSelectedChecksum(): Promise<void> { return (this.inner as any)?.showSelectedChecksum?.() ?? Promise.resolve(); }
  copySelectedPaths(): Promise<void> { return (this.inner as any)?.copySelectedPaths?.() ?? Promise.resolve(); }
  copySelectedRelativePaths(): Promise<void> { return (this.inner as any)?.copySelectedRelativePaths?.() ?? Promise.resolve(); }
  copySelectedFileUris(): Promise<void> { return (this.inner as any)?.copySelectedFileUris?.() ?? Promise.resolve(); }
  copySelectedNames(): Promise<void> { return (this.inner as any)?.copySelectedNames?.() ?? Promise.resolve(); }
  createLinkForSelected(kind: 'symbolic' | 'hard'): Promise<void> { return (this.inner as any)?.createLinkForSelected?.(kind) ?? Promise.resolve(); }
  createShortcutForSelected(): Promise<void> { return (this.inner as any)?.createShortcutForSelected?.() ?? Promise.resolve(); }
  createZipFromSelection(): Promise<void> { return (this.inner as any)?.createZipFromSelection?.() ?? Promise.resolve(); }
  openResultsPanel(title: string, entries: readonly FileInfo[], metadata?: unknown): Promise<any> { return (this.inner as any)?.openResultsPanel?.(title, entries, metadata) ?? Promise.resolve(); }
  openRecentFilesPanel(): Promise<void> { return (this.inner as any)?.openRecentFilesPanel?.() ?? Promise.resolve(); }
  openSurfaceTab(surfaceId: string, title?: string, location?: string): Promise<void> { return (this.inner as any)?.openSurfaceTab?.(surfaceId, title, location) ?? Promise.resolve(); }
  extractSelectedTo(destinationDir: string, flattenPaths = false): Promise<boolean> { return (this.inner as any)?.extractSelectedTo?.(destinationDir, flattenPaths) ?? Promise.resolve(false); }
  transferArchiveSelectionTo(destinationDir: string, operation: 'copy' | 'move'): Promise<any> { return (this.inner as any)?.transferArchiveSelectionTo?.(destinationDir, operation) ?? Promise.resolve(null); }
  treeLeft(): void { (this.inner as any)?.treeLeft?.(); }
  treeRight(): void { (this.inner as any)?.treeRight?.(); }
  selectPeerByName(name: string): boolean { return (this.inner as any)?.selectPeerByName?.(name) ?? false; }
}
