import type { ViewerArchiveContext } from '../viewers/viewer-archive-context';
import type { DirSurfaceVm } from '../surfaces/dir/dir-surface.vm';
import type {
  DirSurfaceContext,
  SearchResultsSurfaceContext,
  SizeManagerSurfaceContext,
  SurfaceContext,
  ViewerSurfaceContext,
} from '../surfaces/surface-context';
import type { SizeManagerSurfaceVm } from '../surfaces/size-manager/size-manager-surface.vm';
import type { TabInfo } from '../shared/protocol-enhanced';
import { tabSurfaceId } from '../surfaces/tab-surface';

export interface FilePaneSurfaceContextSource {
  get paneId(): 'left' | 'right';
  get currentTabId(): string;
  get tabs(): TabInfo[];
  dirVm: DirSurfaceVm;
  sizeManagerVm: SizeManagerSurfaceVm | null;
}

export class FilePaneSurfaceContextFactory {
  getActiveTab(source: FilePaneSurfaceContextSource): TabInfo | undefined {
    return source.tabs.find((tab) => tab.id === source.currentTabId);
  }

  getSurfaceContextForTab(source: FilePaneSurfaceContextSource, tab: TabInfo): SurfaceContext {
    const surfaceId = tabSurfaceId(tab);

    if (surfaceId === 'size-manager' && source.sizeManagerVm) {
      return {
        paneId: source.paneId,
        surfaceId: 'size-manager',
        kind: 'size-manager',
        tab,
        sizeVm: source.sizeManagerVm,
      } satisfies SizeManagerSurfaceContext;
    }

    if (surfaceId === 'viewer') {
      return {
        paneId: source.paneId,
        surfaceId: 'viewer',
        kind: 'viewer',
        tab,
        viewer: {
          targetPath: tab.viewer?.targetPath ?? '',
          fileList: tab.viewer?.fileList ?? [],
          archiveContext: (tab.viewer?.archiveContext as ViewerArchiveContext) ?? null,
          viewerId: tab.viewer?.viewerId ?? null,
        },
      } satisfies ViewerSurfaceContext;
    }

    if (surfaceId === 'search-results') {
      return {
        paneId: source.paneId,
        surfaceId: 'search-results',
        kind: 'search-results',
        tab,
        dirVm: source.dirVm,
      } satisfies SearchResultsSurfaceContext;
    }

    if (surfaceId === 'dir') {
      return {
        paneId: source.paneId,
        surfaceId: 'dir',
        kind: 'dir',
        tab,
        dirVm: source.dirVm,
      } satisfies DirSurfaceContext;
    }

    // Foreign/tool surfaces receive the stable common context. Their Angular
    // component owns any additional runtime state and may inject its services.
    return {
      paneId: source.paneId,
      surfaceId,
      tab,
    };
  }

  getActiveViewerTargetPath(source: FilePaneSurfaceContextSource): string {
    const tab = this.getActiveTab(source);
    if (!tab || tabSurfaceId(tab) !== 'viewer') return '';
    return tab.viewer?.targetPath ?? '';
  }

  getActiveViewerId(source: FilePaneSurfaceContextSource): string | null {
    const tab = this.getActiveTab(source);
    if (!tab || tabSurfaceId(tab) !== 'viewer') return null;
    return tab.viewer?.viewerId ?? null;
  }

  getActiveViewerFileList(source: FilePaneSurfaceContextSource) {
    const tab = this.getActiveTab(source);
    if (!tab || tabSurfaceId(tab) !== 'viewer') return [];
    return tab.viewer?.fileList ?? [];
  }

  getActiveViewerArchiveContext(source: FilePaneSurfaceContextSource): ViewerArchiveContext | null {
    const tab = this.getActiveTab(source);
    if (!tab || tabSurfaceId(tab) !== 'viewer') return null;
    return (tab.viewer?.archiveContext as ViewerArchiveContext) ?? null;
  }

  createSurfaceContext(source: FilePaneSurfaceContextSource): SurfaceContext | null {
    const tab = this.getActiveTab(source);
    return tab ? this.getSurfaceContextForTab(source, tab) : null;
  }
}