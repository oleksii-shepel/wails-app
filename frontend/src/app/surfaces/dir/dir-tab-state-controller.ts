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

/** Slice of DirTabState that is JSON-serialized for cross-restart persistence. */
export interface PersistedDirTabState {
  path: string;
  showHidden: boolean;
  currentView: DirTabState['currentView'];
  sortField: string;
  sortDirection: 'asc' | 'desc';
  directoriesFirst: boolean;
  colorizeByMime: boolean;
  filterQuery: DirTabState['filterQuery'];
  selectedPaths: string[];
  cursorPath: string | null;
  anchorPath: string | null;
  optimisticHiddenPaths: string[];
}

export type DirTabStatePersistCallback = (tabId: string, json: string) => void;

export class DirTabStateController {
  private readonly stateByTabId = new Map<string, DirTabState>();
  private persistCallback: DirTabStatePersistCallback | null = null;

  /** Register a callback invoked whenever a tab's persistable state changes. */
  setPersistCallback(callback: DirTabStatePersistCallback | null): void {
    this.persistCallback = callback;
  }

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

    if (this.persistCallback) {
      try {
        this.persistCallback(tabId, JSON.stringify(this.toPersisted(state)));
      } catch {
        // Persist callbacks must never throw out of save().
      }
    }
  }

  load(tabId: string): DirTabState | null {
    if (!tabId) return null;
    return this.stateByTabId.get(tabId) ?? null;
  }

  /** Seed the in-memory map from a persisted JSON blob (one-shot, on workspace load). */
  hydrateFromJson(tabId: string, json: string | null | undefined): void {
    if (!tabId || !json) return;
    if (this.stateByTabId.has(tabId)) return;
    let parsed: Partial<PersistedDirTabState> | null = null;
    try {
      parsed = JSON.parse(json) as Partial<PersistedDirTabState>;
    } catch {
      return;
    }
    if (!parsed || typeof parsed !== 'object') return;

    this.stateByTabId.set(tabId, {
      path: parsed.path ?? '',
      showHidden: !!parsed.showHidden,
      currentView: parsed.currentView ?? 'list',
      sortField: parsed.sortField ?? 'name',
      sortDirection: parsed.sortDirection === 'desc' ? 'desc' : 'asc',
      directoriesFirst: !!parsed.directoriesFirst,
      colorizeByMime: !!parsed.colorizeByMime,
      filterQuery: parsed.filterQuery ?? { text: '', criteria: [] },
      selectedPaths: Array.isArray(parsed.selectedPaths) ? parsed.selectedPaths : [],
      cursorPath: parsed.cursorPath ?? null,
      anchorPath: parsed.anchorPath ?? null,
      entries: [],
      optimisticHiddenPaths: Array.isArray(parsed.optimisticHiddenPaths) ? parsed.optimisticHiddenPaths : [],
    });
  }

  delete(tabId: string): void {
    if (!tabId) return;
    this.stateByTabId.delete(tabId);
    this.persistCallback?.(tabId, '');
  }

  private toPersisted(state: DirTabState): PersistedDirTabState {
    return {
      path: state.path,
      showHidden: state.showHidden,
      currentView: state.currentView,
      sortField: state.sortField,
      sortDirection: state.sortDirection,
      directoriesFirst: state.directoriesFirst,
      colorizeByMime: state.colorizeByMime,
      filterQuery: state.filterQuery,
      selectedPaths: state.selectedPaths,
      cursorPath: state.cursorPath,
      anchorPath: state.anchorPath,
      optimisticHiddenPaths: state.optimisticHiddenPaths,
    };
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
