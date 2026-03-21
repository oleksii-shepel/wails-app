import type { ApiService } from '../services/api.service';
import type { ConfirmDialogService } from '../services/confirm-dialog.service';
import type { FileInfo } from '../shared/protocol';
import { FileType } from '../shared/protocol';
import type { ViewLayout } from './file-view-container.component';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { normalizePath, parentPath } from './address-bar.utils';
import { buildArchivePath, parseArchivePath } from '../services/archive.service';

declare const window: any;

export interface FilePaneOperationsHost {
  get paneId(): 'left' | 'right';
  get currentPath(): string;
  get currentView(): ViewLayout;
  get entries(): FileInfo[];
  get selectedFiles(): Set<string>;
  apiService: ApiService;
  confirmDialog: ConfirmDialogService;
  isInArchive(): boolean;
  deleteSelectedFromArchive(effective: Set<string>): Promise<boolean>;
  getEffectiveSelection(): Set<string>;
  loadDirectory(path: string): Promise<void>;
  emitCopyToOther(paths: string[]): void;
  emitMoveToOther(paths: string[]): void;
  startRenameSelected(): void;
  getVisibleTreeNodes(): any[];
}

export class FilePaneOperationsController {
  getDestinationDir(host: FilePaneOperationsHost): string {
    if (host.selectedFiles.size !== 1) return host.currentPath;
    const [selectedPath] = host.selectedFiles;

    const entry = host.entries.find(candidate => candidate.path === selectedPath);
    if (entry?.type === FileType.DIRECTORY) return selectedPath;

    if (host.currentView === 'tree') {
      const nodes = host.getVisibleTreeNodes() ?? [];
      const node = nodes.find((candidate: any) => candidate.path === selectedPath);
      if (node?.type === FileType.DIRECTORY) return selectedPath;
    }

    return host.currentPath;
  }

  async onCreateFolder(host: FilePaneOperationsHost): Promise<void> {
    if (host.isInArchive()) {
      host.confirmDialog.alert('Archive Limitation\nCannot create folders inside an archive.');
      return;
    }

    const folderName = (await host.confirmDialog.prompt({
      title: 'Create Folder',
      message: 'Enter folder name.',
      confirmText: 'Create',
      cancelText: 'Cancel',
      placeholder: 'Folder name',
      defaultValue: '',
    }))?.trim();
    if (!folderName) return;

    try {
      const newPath: string = await window.go.app.App.JoinPaths([host.currentPath, folderName]);
      await host.apiService.createDirectory(newPath, false);
      await host.loadDirectory(host.currentPath);
    } catch (error: any) {
      host.confirmDialog.alert(`Create Folder Failed\n${error.message}`);
    }
  }

  async onDeleteSelected(host: FilePaneOperationsHost): Promise<void> {
    const effective = host.getEffectiveSelection();
    if (effective.size === 0) return;

    if (await host.deleteSelectedFromArchive(effective)) {
      return;
    }

    const deleteConfirmed = await host.confirmDialog.confirm({
      title: 'Delete Items',
      message: `Delete ${effective.size} item(s)?`,
      confirmText: 'Delete',
      danger: true,
    });
    if (!deleteConfirmed) return;

    try {
      const result = await host.apiService.batchDelete(Array.from(effective), true);
      if (result.failed > 0) {
        const msg = (result.errors ?? []).map((error: any) => `${error.source}: ${error.error}`).join('\n');
        host.confirmDialog.alert(`Delete Completed With Errors\n${result.failed} item(s) failed:\n${msg}`);
      }
      await host.loadDirectory(host.currentPath);
    } catch (error: any) {
      host.confirmDialog.alert(`Delete Failed\n${error.message}`);
    }
  }

  getTreeArchiveSelectionContext(host: FilePaneOperationsHost): { archivePath: string; innerPaths: string[] } | null {
    if (host.currentView !== 'tree' || host.selectedFiles.size === 0) return null;

    const visible = host.getVisibleTreeNodes();
    const selected = visible.filter((node: any) => host.selectedFiles.has(node.path));
    if (!selected.length) return null;

    const firstArchivePath = (selected[0] as any).archivePath as string | undefined;
    if (!firstArchivePath) return null;

    const { fsPath, nestedChain } = parseArchivePath(firstArchivePath);
    const contextPath = buildArchivePath(fsPath, nestedChain, '');

    if (!selected.every((node: any) => {
      const nodePath = (node as any).archivePath as string | undefined;
      if (!nodePath) return false;
      const { fsPath: nodeFs, nestedChain: nodeChain } = parseArchivePath(nodePath);
      return buildArchivePath(nodeFs, nodeChain, '') === contextPath;
    })) {
      return null;
    }

    const innerPaths = selected.map((node: any) => {
      const nodePath = (node as any).archivePath as string | undefined;
      return nodePath ? (parseArchivePath(nodePath).innerPath || node.name) : node.name;
    }).filter(Boolean);

    return innerPaths.length ? { archivePath: contextPath, innerPaths } : null;
  }

