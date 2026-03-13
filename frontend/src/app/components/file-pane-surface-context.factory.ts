import type { ViewerArchiveContext } from './viewer-surface.component';
import type { DirSurfaceVm } from '../surfaces/dir/dir-surface.vm';
import type { DirSurfaceContext, SurfaceContext, ViewerSurfaceContext } from '../surfaces/surface-context';
import type { TabInfo } from '../shared/protocol-enhanced';

export interface FilePaneSurfaceContextSource {
  get paneId(): 'left' | 'right';
  get currentTabId(): string;
  get tabs(): TabInfo[];
  dirVm: DirSurfaceVm;
}

export class FilePaneSurfaceContextFactory {
  getActiveTab(source: FilePaneSurfaceContextSource): TabInfo | undefined {
    return source.tabs.find((tab) => tab.id === source.currentTabId);
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

    if ((tab.kind ?? 'dir') === 'viewer') {
      return {
        paneId: source.paneId,
        kind: 'viewer',
        tab,
        viewer: {
          targetPath: this.getActiveViewerTargetPath(source),
          archiveContext: this.getActiveViewerArchiveContext(source),
          viewerId: this.getActiveViewerId(source),
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
}