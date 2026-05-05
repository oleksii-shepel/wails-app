import { FilterQuery } from '../../shared/protocol-enhanced';
import { ViewLayout } from '../../components/file-view-container.component';
import { FileInfo } from '../../shared/protocol';

export interface DirTabState {
  /** Directory path for which this state was captured (FS paths only for now). */
  path: string;
  /** Whether hidden files were shown when entries were loaded. */
  showHidden: boolean;
  currentView: ViewLayout;
  sortField: string;
  sortDirection: 'asc' | 'desc';
  directoriesFirst: boolean;
  colorizeByMime: boolean;
  filterQuery: FilterQuery;
  selectedPaths: string[];
  cursorPath: string | null;
  anchorPath: string | null;
  /** Last vertical scroll offset of the file list, in CSS pixels. */
  scrollTop: number;
  expandedPaths: string[];

  /** Cached listing for instant restore when returning to the tab. */
  entries: FileInfo[];
  optimisticHiddenPaths: string[];
}
