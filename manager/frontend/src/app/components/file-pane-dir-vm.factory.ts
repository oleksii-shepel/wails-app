import type { FindWidgetState } from './filter-widget.component';
import type { ViewLayout } from './file-view-container.component';
import type { FileInfo } from '../shared/protocol';
import type { DirSurfaceVm } from '../surfaces/dir/dir-surface.vm';
import type { ActionAvailability } from '../services/action-availability';
import type { PresentedCommand } from './presented-command';
import type { PinnedLocation } from './address-bar.types';

export interface FilePaneDirVmSource {
  get paneId(): 'left' | 'right';
  get pathReady(): boolean;
  get currentPath(): string;
  get loading(): boolean;
  get showPathBar(): boolean;
  get showBreadcrumbs(): boolean;
  get singleClickToOpen(): boolean;
  get showHidden(): boolean;
  get paneActive(): boolean;
  get actionAvailability(): ActionAvailability;
  get toolbarActions(): PresentedCommand[];
  get filteredEntries(): FileInfo[];
  get selectedFiles(): Set<string>;
  get selectionCursorPath(): string | null;
  get error(): string | null;
  get currentView(): ViewLayout;
  get sortField(): string;
  get sortDirection(): 'asc' | 'desc';
  get directoriesFirst(): boolean;
  get colorizeByMime(): boolean;
  get browserPaneSplit(): number | null;
  get archiveMoveWarning(): boolean;
  get archivePendingMoveEntries(): string[];
  set archiveMoveWarning(value: boolean);
  onPathChange(path: string): void;
  onPinnedLocationOpen(pin: PinnedLocation): void;
  onArchiveSegmentClick(path: string): void;
  onNavigateUp(): void;
  onRefresh(): void;
  onViewChange(view: ViewLayout): void;
  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void;
  onDirectoriesFirstChange(value: boolean): void;
  onColorizeByMimeChange(value: boolean): void;
  onBrowserPaneSplitChange(value: number): void;
  onCreateFolder(): void;
  onDeleteSelected(): void;
  onCopyToOther(): void;
  onMoveToOther(): void;
  onFindStateChange(state: FindWidgetState | null): void;
  onFindNext(): void;
  onFindPrev(): void;
  onSelectionChange(entries: FileInfo[]): void;
  onNavigate(entry: FileInfo): void;
  onOpenInNewTab(entry: FileInfo): void;
  onContextMenu(event: { event: MouseEvent; entry: FileInfo }): void;
  onBackgroundContextMenu(event: MouseEvent): void;
  onNodeDoubleClick(entry: FileInfo): void;
  onDragDrop(event: any): void;
  onRename(event: { entry: FileInfo; newName: string }): void;
  executePendingMove(): void;
}

export class FilePaneDirVmFactory {
  create(source: FilePaneDirVmSource): DirSurfaceVm {
    return new (class implements DirSurfaceVm {
      get paneId() { return source.paneId; }
      get pathReady() { return source.pathReady; }
      get currentPath() { return source.currentPath; }
      get loading() { return source.loading; }
      get showPathBar() { return source.showPathBar; }
      get showBreadcrumbs() { return source.showBreadcrumbs; }
      get singleClickToOpen() { return source.singleClickToOpen; }
      get showHidden() { return source.showHidden; }
      get paneActive() { return source.paneActive; }
      get actionAvailability() { return source.actionAvailability; }
      get toolbarActions() { return source.toolbarActions; }
      get entries() { return source.filteredEntries; }
      get selectedFiles() { return source.selectedFiles; }
      get cursorPath() { return source.selectionCursorPath; }
      get error() { return source.error; }
      get currentView() { return source.currentView; }
      get sortField() { return source.sortField; }
      get sortDirection() { return source.sortDirection; }
      get directoriesFirst() { return source.directoriesFirst; }
      get colorizeByMime() { return source.colorizeByMime; }
      get browserPaneSplit() { return source.browserPaneSplit; }
      get archiveMoveWarning() { return source.archiveMoveWarning; }
      get archiveMoveEntryCount() { return source.archivePendingMoveEntries.length; }

      onPathChange = (path: string) => source.onPathChange(path);
      onPinnedLocationOpen = (pin: PinnedLocation) => source.onPinnedLocationOpen(pin);
  onArchiveSegmentClick = (path: string) => source.onArchiveSegmentClick(path);
      onNavigateUp = () => source.onNavigateUp();
      onRefresh = () => source.onRefresh();
      onViewChange = (view: ViewLayout) => source.onViewChange(view);
      onSortChange = (sort: { field: string; direction: 'asc' | 'desc' }) => source.onSortChange(sort);
      onDirectoriesFirstChange = (value: boolean) => source.onDirectoriesFirstChange(value);
      onColorizeByMimeChange = (value: boolean) => source.onColorizeByMimeChange(value);
      onBrowserPaneSplitChange = (value: number) => source.onBrowserPaneSplitChange(value);
      onCreateFolder = () => source.onCreateFolder();
      onDeleteSelected = () => source.onDeleteSelected();
      onCopyToOther = () => source.onCopyToOther();
      onMoveToOther = () => source.onMoveToOther();
      onFindStateChange = (state: FindWidgetState | null) => source.onFindStateChange(state);
      onFindNext = () => source.onFindNext();
      onFindPrev = () => source.onFindPrev();
      onSelectionChange = (entries: FileInfo[]) => source.onSelectionChange(entries);
      onNavigate = (entry: FileInfo) => source.onNavigate(entry);
      onOpenInNewTab = (entry: FileInfo) => source.onOpenInNewTab(entry);
      onContextMenu = (event: { event: MouseEvent; entry: FileInfo }) => source.onContextMenu(event);
      onBackgroundContextMenu = (event: MouseEvent) => source.onBackgroundContextMenu(event);
      onNodeDoubleClick = (entry: FileInfo) => source.onNodeDoubleClick(entry);
      onDragDrop = (event: any) => source.onDragDrop(event);
      onRename = (event: { entry: FileInfo; newName: string }) => source.onRename(event);
      onArchiveMoveConfirmed = () => source.executePendingMove();
      onArchiveMoveCancelled = () => {
        source.archiveMoveWarning = false;
      };
    })();
  }
}