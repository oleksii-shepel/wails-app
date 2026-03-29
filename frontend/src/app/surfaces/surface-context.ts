import type { TabInfo } from '../shared/protocol-enhanced';
import type { ViewerFileListItem } from '../shared/protocol-enhanced';
import type { ViewerArchiveContext } from '../viewers/viewer-archive-context';
import type { DirSurfaceVm } from './dir/dir-surface.vm';
import type { SizeManagerSurfaceVm } from './size-manager/size-manager-surface.vm';

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
    fileList: ViewerFileListItem[];
    archiveContext: ViewerArchiveContext | null;
    viewerId: string | null;
  };
}

export interface SizeManagerSurfaceContext extends BaseSurfaceContext {
  kind: 'size-manager';
  sizeVm: SizeManagerSurfaceVm;
}

export type SurfaceContext = DirSurfaceContext | ViewerSurfaceContext | SizeManagerSurfaceContext;

