import type { ArchiveFrame } from '../services/archive.service';

export interface FilePaneRuntimeStateHost {
  get archiveStack(): ArchiveFrame[];
  set archiveStack(value: ArchiveFrame[]);
  get fsParentDir(): string;
  set fsParentDir(value: string);
  get activeDirTabId(): string | null;
  get currentTabId(): string;
  get activeSurfaceId(): string;
  isInArchive(): boolean;
  restoreActiveDirTabState(): void;
  saveDirTabState(tabId: string): void;
}

export class FilePaneRuntimeStateController {
  afterNavigationMutation(host: FilePaneRuntimeStateHost): void {
    host.restoreActiveDirTabState();
  }

  clearArchiveState(host: FilePaneRuntimeStateHost): void {
    host.archiveStack = [];
    host.fsParentDir = '';
  }

  persistDirTabState(host: FilePaneRuntimeStateHost): void {
    const tabId = host.activeDirTabId ?? host.currentTabId;
    if (!tabId || host.activeSurfaceId !== 'dir') {
      return;
    }

    host.saveDirTabState(tabId);
  }
}