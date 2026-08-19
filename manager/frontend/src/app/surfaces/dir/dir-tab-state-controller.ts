import { DirTabState } from './dir-tab-state';
import { FilterQuery } from '../../shared/protocol-enhanced';
import { FileInfo } from '../../shared/protocol';

export interface PendingRestoreSlot {
  selectedPaths: string[];
  cursorPath: string | null;
  anchorPath: string | null;
  scrollTop: number | null;
  expandedPaths: string[];
}

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
  pendingRestore: PendingRestoreSlot | null;
  userInteractedSinceLoad: boolean;
}

export interface DirTabUiRuntime {
  isInArchive(): boolean;
  getSelectableEntriesPaths(): string[];
  applyFilter(): void;
  scrollToCursorPath(path: string): void;
  emitPreviewTargetFromState(): void;
  getExpandedPaths?(): string[];
  setExpandedPaths?(paths: string[]): void;
  /** Current scroll offset of the file list, or 0 if not measurable. */
  getScrollTop?(): number;
  /** Restore the file list scroll offset (best-effort). */
  setScrollTop?(top: number): void;
}

export interface DirTabRestoreTarget {
  session: DirTabSessionState;
  ui: DirTabUiRuntime;
}

/** Slice of DirTabState that is JSON-serialized for cross-restart persistence. */
export interface PersistedDirTabState {
  /**
   * Bumped whenever the persisted shape changes incompatibly. Older blobs run
   * through DIR_TAB_STATE_MIGRATIONS to upgrade to the current version on load.
   * Missing field is treated as version 0 (the original unversioned shape).
   */
  schemaVersion: number;
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
  scrollTop: number;
  expandedPaths?: string[];
  entries?: FileInfo[];
  optimisticHiddenPaths: string[];
}

export const CURRENT_DIR_TAB_STATE_VERSION = 1;

/**
 * Migrations from version N to N+1. Each entry takes the raw (parsed-JSON)
 * blob produced under version N and returns a blob shaped like version N+1.
 * Add a new entry here whenever the persisted shape changes incompatibly.
 */
export const DIR_TAB_STATE_MIGRATIONS: Record<number, (raw: any) => any> = {
  // 0 → 1: original shape — no field reshapes; just stamp the version.
  0: (raw) => ({ ...raw, schemaVersion: 1 }),
};

