// ─── Shared types for the address-bar feature ────────────────────────────────

export interface PathSegment {
  label: string;
  fullPath: string;
  isRoot: boolean;
  isLast?: boolean;
  rootLabel?: string;
  isArchiveRoot?: boolean;
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
  segmentIndex: number;
  segment: PathSegment | null;
}

export const PINNED_KEY = 'filemanager:pinned-paths-v2';

export const CTX_MENU_CLOSED: ContextMenuState = {
  visible: false, segmentIndex: -1, segment: null,
};
