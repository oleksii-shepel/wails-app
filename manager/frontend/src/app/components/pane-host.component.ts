import { CommonModule } from '@angular/common';
import { Component, EventEmitter, Input, Output, ViewChild } from '@angular/core';

import { DirectoryPaneComponent } from './directory-pane.component';
import type {
  PaneCommands,
  PaneComponent,
  PaneNavigation,
  PanePresentation,
  PanePresentationPort,
  PaneSelection,
  PaneTabs,
  PaneViewerCommands,
  PaneViewportSnapshot,
} from './pane-component.contract';
import { RegistryPaneComponent } from './registry-pane.component';
import type { PresentedCommand } from './presented-command';
import type { PaneComparisonStatus } from '../services/pane-comparison.service';
import type { FileInfo } from '../shared/protocol';
import { getSourceCapabilities, type SourceCapabilities } from '../services/source-capabilities';
import { WorkspaceService } from '../services/workspace.service';
import type { PanePeerNavigation, PanePeerPort, PanePeerSelection, PanePeerState } from './pane-peer.port';

@Component({
  selector: 'app-pane-host',
  standalone: true,
  imports: [CommonModule, DirectoryPaneComponent, RegistryPaneComponent],
  template: `
    <ng-container *ngIf="useRegistryPane; else directoryPane">
      <app-registry-pane
        #registry
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
      </app-registry-pane>
    </ng-container>

    <ng-template #directoryPane>
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
    </ng-template>
  `,
})
export class PaneHostComponent implements PanePeerPort {
  @ViewChild('component') private inner?: DirectoryPaneComponent;
  @ViewChild('registry') private registry?: RegistryPaneComponent;

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

  constructor(private readonly workspaceService: WorkspaceService) {}

  get useRegistryPane(): boolean {
    const path = this.workspaceService.getActiveTab(this.paneId)?.path ?? '';
    return /^registry:\/\//i.test(path);
  }

  private get activeComponent(): PaneComponent | undefined {
    return this.registry ?? (this.inner as unknown as PaneComponent | undefined);
  }

  get component(): PaneComponent | undefined { return this.activeComponent; }

  get peerState(): PanePeerState {
    const host = this;
    return {
      get path() { return host.path; },
      get entries() { return host.entries; },
      get cursorEntry() { return host.cursorEntry; },
      get selectedPaths() { return host.selectedPaths; },
      get sourceCapabilities() { return host.sourceCapabilities; },
      get isDirectoryTab() { return host.isDirectoryTab; },
      get isInArchive() { return host.isInArchive; },
    };
  }

  get peerNavigation(): PanePeerNavigation {
    return {
      refresh: () => this.refresh(),
      canNavigateToLocation: path => this.canNavigateToLocation(path),
      navigateLinkedLocation: (path, options) =>
        this.navigateLinkedLocation(path, options).then(Boolean),
    };
  }

  get peerSelection(): PanePeerSelection {
    return {
      selectPeerByName: name => this.selectPeerByName(name),
      selectPaths: paths => this.selectPaths(paths),
    };
  }

