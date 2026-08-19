import type { FileInfo } from '../../shared/protocol';

/**
 * Minimal UI runtime adapter for directory surfaces.
 * This lets directory state/session code ask for "selectable entries" without
 * knowing whether the UI is tree/list/grid/details.
 */
export interface DirSurfaceRuntime {
  getSelectableEntries(): FileInfo[];
  getCursorEntries(): FileInfo[];
  scrollToCursorPath(path: string): void;
}