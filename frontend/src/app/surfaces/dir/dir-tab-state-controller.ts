import { DirTabState } from './dir-tab-state';
import { FilterQuery } from '../../shared/protocol-enhanced';
import { FileInfo } from '../../shared/protocol';

export interface DirTabRuntime {
  isInArchive(): boolean;
  getEntriesPaths(): string[];
  getSelectableEntriesPaths(): string[];
  applyFilter(): void;
  scrollToCursorPath(path: string): void;
  emitPreviewTargetFromState(): void;
  getOptimisticHiddenPaths(): string[];
  setOptimisticHiddenPaths(paths: string[]): void;

  // State fields we snapshot/restore.
  currentPath: string;
  showHidden: boolean;
  entries: FileInfo[];
  currentView: DirTabState['currentView'];
  sortField: string;
  sortDirection: 'asc' | 'desc';
  directoriesFirst: boolean;
  filterQuery: FilterQuery;
  selectedFiles: Set<string>;
  selectionCursorPath: string | null;
  selectionAnchorPath: string | null;
}

export class DirTabStateController {
  private readonly stateByTabId = new Map<string, DirTabState>();

  save(tabId: string, runtime: DirTabRuntime): void {
    if (!tabId) return;
    if (runtime.isInArchive()) return;
    const state: DirTabState = {
      path: runtime.currentPath || '',
      showHidden: !!runtime.showHidden,
      currentView: runtime.currentView,
      sortField: runtime.sortField,
      sortDirection: runtime.sortDirection,
      directoriesFirst: runtime.directoriesFirst,
      filterQuery: runtime.filterQuery ?? { text: '', criteria: [] },
      selectedPaths: Array.from(runtime.selectedFiles ?? new Set<string>()),
      cursorPath: runtime.selectionCursorPath,
      anchorPath: runtime.selectionAnchorPath,
      entries: Array.isArray(runtime.entries) ? [...runtime.entries] : [],
      optimisticHiddenPaths: runtime.getOptimisticHiddenPaths(),
    };
    this.stateByTabId.set(tabId, state);
  }

  load(tabId: string): DirTabState | null {
    if (!tabId) return null;
    return this.stateByTabId.get(tabId) ?? null;
  }

  delete(tabId: string): void {
    if (!tabId) return;
    this.stateByTabId.delete(tabId);
  }

  applyBeforeLoad(state: DirTabState, runtime: DirTabRuntime): void {
    runtime.currentView = state.currentView;
    runtime.sortField = state.sortField;
    runtime.sortDirection = state.sortDirection;
    runtime.directoriesFirst = state.directoriesFirst;
    runtime.filterQuery = state.filterQuery ?? { text: '', criteria: [] };
  }

  finalizeAfterLoad(state: DirTabState, runtime: DirTabRuntime): void {
    if (runtime.isInArchive()) return;

    runtime.setOptimisticHiddenPaths(state.optimisticHiddenPaths ?? []);

    const present = new Set(runtime.getEntriesPaths());
    const restoredSelected = (state.selectedPaths ?? []).filter(p => present.has(p));
    runtime.selectedFiles = new Set(restoredSelected);

    if (state.cursorPath && present.has(state.cursorPath)) {
      runtime.selectionCursorPath = state.cursorPath;
    } else if (restoredSelected.length > 0) {
      runtime.selectionCursorPath = restoredSelected[restoredSelected.length - 1];
    } else {
      const selectable = runtime.getSelectableEntriesPaths();
      runtime.selectionCursorPath = selectable.length ? selectable[0] : null;
    }

    runtime.selectionAnchorPath =
      state.anchorPath && present.has(state.anchorPath)
        ? state.anchorPath
        : runtime.selectionCursorPath;

    runtime.applyFilter();
    if (runtime.selectionCursorPath) runtime.scrollToCursorPath(runtime.selectionCursorPath);
    runtime.emitPreviewTargetFromState();
  }
}
