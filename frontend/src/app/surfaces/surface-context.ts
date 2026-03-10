import type { TabInfo } from '../shared/protocol-enhanced';
import type { ViewerArchiveContext } from '../components/viewer-surface.component';
import type { DirSurfaceVm } from './dir/dir-surface.vm';

export interface SurfaceContext {
  paneId: 'left' | 'right';
  tab: TabInfo;

  // Dir surface VM (valid when tab.kind === 'dir')
  dirVm: DirSurfaceVm;

  // Viewer inputs (valid when tab.kind === 'viewer')
  viewer: {
    targetPath: string;
    archiveContext: ViewerArchiveContext | null;
    viewerId: string | null;
  };
}

