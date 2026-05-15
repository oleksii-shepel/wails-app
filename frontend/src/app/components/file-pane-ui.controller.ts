import type { ViewLayout } from './file-view-container.component';
import { PARENT_ENTRY_PATH } from './file-entry.component';

export interface FilePaneUiHost {
  get paneId(): 'left' | 'right';
  get currentView(): ViewLayout;
  set currentView(value: ViewLayout);
  get sortField(): string;
  set sortField(value: string);
  get sortDirection(): 'asc' | 'desc';
  set sortDirection(value: 'asc' | 'desc');
  get directoriesFirst(): boolean;
  set directoriesFirst(value: boolean);
  get colorizeByMime(): boolean;
  set colorizeByMime(value: boolean);
  get selectedFiles(): Set<string>;
  set selectedFiles(value: Set<string>);
  getOptimisticHiddenPaths(): string[];
  setOptimisticHiddenPaths(paths: string[]): void;
  applyFilter(): void;
  persistDirTabState(): void;
  refreshCurrentPathTransparently(): Promise<void>;
}

export class FilePaneUiController {
  private optimisticHideListener?: (event: Event) => void;
  private optimisticRestoreListener?: (event: Event) => void;

  loadViewPreference(host: FilePaneUiHost): void {
    const savedView = localStorage.getItem('fileManager_viewMode') as ViewLayout | null;
    if (savedView === 'list' || savedView === 'grid' || savedView === 'tree' || savedView === 'details') {
      host.currentView = savedView;
    }

    const savedSortField = localStorage.getItem('fileManager_sortField');
    const savedSortDirection = localStorage.getItem('fileManager_sortDirection') as 'asc' | 'desc' | null;
    if (savedSortField) host.sortField = savedSortField;
    if (savedSortDirection === 'asc' || savedSortDirection === 'desc') host.sortDirection = savedSortDirection;

    const savedDirsFirst = localStorage.getItem('fileManager_directoriesFirst');
    if (savedDirsFirst === 'true' || savedDirsFirst === 'false') {
      host.directoriesFirst = savedDirsFirst === 'true';
    }

    const savedColorizeByMime = localStorage.getItem('fileManager_colorizeByMime');
    if (savedColorizeByMime === 'true' || savedColorizeByMime === 'false') {
      host.colorizeByMime = savedColorizeByMime === 'true';
    }
  }

  async onViewChange(host: FilePaneUiHost, view: ViewLayout): Promise<void> {
    const previousView = host.currentView;
    host.currentView = view;

    if (previousView === 'tree' && view !== 'tree') {
      await host.refreshCurrentPathTransparently();
      return;
    }

    host.applyFilter();
    host.persistDirTabState();
  }

  onSortChange(host: FilePaneUiHost, sort: { field: string; direction: 'asc' | 'desc' }): void {
    host.sortField = sort.field;
    host.sortDirection = sort.direction;
    host.applyFilter();
    host.persistDirTabState();
  }

  onDirectoriesFirstChange(host: FilePaneUiHost, value: boolean): void {
    host.directoriesFirst = value;
    host.applyFilter();
    host.persistDirTabState();
  }

  onColorizeByMimeChange(host: FilePaneUiHost, value: boolean): void {
    host.colorizeByMime = value;
    host.applyFilter();
    host.persistDirTabState();
  }

  bindOptimisticDnD(host: FilePaneUiHost): void {
    this.optimisticHideListener = (event: Event) => {
      const detail = (event as CustomEvent).detail as { paneId?: 'left' | 'right'; paths?: string[] } | undefined;
      if (!detail?.paneId || detail.paneId !== host.paneId) return;

      const paths = (detail.paths ?? []).filter(path => path && path !== PARENT_ENTRY_PATH);
      if (!paths.length) return;

      const hidden = new Set(host.getOptimisticHiddenPaths());
      for (const path of paths) hidden.add(path);
      host.setOptimisticHiddenPaths(Array.from(hidden));

      const nextSelected = new Set(host.selectedFiles);
      for (const path of paths) nextSelected.delete(path);
      host.selectedFiles = nextSelected;
      host.applyFilter();
    };

    this.optimisticRestoreListener = (event: Event) => {
      const detail = (event as CustomEvent).detail as { paneId?: 'left' | 'right'; paths?: string[] } | undefined;
      if (!detail?.paneId || detail.paneId !== host.paneId) return;

      const paths = (detail.paths ?? []).filter(path => path && path !== PARENT_ENTRY_PATH);
      if (!paths.length) return;

      const hidden = new Set(host.getOptimisticHiddenPaths());
      for (const path of paths) hidden.delete(path);
      host.setOptimisticHiddenPaths(Array.from(hidden));
      host.applyFilter();
    };

    window.addEventListener('pane:dnd-optimistic-hide', this.optimisticHideListener);
    window.addEventListener('pane:dnd-optimistic-restore', this.optimisticRestoreListener);
  }

  unbindOptimisticDnD(): void {
    if (this.optimisticHideListener) {
      window.removeEventListener('pane:dnd-optimistic-hide', this.optimisticHideListener);
      this.optimisticHideListener = undefined;
    }
    if (this.optimisticRestoreListener) {
      window.removeEventListener('pane:dnd-optimistic-restore', this.optimisticRestoreListener);
      this.optimisticRestoreListener = undefined;
    }
  }

  /**
   * Promotes the active pane's current view, sort, and grouping options to the
   * global "default for new tabs" slot. Called only from explicit user intent
   * (e.g. a "Set as default" action) — never from per-tab change handlers,
   * which previously caused per-tab tweaks to overwrite the global default.
   */
  promoteCurrentToDefault(host: FilePaneUiHost): void {
    localStorage.setItem('fileManager_viewMode', host.currentView);
    localStorage.setItem('fileManager_sortField', host.sortField);
    localStorage.setItem('fileManager_sortDirection', host.sortDirection);
    localStorage.setItem('fileManager_directoriesFirst', String(host.directoriesFirst));
    localStorage.setItem('fileManager_colorizeByMime', String(host.colorizeByMime));
  }
}