  // Transitional capability ports. New shell code should depend on the narrow
  // capability it needs instead of the full PaneComponent compatibility facade.
  get navigation(): PaneNavigation | undefined { return this.activeComponent; }
  get selection(): PaneSelection | undefined { return this.activeComponent; }
  get commands(): PaneCommands | undefined { return this.activeComponent; }
  get presentation(): PanePresentationPort | undefined { return this.activeComponent; }
  get tabCommands(): PaneTabs | undefined { return this.activeComponent; }
  get viewerCommands(): PaneViewerCommands | undefined { return this.activeComponent; }
  get path(): string { return this.activeComponent?.path ?? ''; }
  get entries(): FileInfo[] { return this.activeComponent?.entries ?? []; }
  get tabs() { return this.activeComponent?.tabs ?? []; }
  get activeTab() { return this.activeComponent?.activeTab; }
  get activeViewerTab() { return this.activeComponent?.activeViewerTab; }
  get activeSurfaceId(): string { return this.activeComponent?.activeSurfaceId ?? ''; }
  get viewMode() { return this.activeComponent?.viewMode ?? 'list'; }
  get visibleEntries(): readonly FileInfo[] { return this.activeComponent?.visibleEntries ?? []; }
  get cursorEntry(): FileInfo | null { return this.activeComponent?.cursorEntry ?? null; }
  get cursorPath(): string | null { return this.activeComponent?.cursorPath ?? null; }
  get selectedPaths(): ReadonlySet<string> { return this.activeComponent?.selectedPaths ?? new Set<string>(); }
  get selectedEntries(): FileInfo[] { return this.activeComponent?.selectedEntries ?? []; }
  get sourceCapabilities(): SourceCapabilities {
    return this.activeComponent?.sourceCapabilities ?? getSourceCapabilities(this.path);
  }
  get activePanelSource(): any { return this.activeComponent?.activePanelSource; }
  get mountedViewer() { return this.activeComponent?.mountedViewer; }
  get viewerArchiveContext() { return this.activeComponent?.viewerArchiveContext ?? null; }
  get currentArchiveFrame() { return this.activeComponent?.currentArchiveFrame ?? null; }
  get pinnableLocation(): string { return this.activeComponent?.pinnableLocation ?? ''; }
  get pinnableSurfaceId(): string { return this.activeComponent?.pinnableSurfaceId ?? 'dir'; }
  get pinnableTitle(): string { return this.activeComponent?.pinnableTitle ?? ''; }
  get hasSelection(): boolean { return !!this.activeComponent?.hasSelection; }
  get canCreate(): boolean { return !!this.activeComponent?.canCreate; }
  get canRename(): boolean { return !!this.activeComponent?.canRename; }
  get canDelete(): boolean { return !!this.activeComponent?.canDelete; }
  get canCut(): boolean { return !!this.activeComponent?.canCut; }
  get canPaste(): boolean { return !!this.activeComponent?.canPaste; }
  get isInArchive(): boolean { return !!this.activeComponent?.isInArchive; }
  get isDirectoryTab(): boolean { return !!this.activeComponent?.isDirectoryTab; }

  get lastAction(): string { return this.activeComponent?.lastAction ?? ''; }
  set lastAction(value: string) {
    if (this.activeComponent) this.activeComponent.lastAction = value;
  }

