import type { FileInfo } from '../../shared/protocol';
import type { ViewLayout } from '../../components/file-view-container.component';
import type { FindWidgetState } from '../../components/filter-widget.component';

export interface DirSurfaceVm {
  paneId: 'left' | 'right';
  pathReady: boolean;
  currentPath: string;
  loading: boolean;
  showPathBar: boolean;
  showBreadcrumbs: boolean;
  singleClickToOpen: boolean;
  showHidden: boolean;
  paneActive: boolean;

  entries: FileInfo[];
  selectedFiles: Set<string>;
  cursorPath: string | null;
  error: string | null;

  currentView: ViewLayout;
  sortField: string;
  sortDirection: 'asc' | 'desc';
  directoriesFirst: boolean;
  colorizeByMime: boolean;
  browserPaneSplit: number | null;

  archiveMoveWarning: boolean;
  archiveMoveEntryCount: number;

  onPathChange(path: string): void;
  onArchiveSegmentClick(path: string): void;
  onNavigateUp(): void;
  onRefresh(): void;

  onViewChange(view: ViewLayout): void;
  onSortChange(sort: { field: string; direction: 'asc' | 'desc' }): void;
  onDirectoriesFirstChange(v: boolean): void;
  onColorizeByMimeChange(v: boolean): void;
  onBrowserPaneSplitChange(v: number): void;

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
  onContextMenu(ev: { event: MouseEvent; entry: FileInfo }): void;
  onBackgroundContextMenu(event: MouseEvent): void;
  onNodeDoubleClick(ev: any): void;
  onDragDrop(ev: any): void;
  onRename(ev: { entry: FileInfo; newName: string }): void;

  onArchiveMoveConfirmed(): void;
  onArchiveMoveCancelled(): void;
}
