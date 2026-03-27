import { DirTabState } from './dir-tab-state';
import { FilterQuery } from '../../shared/protocol-enhanced';
import { FileInfo } from '../../shared/protocol';

export interface DirTabSessionState {
  getEntriesPaths(): string[];
  getOptimisticHiddenPaths(): string[];
  setOptimisticHiddenPaths(paths: string[]): void;

  currentPath: string;
  showHidden: boolean;
  entries: FileInfo[];
  currentView: DirTabState['currentView'];
  sortField: string;
  sortDirection: 'asc' | 'desc';
  directoriesFirst: boolean;
  colorizeByMime: boolean;
  filterQuery: FilterQuery;
  selectedFiles: Set<string>;
  selectionCursorPath: string | null;
  selectionAnchorPath: string | null;
}

export interface DirTabUiRuntime {
  isInArchive(): boolean;
  getSelectableEntriesPaths(): string[];
  applyFilter(): void;
  scrollToCursorPath(path: string): void;
  emitPreviewTargetFromState(): void;
}

export interface DirTabRestoreTarget {
  session: DirTabSessionState;
  ui: DirTabUiRuntime;
}

export class DirTabStateController {
  private readonly stateByTabId = new Map<string, DirTabState>();

  save(tabId: string, target: DirTabRestoreTarget): void {
    if (!tabId) return;
    if (target.ui.isInArchive()) return;
    const state: DirTabState = {
      path: target.session.currentPath || '',
      showHidden: !!target.session.showHidden,
      currentView: target.session.currentView,
      sortField: target.session.sortField,
      sortDirection: target.session.sortDirection,
      directoriesFirst: target.session.directoriesFirst,
      colorizeByMime: target.session.colorizeByMime,
      filterQuery: target.session.filterQuery ?? { text: '', criteria: [] },
      selectedPaths: Array.from(target.session.selectedFiles ?? new Set<string>()),
      cursorPath: target.session.selectionCursorPath,
      anchorPath: target.session.selectionAnchorPath,
      entries: Array.isArray(target.session.entries) ? [...target.session.entries] : [],
      optimisticHiddenPaths: target.session.getOptimisticHiddenPaths(),
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

  applyBeforeLoad(state: DirTabState, session: DirTabSessionState): void {
    session.currentView = state.currentView;
    session.sortField = state.sortField;
    session.sortDirection = state.sortDirection;
    session.directoriesFirst = state.directoriesFirst;
    session.colorizeByMime = state.colorizeByMime ?? false;
    session.filterQuery = state.filterQuery ?? { text: '', criteria: [] };
  }

  finalizeAfterLoad(state: DirTabState, target: DirTabRestoreTarget): void {
    if (target.ui.isInArchive()) return;

    target.session.setOptimisticHiddenPaths(state.optimisticHiddenPaths ?? []);

    const present = new Set(target.session.getEntriesPaths());
    const restoredSelected = (state.selectedPaths ?? []).filter(p => present.has(p));
    target.session.selectedFiles = new Set(restoredSelected);

    if (state.cursorPath && present.has(state.cursorPath)) {
      target.session.selectionCursorPath = state.cursorPath;
    } else if (restoredSelected.length > 0) {
      target.session.selectionCursorPath = restoredSelected[restoredSelected.length - 1];
    } else {
      const selectable = target.ui.getSelectableEntriesPaths();
      target.session.selectionCursorPath = selectable.length ? selectable[0] : null;
    }

    target.session.selectionAnchorPath =
      state.anchorPath && present.has(state.anchorPath)
        ? state.anchorPath
        : target.session.selectionCursorPath;

    target.ui.applyFilter();
    if (target.session.selectionCursorPath) target.ui.scrollToCursorPath(target.session.selectionCursorPath);
    target.ui.emitPreviewTargetFromState();
  }
}