function migrateDirTabState(raw: any): Partial<PersistedDirTabState> | null {
  if (!raw || typeof raw !== 'object') return null;
  let working: any = raw;
  let version = typeof working.schemaVersion === 'number' ? working.schemaVersion : 0;
  while (version < CURRENT_DIR_TAB_STATE_VERSION) {
    const migrate = DIR_TAB_STATE_MIGRATIONS[version];
    if (!migrate) {
      // No migration registered — refuse rather than silently mis-interpret.
      return null;
    }
    working = migrate(working);
    version += 1;
  }
  return working as Partial<PersistedDirTabState>;
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
      scrollTop: Math.max(0, Math.round(target.ui.getScrollTop?.() ?? 0)),
      expandedPaths: Array.isArray(target.ui.getExpandedPaths?.()) ? target.ui.getExpandedPaths?.() ?? [] : [],
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
      const raw = JSON.parse(json) as Record<string, unknown>;
      const source = raw && typeof raw === 'object' && 'dir' in raw
        ? raw['dir']
        : raw;
      parsed = migrateDirTabState(source);
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
      scrollTop: typeof parsed.scrollTop === 'number' && parsed.scrollTop >= 0 ? parsed.scrollTop : 0,
      expandedPaths: Array.isArray(parsed.expandedPaths) ? parsed.expandedPaths.filter((path): path is string => typeof path === 'string') : [],
      entries: Array.isArray(parsed.entries) ? parsed.entries.map(entry => ({ ...entry })) : [],
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
      schemaVersion: CURRENT_DIR_TAB_STATE_VERSION,
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
      scrollTop: state.scrollTop,
      expandedPaths: [...state.expandedPaths],
      entries: state.entries.map(entry => ({ ...entry })),
      optimisticHiddenPaths: state.optimisticHiddenPaths,
    };
  }

  applyBeforeLoad(state: DirTabState, session: DirTabSessionState): void {
    session.currentPath = state.path;
    session.currentView = state.currentView;
    session.sortField = state.sortField;
    session.sortDirection = state.sortDirection;
    session.directoriesFirst = state.directoriesFirst;
    session.colorizeByMime = state.colorizeByMime ?? false;
    session.filterQuery = state.filterQuery ?? { text: '', criteria: [] };
    session.entries = Array.isArray(state.entries) ? state.entries.map(entry => ({ ...entry })) : [];
  }

  finalizeAfterLoad(state: DirTabState, target: DirTabRestoreTarget): void {
    target.session.setOptimisticHiddenPaths(state.optimisticHiddenPaths ?? []);

    target.session.pendingRestore = {
      selectedPaths: [...(state.selectedPaths ?? [])],
      cursorPath: state.cursorPath ?? null,
      anchorPath: state.anchorPath ?? null,
      scrollTop: typeof state.scrollTop === 'number' ? state.scrollTop : null,
      expandedPaths: [...(state.expandedPaths ?? [])],
    };
    target.session.userInteractedSinceLoad = false;

    target.ui.applyFilter();
    this.applyPendingRestore(target);
  }

  /**
   * Re-attempt restoration of selection, cursor, anchor, scroll position, and
   * tree expansion against whatever entries are currently available. Safe to
   * call multiple times — each pass resolves whichever pending paths are now
   * present and leaves the rest pending for the next call. Bails out if the
   * user has touched the UI since the last load.
   */
  applyPendingRestore(target: DirTabRestoreTarget): void {
    const pending = target.session.pendingRestore;
    if (!pending || target.session.userInteractedSinceLoad) {
      return;
    }

    const present = new Set(target.session.getEntriesPaths());

    const resolvedSelected = pending.selectedPaths.filter(p => present.has(p));
    const stillPendingSelected = pending.selectedPaths.filter(p => !present.has(p));
    // Only seed selection while the user hasn't manually selected anything.
    // Once the user clicks a row, selectedFiles is non-empty and we leave it.
    if (target.session.selectedFiles.size === 0 && resolvedSelected.length > 0) {
      target.session.selectedFiles = new Set(resolvedSelected);
    }

    if (!target.session.selectionCursorPath) {
      if (pending.cursorPath && present.has(pending.cursorPath)) {
        target.session.selectionCursorPath = pending.cursorPath;
      } else if (resolvedSelected.length > 0) {
        target.session.selectionCursorPath = resolvedSelected[resolvedSelected.length - 1];
      } else {
        const selectable = target.ui.getSelectableEntriesPaths();
        if (selectable.length) target.session.selectionCursorPath = selectable[0];
      }
    }

    if (!target.session.selectionAnchorPath) {
      if (pending.anchorPath && present.has(pending.anchorPath)) {
        target.session.selectionAnchorPath = pending.anchorPath;
      } else {
        target.session.selectionAnchorPath = target.session.selectionCursorPath;
      }
    }

    target.ui.setExpandedPaths?.(pending.expandedPaths);

    if (pending.scrollTop !== null && target.ui.setScrollTop) {
      this.deferredApplyScrollTop(target.ui, pending.scrollTop);
    } else if (pending.scrollTop === null && target.session.selectionCursorPath) {
      target.ui.scrollToCursorPath(target.session.selectionCursorPath);
    }

    target.ui.emitPreviewTargetFromState();

    const cursorResolved = !pending.cursorPath || present.has(pending.cursorPath);
    const anchorResolved = !pending.anchorPath || present.has(pending.anchorPath);

    if (stillPendingSelected.length === 0 && cursorResolved && anchorResolved) {
      target.session.pendingRestore = null;
    } else {
      target.session.pendingRestore = {
        selectedPaths: stillPendingSelected,
        cursorPath: cursorResolved ? null : pending.cursorPath,
        anchorPath: anchorResolved ? null : pending.anchorPath,
        // Scroll only applies once; clear after first attempt so future entry
        // changes don't yank the user back to the persisted offset.
        scrollTop: null,
        // Same for expanded paths — set once.
        expandedPaths: [],
      };
    }
  }

  /**
   * setScrollTop runs before Angular has reconciled the new entries, so the
   * scrollable container's content height is often still stale. Try once
   * synchronously, then again on the next frame, then once more after layout
   * has likely settled — drop the attempt if the user has scrolled in between.
   */
  private deferredApplyScrollTop(ui: DirTabUiRuntime, target: number): void {
    if (!ui.setScrollTop) return;
    ui.setScrollTop(target);
    requestAnimationFrame(() => {
      if (!ui.setScrollTop) return;
      const current = ui.getScrollTop?.() ?? 0;
      if (Math.abs(current - target) > 4) {
        ui.setScrollTop(target);
      }
    });
  }
}