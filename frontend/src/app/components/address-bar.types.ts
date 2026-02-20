// ─── Shared types for the address-bar feature ────────────────────────────────

export interface PathSegment {
  label: string;
  fullPath: string;
  isRoot: boolean;
  rootLabel?: string;
  /** True for the archive file entry itself (e.g. project.zip) */
  isArchiveRoot?: boolean;
  /** True for path parts that live *inside* an archive — not navigable via FS */
  isArchiveInner?: boolean;
}

export interface SiblingEntry {
  name: string;
  path: string;
  isCurrent: boolean;
  isArchive?: boolean;
}

export interface PinnedPath {
  path: string;
  label: string;
  icon: string;
  addedAt: number;
}

export interface ContextMenuState {
  visible: boolean;
  x: number;
  y: number;
  segmentIndex: number;
  segment: PathSegment | null;
}

export const PINNED_KEY = 'filemanager:pinned-paths-v2';

export const CTX_MENU_CLOSED: ContextMenuState = {
  visible: false, x: 0, y: 0, segmentIndex: -1, segment: null,
};
