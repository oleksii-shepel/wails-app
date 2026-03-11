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

export type SurfaceContext = DirSurfaceContext | ViewerSurfaceContext;