  onCopyToOther(host: FilePaneOperationsHost): void {
    if (host.selectedFiles.size > 0) {
      host.emitCopyToOther(Array.from(host.selectedFiles));
    }
  }

  async onRenameSelected(host: FilePaneOperationsHost): Promise<void> {
    const selected = Array.from(host.selectedFiles);
    if (selected.length !== 1) return;
    const entry = host.entries.find(candidate => candidate.path === selected[0]);
    if (!entry || entry.path === PARENT_ENTRY_PATH) return;
    if (host.isInArchive()) {
      host.confirmDialog.alert('Archive Limitation\nRename is not supported inside archives.');
      return;
    }
    host.startRenameSelected();
  }

  async onRename(host: FilePaneOperationsHost, event: { entry: FileInfo; newName: string }): Promise<void> {
    await this.renameEntry(host, event.entry, event.newName);
  }

  async onDragDrop(host: FilePaneOperationsHost, event: { event: DragEvent; source: FileInfo[]; targetPath: string }): Promise<void> {
    if (!event.source?.length || !event.targetPath) return;

    if (host.isInArchive()) {
      host.confirmDialog.alert('Archive Drag & Drop Not Supported\nUse copy/move to other pane instead.');
      return;
    }

    const sourcePaths = event.source
      .map(entry => entry.path)
      .filter(path => path && path !== PARENT_ENTRY_PATH);

    if (!sourcePaths.length) return;

    try {
      const isCopy = event.event.ctrlKey || event.event.metaKey;
      const sourcePaneIdRaw = event.event.dataTransfer?.getData('application/x-wails-pane-id') || '';
      const sourcePaneId = (sourcePaneIdRaw === 'left' || sourcePaneIdRaw === 'right') ? sourcePaneIdRaw : null;
      const isCrossPane = !!sourcePaneId && sourcePaneId !== host.paneId;

      const items = await Promise.all(sourcePaths.map(async sourcePath => ({
        source: sourcePath,
        destination: await this.buildDestinationPath(host, sourcePath, event.targetPath),
      })));

      if (!isCopy) {
        const moveItems = items.filter(item => this.normalizePath(item.source) !== this.normalizePath(item.destination));
        if (moveItems.length === 0) return;

        if (isCrossPane) {
          try {
            window.dispatchEvent(new CustomEvent('pane:dnd-optimistic-hide', {
              detail: { paneId: sourcePaneId, paths: moveItems.map(item => item.source) },
            }));
          } catch {
            // ignore
          }
        }

        await host.apiService.batchMove(moveItems, true);
        return;
      }

      await host.apiService.batchCopy(items, true);
    } catch (error: any) {
      try {
        const isCopy = event.event.ctrlKey || event.event.metaKey;
        const sourcePaneIdRaw = event.event.dataTransfer?.getData('application/x-wails-pane-id') || '';
        const sourcePaneId = (sourcePaneIdRaw === 'left' || sourcePaneIdRaw === 'right') ? sourcePaneIdRaw : null;
        const isCrossPane = !!sourcePaneId && sourcePaneId !== host.paneId;
        if (!isCopy && isCrossPane) {
          window.dispatchEvent(new CustomEvent('pane:dnd-optimistic-restore', {
            detail: { paneId: sourcePaneId, paths: sourcePaths },
          }));
        }
      } catch {
        // ignore
      }
      host.confirmDialog.alert(`Drag & Drop Failed\n${error.message}`);
    }
  }

  onMoveToOther(host: FilePaneOperationsHost): void {
    if (host.selectedFiles.size > 0) {
      host.emitMoveToOther(Array.from(host.selectedFiles));
    }
  }

  private async renameEntry(host: FilePaneOperationsHost, entry: FileInfo, forcedName?: string): Promise<void> {
    if (host.isInArchive()) {
      host.confirmDialog.alert('Archive Limitation\nRename is not supported inside archives.');
      return;
    }

    if (entry.path === PARENT_ENTRY_PATH) return;

    const nextName = (forcedName ?? '').trim();
    if (!nextName || nextName === entry.name) return;

    try {
      const destination = await host.apiService.joinPaths([parentPath(entry.path), nextName]);
      await host.apiService.batchMove([{ source: entry.path, destination }], true);
    } catch (error: any) {
      host.confirmDialog.alert(`Rename Failed\n${error.message}`);
    }
  }

  private async buildDestinationPath(host: FilePaneOperationsHost, sourcePath: string, targetDir: string): Promise<string> {
    const split = await host.apiService.splitPath(sourcePath);
    const base = split.base || this.normalizePath(sourcePath).split('/').filter(Boolean).pop() || '';
    return await host.apiService.joinPaths([targetDir, base]);
  }

  private normalizePath(path: string): string {
    return normalizePath(path);
  }
}