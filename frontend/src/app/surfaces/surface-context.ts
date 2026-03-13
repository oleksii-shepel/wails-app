import type { TabInfo } from '../shared/protocol-enhanced';
import type { ViewerArchiveContext } from '../components/viewer-surface.component';
import type { DirSurfaceVm } from './dir/dir-surface.vm';

interface BaseSurfaceContext {
  paneId: 'left' | 'right';
  tab: TabInfo;
}

export interface DirSurfaceContext extends BaseSurfaceContext {
  kind: 'dir';
  dirVm: DirSurfaceVm;
}

export interface ViewerSurfaceContext extends BaseSurfaceContext {
  kind: 'viewer';
  viewer: {
    targetPath: string;
    archiveContext: ViewerArchiveContext | null;
    viewerId: string | null;
  };
}

export interface SizeManagerSurfaceContext extends BaseSurfaceContext {
  kind: 'size-manager';
  sourcePath: string;
  sourcePaneId: 'left' | 'right';
}

export type SurfaceContext = DirSurfaceContext | ViewerSurfaceContext | SizeManagerSurfaceContext;

