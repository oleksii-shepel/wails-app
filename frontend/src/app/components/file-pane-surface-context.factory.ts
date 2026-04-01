import type { ViewerArchiveContext } from '../viewers/viewer-archive-context';
import type { DirSurfaceVm } from '../surfaces/dir/dir-surface.vm';
import type { DirSurfaceContext, SurfaceContext, ViewerSurfaceContext, SizeManagerSurfaceContext } from '../surfaces/surface-context';
import type { SizeManagerSurfaceVm } from '../surfaces/size-manager/size-manager-surface.vm';
import type { TabInfo } from '../shared/protocol-enhanced';

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
    if ((tab.kind ?? 'dir') === 'size-manager' && source.sizeManagerVm) {
      return {
        paneId: source.paneId,
        kind: 'size-manager',
        tab,
        sizeVm: source.sizeManagerVm,
      } satisfies SizeManagerSurfaceContext;
    }

    if ((tab.kind ?? 'dir') === 'viewer') {
      return {
        paneId: source.paneId,
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

    return {
      paneId: source.paneId,
      kind: 'dir',
      tab,
      dirVm: source.dirVm,
    } satisfies DirSurfaceContext;
  }

  getActiveTabKind(source: FilePaneSurfaceContextSource): 'dir' | 'viewer' {
    return (this.getActiveTab(source)?.kind ?? 'dir') === 'viewer' ? 'viewer' : 'dir';
  }

  getActiveViewerTargetPath(source: FilePaneSurfaceContextSource): string {
    const tab = this.getActiveTab(source);
    if (!tab || (tab.kind ?? 'dir') !== 'viewer') {
      return '';
    }

    return tab.viewer?.targetPath ?? '';
  }

  getActiveViewerId(source: FilePaneSurfaceContextSource): string | null {
    const tab = this.getActiveTab(source);
    if (!tab || (tab.kind ?? 'dir') !== 'viewer') {
      return null;
    }

    return tab.viewer?.viewerId ?? null;
  }

  getActiveViewerFileList(source: FilePaneSurfaceContextSource) {
    const tab = this.getActiveTab(source);
    if (!tab || (tab.kind ?? 'dir') !== 'viewer') {
      return [];
    }

    return tab.viewer?.fileList ?? [];
  }

  getActiveViewerArchiveContext(source: FilePaneSurfaceContextSource): ViewerArchiveContext | null {
    const tab = this.getActiveTab(source);
    if (!tab || (tab.kind ?? 'dir') !== 'viewer') {
      return null;
    }

    return (tab.viewer?.archiveContext as ViewerArchiveContext) ?? null;
  }

  createSurfaceContext(source: FilePaneSurfaceContextSource): SurfaceContext | null {
    const tab = this.getActiveTab(source);
    if (!tab) {
      return null;
    }

    return this.getSurfaceContextForTab(source, tab);
  }
}
