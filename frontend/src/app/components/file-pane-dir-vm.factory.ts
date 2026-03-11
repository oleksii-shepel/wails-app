import type { FindWidgetState } from './filter-widget.component';
import type { ViewLayout } from './file-view-container.component';
import type { FileInfo } from '../shared/protocol';
import type { DirSurfaceVm } from '../surfaces/dir/dir-surface.vm';

export interface FilePaneDirVmSource {
  get paneId(): 'left' | 'right';
  get pathReady(): boolean;
  get currentPath(): string;
  get showHidden(): boolean;
  get paneActive(): boolean;
  get filteredEntries(): FileInfo[];
  get selectedFiles(): Set<string>;
  get selectionCursorPath(): string | null;
  get error(): string | null;
  get currentView(): ViewLayout;
  get sortField(): string;
  get sortDirection(): 'asc' | 'desc';
  get directoriesFirst(): boolean;
  get archiveMoveWarning(): boolean;
  get archivePendingMoveEntries(): string[];
  set archiveMoveWarning(value: boolean);
  onPathChange(path: string): void;
  onArchiveSegmentClick(path: string): void;
  onNavigateUp(): void;
  onRefresh(): void;
  onViewChange(view: ViewLayout): void;
  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void;
  onDirectoriesFirstChange(value: boolean): void;
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
      get showHidden() { return source.showHidden; }
      get paneActive() { return source.paneActive; }
      get entries() { return source.filteredEntries; }
      get selectedFiles() { return source.selectedFiles; }
      get cursorPath() { return source.selectionCursorPath; }
      get error() { return source.error; }
      get currentView() { return source.currentView; }
      get sortField() { return source.sortField; }
      get sortDirection() { return source.sortDirection; }
      get directoriesFirst() { return source.directoriesFirst; }
      get archiveMoveWarning() { return source.archiveMoveWarning; }
      get archiveMoveEntryCount() { return source.archivePendingMoveEntries.length; }

      onPathChange = (path: string) => source.onPathChange(path);
  onArchiveSegmentClick = (path: string) => source.onArchiveSegmentClick(path);
      onNavigateUp = () => source.onNavigateUp();
      onRefresh = () => source.onRefresh();
      onViewChange = (view: ViewLayout) => source.onViewChange(view);
      onSortChange = (sort: { field: string; direction: 'asc' | 'desc' }) => source.onSortChange(sort);
      onDirectoriesFirstChange = (value: boolean) => source.onDirectoriesFirstChange(value);
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