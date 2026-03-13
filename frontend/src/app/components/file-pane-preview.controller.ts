import type { ApiService } from '../services/api.service';
import type { WorkspaceService } from '../services/workspace.service';
import type { ConfirmDialogService } from '../services/confirm-dialog.service';
import type { FileInfo } from '../shared/protocol';
import { FileType } from '../shared/protocol';
import type { ArchiveFrame } from '../services/archive.service';
import type { ViewerArchiveContext } from './viewer-surface.component';
import { PARENT_ENTRY_PATH } from './file-entry.component';

export interface FilePanePreviewHost {
  get paneId(): 'left' | 'right';
  get activeTabKind(): 'dir' | 'viewer' | 'size-manager';
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

export class FilePanePreviewController {
  async openQuickView(host: FilePanePreviewHost): Promise<void> {
    if (host.activeTabKind !== 'dir') return;

    const selected = Array.from(host.getEffectiveSelection()).filter(path => path && path !== PARENT_ENTRY_PATH);
    if (selected.length !== 1) return;

    await this.openViewerTab(host, selected[0], {
      activate: true,
      closable: true,
    });
  }

  async openFrozenPreviewTab(
    host: FilePanePreviewHost,
    targetPath: string,
    opts?: { sourcePane: 'left' | 'right'; sourceTabId: string; sourceDir: string; autoFollow?: boolean },
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
    });
  }

  private async openViewerTab(
    host: FilePanePreviewHost,
    path: string,
    opts: { activate: boolean; closable: boolean; autoFollow?: boolean; sourcePane?: 'left' | 'right'; sourceTabId?: string; sourceDir?: string },
  ): Promise<void> {
    try {
      if (host.isInArchive()) {
        const archiveContext = this.buildArchiveViewerContext(host, path);
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