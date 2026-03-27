import type { FileInfo } from '../../shared/protocol';
import type { FilterQuery } from '../../shared/protocol-enhanced';
import type { ViewLayout } from '../../components/file-view-container.component';

export class DirSurfaceSession {
  constructor(public readonly tabId: string) {}

  currentPath = '';
  fsParentDir = '';
  showHiddenLoaded: boolean | null = null;

  entries: FileInfo[] = [];
  filteredEntries: FileInfo[] = [];

  selectedFiles: Set<string> = new Set<string>();
  selectionCursorPath: string | null = null;
  selectionAnchorPath: string | null = null;

  loading = false;
  error: string | null = null;

  filterQuery: FilterQuery = { text: '', criteria: [] };
  optimisticHiddenPaths = new Set<string>();

  currentView: ViewLayout = 'list';
  sortField: string = 'name';
  sortDirection: 'asc' | 'desc' = 'asc';
  directoriesFirst = true;
  colorizeByMime = false;

  lastPreviewTargetEmitted: string | null = null;

  getEntriesPaths(): string[] {
    return (this.entries ?? []).map((entry) => entry.path);
  }

  getOptimisticHiddenPaths(): string[] {
    return Array.from(this.optimisticHiddenPaths ?? []);
  }

  setOptimisticHiddenPaths(paths: string[]): void {
    this.optimisticHiddenPaths = new Set((paths ?? []).filter(Boolean));
  }
}
