import type { ApiService } from '../services/api.service';
import type { WorkspaceService } from '../services/workspace.service';
import type { ConfirmDialogService } from '../services/confirm-dialog.service';
import type { FileInfo } from '../shared/protocol';
import { FileType } from '../shared/protocol';
import type { ArchiveFrame } from '../services/archive.service';
import type { ViewerArchiveContext } from '../viewers/viewer-archive-context';
import { PARENT_ENTRY_PATH } from './file-entry.component';

export interface FilePanePreviewHost {
  get paneId(): 'left' | 'right';
  get activeTabKind(): 'dir' | 'viewer';
  get entries(): FileInfo[];
  get archiveStack(): ArchiveFrame[];
  get tabs(): any[];
  set tabs(value: any[]);
  apiService: ApiService;
  workspaceService: WorkspaceService;
  confirmDialog: ConfirmDialogService;
  isInArchive(): boolean;
  getEffectiveSelection(): Set<string>;
  onTabChange(tab: any): Promise<void>;
}

export interface FilePanePreviewRequest {
	path: string;
	archiveContext: ViewerArchiveContext | null;
}

export class FilePanePreviewController {
  getPreviewRequest(host: FilePanePreviewHost): FilePanePreviewRequest | null {
    if (host.activeTabKind !== 'dir') return null;

    const selected = Array.from(host.getEffectiveSelection()).filter(path => path && path !== PARENT_ENTRY_PATH);
    if (selected.length !== 1) return null;

    const path = selected[0];
    if (host.isInArchive()) {
      const archiveContext = this.buildArchiveViewerContext(host, path);
      if (!archiveContext) return null;
      return { path, archiveContext };
    }

    const entry = host.entries.find(candidate => candidate.path === path);
    if (!entry || (entry.type !== FileType.FILE && entry.type !== FileType.SYMLINK)) return null;
    return { path, archiveContext: null };
  }

  async openQuickView(host: FilePanePreviewHost): Promise<void> {
    const request = this.getPreviewRequest(host);
    if (!request) return;

    await this.openViewerTab(host, request.path, {
      activate: true,
      closable: true,
      archiveContext: request.archiveContext,
    });
  }

  async openFrozenPreviewTab(
    host: FilePanePreviewHost,
    targetPath: string,
    opts?: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string; autoFollow?: boolean; archiveContext?: ViewerArchiveContext | null },
  ): Promise<void> {
    const path = (targetPath || '').trim();
    if (!path) return;

    await this.openViewerTab(host, path, {
      activate: true,
      closable: true,
      autoFollow: !!opts?.autoFollow,
      sourcePane: opts?.sourcePane,
      sourceTabId: opts?.sourceTabId,
      sourceDir: opts?.sourceDir,
      archiveContext: opts?.archiveContext ?? null,
    });
  }

  private async openViewerTab(
    host: FilePanePreviewHost,
    path: string,
    opts: { activate: boolean; closable: boolean; autoFollow?: boolean; sourcePane?: 'left' | 'right'; sourceTabId?: string; sourceDir?: string; archiveContext?: ViewerArchiveContext | null },
  ): Promise<void> {
    try {
      if (host.isInArchive() || opts.archiveContext) {
        const archiveContext = opts.archiveContext ?? this.buildArchiveViewerContext(host, path);
        if (!archiveContext) return;

        const tab = host.workspaceService.addViewerTab(host.paneId, path, {
          activate: opts.activate,
          closable: opts.closable,
          archiveContext,
        });
        if (!tab) return;
        host.tabs = host.workspaceService.getTabs(host.paneId);
        await host.onTabChange(tab);
        return;
      }

      const isDir = await host.apiService.isDirectory(path);
      if (isDir) return;

      const tab = host.workspaceService.addViewerTab(host.paneId, path, {
        activate: opts.activate,
        closable: opts.closable,
        autoFollow: !!opts.autoFollow,
        sourcePane: opts.sourcePane,
        sourceTabId: opts.sourceTabId,
        sourceDir: opts.sourceDir,
      });
      if (!tab) return;

      host.tabs = host.workspaceService.getTabs(host.paneId);
      await host.onTabChange(tab);
    } catch (err: any) {
      host.confirmDialog.alert(`Preview Failed\n${err?.message || String(err)}`);
    }
  }

  private buildArchiveViewerContext(host: FilePanePreviewHost, path: string): ViewerArchiveContext | null {
    const frame = host.archiveStack[host.archiveStack.length - 1];
    const entry = host.entries.find(candidate => candidate.path === path);
    if (!frame || !entry || entry.type === FileType.DIRECTORY) {
      return null;
    }

    return {
      fsPath: host.archiveStack[0].fsPath,
      nestedChain: frame.nestedChain ?? [],
      entryPath: entry.path,
    };
  }
}