  refresh(): Promise<void> { return this.activeComponent?.refresh() ?? Promise.resolve(); }
  focusContent(): void { this.activeComponent?.focusContent(); }
  focusLocation(): void { this.activeComponent?.focusLocation(); }
  focusFilter(): void { this.activeComponent?.focusFilter(); }
  goBack(): void { this.activeComponent?.goBack(); }
  goForward(): void { this.activeComponent?.goForward(); }
  navigateUp(): void { this.activeComponent?.navigateUp(); }
  navigateTo(path: string): Promise<void> { return Promise.resolve(this.activeComponent?.navigateTo(path)).then(() => undefined); }
  navigateLinkedLocation(path: string, options?: unknown): Promise<any> { return this.activeComponent?.navigateLinkedLocation(path, options) ?? Promise.resolve(false); }
  canNavigateToLocation(path: string): boolean | Promise<boolean> { return this.activeComponent?.canNavigateToLocation(path) ?? false; }
  currentLocationAvailable(): Promise<boolean> { return this.activeComponent?.currentLocationAvailable() ?? Promise.resolve(false); }
  jumpNavigationHistory(path: string): Promise<any> { return this.activeComponent?.jumpNavigationHistory(path) ?? Promise.resolve(false); }
  panePresentation(): PanePresentation { return this.activeComponent?.panePresentation() ?? { viewMode: 'list', sortField: 'name', sortDirection: 'asc', directoriesFirst: true, filterText: '', filterVisible: false }; }
  viewportSnapshot(): PaneViewportSnapshot { return this.activeComponent?.viewportSnapshot() ?? { topName: null, ratio: 0 }; }
  syncViewportToPeer(snapshot: PaneViewportSnapshot): void { this.activeComponent?.syncViewportToPeer(snapshot); }
  applyMirroredPresentation(presentation: PanePresentation, options?: unknown): void { this.activeComponent?.applyMirroredPresentation(presentation, options); }
  moveCursor(delta: number, extend = false): void { this.activeComponent?.moveCursor(delta, extend); }
  moveCursorHome(extend = false): void { this.activeComponent?.moveCursorHome(extend); }
  moveCursorEnd(extend = false): void { this.activeComponent?.moveCursorEnd(extend); }
  moveCursorPage(direction: -1 | 1, extend = false): void { this.activeComponent?.moveCursorPage(direction, extend); }
  openEntry(entry: FileInfo): Promise<void> { return this.activeComponent?.openEntry(entry) ?? Promise.resolve(); }
  openViewer(entry: FileInfo, viewerId?: string | null, autoFollow?: boolean): any { return this.activeComponent?.openViewer(entry, viewerId, autoFollow); }
  openSelectedWithViewer(viewerId: string): Promise<void> { return this.activeComponent?.openSelectedWithViewer(viewerId) ?? Promise.resolve(); }
  openQuickView(entry: FileInfo): void { this.activeComponent?.openQuickView(entry); }
  openActiveViewerExternally(): Promise<void> { return this.activeComponent?.openActiveViewerExternally() ?? Promise.resolve(); }
  returnToViewerSource(): Promise<void> { return this.activeComponent?.returnToViewerSource() ?? Promise.resolve(); }
  closeActiveTab(): void { this.activeComponent?.closeActiveTab(); }
  newTab(): void { this.activeComponent?.newTab(); }
  nextTab(): void { this.activeComponent?.nextTab(); }
  previousTab(): void { this.activeComponent?.previousTab(); }
  jumpToTab(index: number): void { this.activeComponent?.jumpToTab(index); }
  closeOtherTabs(): void { this.activeComponent?.closeOtherTabs(); }
  closeTabsToRight(): void { this.activeComponent?.closeTabsToRight(); }
  toggleActiveTabPin(): void { this.activeComponent?.toggleActiveTabPin(); }
  exportActiveTabId(): string | null { return this.activeComponent?.exportActiveTabId() ?? null; }
  exportTabCount(): number { return this.activeComponent?.exportTabCount() ?? 0; }
  restoreWorkspaceState(): Promise<void> { return this.activeComponent?.restoreWorkspaceState() ?? Promise.resolve(); }
  reloadWorkspaceTabs(preferredTabId?: string): Promise<any> { return this.activeComponent?.reloadWorkspaceTabs(preferredTabId) ?? Promise.resolve(); }
  selectAll(): void { this.activeComponent?.selectAll(); }
  clearSelection(): void { this.activeComponent?.clearSelection(); }
  toggleCurrentSelection(): void { this.activeComponent?.toggleCurrentSelection(); }
  extendSelection(delta: number): void { this.activeComponent?.extendSelection(delta); }
  invertSelection(): void { this.activeComponent?.invertSelection(); }
  selectSameExtension(): void { this.activeComponent?.selectSameExtension(); }
  selectSameType(): void { this.activeComponent?.selectSameType(); }
  selectFiles(): void { this.activeComponent?.selectFiles(); }
  selectFolders(): void { this.activeComponent?.selectFolders(); }
  unselectFiles(): void { this.activeComponent?.unselectFiles(); }
  unselectFolders(): void { this.activeComponent?.unselectFolders(); }
  selectByMaskPrompt(): Promise<void> { return this.activeComponent?.selectByMaskPrompt() ?? Promise.resolve(); }
  unselectByMaskPrompt(): Promise<void> { return this.activeComponent?.unselectByMaskPrompt() ?? Promise.resolve(); }
  selectPaths(paths: readonly string[]): void { this.activeComponent?.selectPaths(paths); }
  selectComparisonItem(path: string): void { this.activeComponent?.selectComparisonItem(path); }
  recoverVisibleCursor(): void { this.activeComponent?.recoverVisibleCursor(); }
  calculateSelectedSize(): Promise<void> { return this.activeComponent?.calculateSelectedSize() ?? Promise.resolve(); }
  copySelected(): void { this.activeComponent?.copySelected(); }
  cutSelected(): void { this.activeComponent?.cutSelected(); }
  paste(): Promise<void> { return this.activeComponent?.paste() ?? Promise.resolve(); }
  deleteSelected(permanent?: boolean): Promise<void> { return this.activeComponent?.deleteSelected(permanent) ?? Promise.resolve(); }
  renameSelected(): Promise<void> { return this.activeComponent?.renameSelected() ?? Promise.resolve(); }
  newFolder(): Promise<void> { return this.activeComponent?.newFolder() ?? Promise.resolve(); }
  newFile(): Promise<void> { return this.activeComponent?.newFile() ?? Promise.resolve(); }
  newDocumentTemplate(kind: string): Promise<void> { return this.activeComponent?.newDocumentTemplate(kind) ?? Promise.resolve(); }
  moveSelectionToNewFolder(): Promise<void> { return this.activeComponent?.moveSelectionToNewFolder() ?? Promise.resolve(); }
  duplicateSelected(): Promise<void> { return this.activeComponent?.duplicateSelected() ?? Promise.resolve(); }
  revealSelectedInFileManager(): Promise<void> { return this.activeComponent?.revealSelectedInFileManager() ?? Promise.resolve(); }
  touchSelectedModifiedTime(): Promise<void> { return this.activeComponent?.touchSelectedModifiedTime() ?? Promise.resolve(); }
  editSelectedAttributes(): Promise<void> { return this.activeComponent?.editSelectedAttributes() ?? Promise.resolve(); }
  showSelectedProperties(): Promise<void> { return this.activeComponent?.showSelectedProperties() ?? Promise.resolve(); }
  showSelectedChecksum(): Promise<void> { return this.activeComponent?.showSelectedChecksum() ?? Promise.resolve(); }
  copySelectedPaths(): Promise<void> { return this.activeComponent?.copySelectedPaths() ?? Promise.resolve(); }
  copySelectedRelativePaths(): Promise<void> { return this.activeComponent?.copySelectedRelativePaths() ?? Promise.resolve(); }
  copySelectedFileUris(): Promise<void> { return this.activeComponent?.copySelectedFileUris() ?? Promise.resolve(); }
  copySelectedNames(): Promise<void> { return this.activeComponent?.copySelectedNames() ?? Promise.resolve(); }
  createLinkForSelected(kind: 'symbolic' | 'hard'): Promise<void> { return this.activeComponent?.createLinkForSelected(kind) ?? Promise.resolve(); }
  createShortcutForSelected(): Promise<void> { return this.activeComponent?.createShortcutForSelected() ?? Promise.resolve(); }
  createZipFromSelection(): Promise<void> { return this.activeComponent?.createZipFromSelection() ?? Promise.resolve(); }
  openResultsPanel(title: string, entries: readonly FileInfo[], metadata?: unknown): Promise<any> { return this.activeComponent?.openResultsPanel(title, entries, metadata) ?? Promise.resolve(); }
  openRecentFilesPanel(): Promise<void> { return this.activeComponent?.openRecentFilesPanel() ?? Promise.resolve(); }
  openSurfaceTab(surfaceId: string, title?: string, location?: string): Promise<void> { return this.activeComponent?.openSurfaceTab(surfaceId, title, location) ?? Promise.resolve(); }
  extractSelectedTo(destinationDir: string, flattenPaths = false): Promise<boolean> { return this.activeComponent?.extractSelectedTo(destinationDir, flattenPaths) ?? Promise.resolve(false); }
  transferArchiveSelectionTo(destinationDir: string, operation: 'copy' | 'move'): Promise<any> { return this.activeComponent?.transferArchiveSelectionTo(destinationDir, operation) ?? Promise.resolve(null); }
  treeLeft(): void { this.activeComponent?.treeLeft(); }
  treeRight(): void { this.activeComponent?.treeRight(); }
  selectPeerByName(name: string): boolean { return this.activeComponent?.selectPeerByName(name) ?? false; }
}
