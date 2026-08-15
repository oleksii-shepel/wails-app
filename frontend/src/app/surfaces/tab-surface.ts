import type { TabInfo } from '../shared/protocol-enhanced';

export const BUILTIN_SURFACE_IDS = {
  dir: 'dir',
  viewer: 'viewer',
  sizeManager: 'size-manager',
  searchResults: 'search-results',
} as const;

export function legacyKindSurfaceId(kind: TabInfo['kind']): string {
  if (kind === 'viewer') return BUILTIN_SURFACE_IDS.viewer;
  if (kind === 'size-manager') return BUILTIN_SURFACE_IDS.sizeManager;
  return BUILTIN_SURFACE_IDS.dir;
}

export function tabSurfaceId(tab: Pick<TabInfo, 'surfaceId' | 'kind'>): string {
  return tab.surfaceId?.trim() || legacyKindSurfaceId(tab.kind);
}

export function tabLocation(tab: Pick<TabInfo, 'location' | 'path'>): string {
  return tab.location?.location?.trim() || tab.path?.trim() || '';
}
export function isDirectoryBackedSurfaceId(surfaceId: string): boolean {
  return surfaceId === BUILTIN_SURFACE_IDS.dir || surfaceId === BUILTIN_SURFACE_IDS.searchResults;
}
