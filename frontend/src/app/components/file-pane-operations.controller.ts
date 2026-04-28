import type { ApiService } from '../services/api.service';
import type { ConfirmDialogService } from '../services/confirm-dialog.service';
import { OperationService } from '../services/operation.service';
import type { FileInfo } from '../shared/protocol';
import { FileType } from '../shared/protocol';
import type { ViewLayout } from './file-view-container.component';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { normalizePath, parentPath, samePath } from './address-bar.utils';
import { buildArchivePath, parseArchivePath } from '../services/archive.service';

declare const window: any;

export interface FilePaneOperationsHost {
  get paneId(): 'left' | 'right';
  get currentPath(): string;
  get currentView(): ViewLayout;
  get confirmBeforeDelete(): boolean;
  get confirmDragAndDrop(): boolean;
  get entries(): FileInfo[];
  get selectedFiles(): Set<string>;
  get selectionCursorPath(): string | null;
  apiService: ApiService;
  confirmDialog: ConfirmDialogService;
  isInArchive(): boolean;
  deleteSelectedFromArchive(effective: Set<string>): Promise<boolean>;
  getEffectiveSelection(): Set<string>;
  loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void>;
  refreshTreeNode(path: string): void;
  emitCopyToOther(paths: string[]): void;
  emitMoveToOther(paths: string[]): void;
  startRenameSelected(): void;
  startRenamePath(path: string): void;
  setPendingRenameDisplayName(path: string, displayName: string | null): void;
  getVisibleTreeNodes(): any[];
  isTreeNodeExpanded(path: string): boolean;
  suppressNextTreeAutoExpand(): void;
  focusTreeNode(path: string): void;
}

export class FilePaneOperationsController {
  constructor(private readonly operationService: OperationService) {}

  getDestinationDir(host: FilePaneOperationsHost): string {
    if (host.selectedFiles.size !== 1) return host.currentPath;
    const [selectedPath] = host.selectedFiles;

    if (host.currentView === 'tree') {
      const nodes = host.getVisibleTreeNodes() ?? [];
      const node = nodes.find((candidate: any) => candidate.path === selectedPath);
      if (node?.type === FileType.DIRECTORY) {
        if (host.isTreeNodeExpanded(selectedPath)) {
          return selectedPath;
        }
        return parentPath(selectedPath) || selectedPath;
      }
    }

    const entry = host.entries.find(candidate => candidate.path === selectedPath);
    if (entry?.type === FileType.DIRECTORY) return selectedPath;

    return host.currentPath;
  }

  async onCreateFolder(host: FilePaneOperationsHost): Promise<void> {
    if (host.isInArchive()) {
      host.confirmDialog.alert('Archive Limitation\nCannot create folders inside an archive.');
      return;
    }

    try {
      const targetDir = this.getDestinationDir(host);
      const newPath = await this.createNewFolderForInlineRename(host, targetDir);

      if (host.currentView === 'tree' && !samePath(targetDir, host.currentPath)) {
        host.suppressNextTreeAutoExpand();
        host.refreshTreeNode(targetDir);
      } else {
        await host.loadDirectory(host.currentPath, { preserveUI: true, focusPath: newPath });
      }

      await new Promise<void>((resolve) => {
        requestAnimationFrame(() => {
          host.startRenamePath(newPath);
          resolve();
        });
      });
    } catch (error: any) {
      host.confirmDialog.alert(`Create Folder Failed\n${this.getErrorMessage(error, 'Unknown error')}`);
    }
  }

  async onDeleteSelected(host: FilePaneOperationsHost): Promise<void> {
    const effective = this.pruneNestedSelection(host.getEffectiveSelection());
    if (effective.size === 0) return;
    const postDeletePath = this.getPostDeletePath(host.currentPath, effective);
    const focusPath = postDeletePath ? null : this.getPostDeleteFocusPath(host, effective);
    const treeRefreshParents = host.currentView === 'tree'
      ? Array.from(new Set(
          Array.from(effective)
            .map((path) => parentPath(path))
            .filter((path) => !!path)
        ))
      : [];

    if (await host.deleteSelectedFromArchive(effective)) {
      return;
    }

    if (host.confirmBeforeDelete) {
      const deleteConfirmed = await host.confirmDialog.confirm({
        title: 'Delete Items',
        message: `Delete ${effective.size} item(s)?`,
        confirmText: 'Delete',
        danger: true,
      });
      if (!deleteConfirmed) return;
    }

    try {
      if (postDeletePath) {
        await host.loadDirectory(postDeletePath, { preserveUI: true });
      }

      const result = await host.apiService.batchDelete(Array.from(effective), true);
      if (result.failed > 0) {
        const msg = (result.errors ?? []).map((error: any) => `${error.source}: ${error.error}`).join('\n');
        host.confirmDialog.alert(`Delete Completed With Errors\n${result.failed} item(s) failed:\n${msg}`);
      }
      await host.loadDirectory(postDeletePath ?? host.currentPath, { preserveUI: true, focusPath: focusPath ?? undefined });
      for (const path of treeRefreshParents) {
        host.refreshTreeNode(path);
      }
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

      if (host.confirmDragAndDrop) {
        const confirmed = await host.confirmDialog.confirm({
          title: isCopy ? 'Confirm Copy' : 'Confirm Move',
          message: `${isCopy ? 'Copy' : 'Move'} ${sourcePaths.length} item(s) to ${event.targetPath}?`,
          confirmText: isCopy ? 'Copy' : 'Move',
          danger: !isCopy,
        });
        if (!confirmed) return;
      }

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

  private async createNewFolderForInlineRename(host: FilePaneOperationsHost, targetDir: string): Promise<string> {
    for (let attempt = 1; attempt <= 100; attempt += 1) {
      const folderName = attempt === 1 ? 'New Folder' : `New Folder (${attempt})`;
      const newPath: string = await window.go.app.App.JoinPaths([targetDir, folderName]);
      try {
        const result = await host.apiService.createDirectory(newPath, false);
        if (!result?.success) {
          throw new Error(result?.message || 'Create directory failed');
        }
        return newPath;
      } catch (error: any) {
        if (this.isAlreadyExistsError(error)) {
          continue;
        }
        throw error;
      }
    }

    throw new Error('Unable to create a unique folder name.');
  }

  private isAlreadyExistsError(error: any): boolean {
    const message = this.getErrorMessage(error).toLowerCase();
    return message.includes('already exists') || message.includes('exists');
  }

  private getErrorMessage(error: any, fallback: string = 'Operation failed'): string {
    if (error instanceof Error) {
      return error.message || fallback;
    }
    if (typeof error === 'string') {
      return error || fallback;
    }
    if (typeof error?.message === 'string' && error.message) {
      return error.message;
    }
    if (typeof error?.error === 'string' && error.error) {
      return error.error;
    }
    if (typeof error?.cause?.message === 'string' && error.cause.message) {
      return error.cause.message;
    }
    return fallback;
  }

  private getPostDeletePath(currentPath: string, deletedPaths: Set<string>): string | null {
    const affectedAncestor = Array.from(deletedPaths)
      .filter((deletedPath) => this.isSameOrDescendantPath(currentPath, deletedPath))
      .sort((left, right) => normalizePath(right).length - normalizePath(left).length)[0];

    if (!affectedAncestor) {
      return null;
    }

    return parentPath(affectedAncestor);
  }

  private getPostDeleteFocusPath(host: FilePaneOperationsHost, deletedPaths: Set<string>): string | null {
    const visibleEntries = host.entries.filter((entry) => entry.path !== PARENT_ENTRY_PATH);
    if (visibleEntries.length === 0) return null;

    const anchorPath = this.getDeleteAnchorPath(host, visibleEntries, deletedPaths);
    const anchorIndex = visibleEntries.findIndex((entry) => samePath(entry.path, anchorPath));
    if (anchorIndex < 0) return null;

    for (let index = anchorIndex + 1; index < visibleEntries.length; index += 1) {
      const candidate = visibleEntries[index];
      if (!this.pathSetHas(deletedPaths, candidate.path)) return candidate.path;
    }

    for (let index = anchorIndex - 1; index >= 0; index -= 1) {
      const candidate = visibleEntries[index];
      if (!this.pathSetHas(deletedPaths, candidate.path)) return candidate.path;
    }

    return null;
  }

  private getDeleteAnchorPath(host: FilePaneOperationsHost, visibleEntries: FileInfo[], deletedPaths: Set<string>): string {
    const cursorPath = host.selectionCursorPath;
    if (cursorPath && this.pathSetHas(deletedPaths, cursorPath)) {
      return cursorPath;
    }

    for (const entry of visibleEntries) {
      if (this.pathSetHas(deletedPaths, entry.path)) {
        return entry.path;
      }
    }

    return visibleEntries[0].path;
  }

  private pathSetHas(paths: Set<string>, candidate: string): boolean {
    for (const path of paths) {
      if (samePath(path, candidate)) return true;
    }
    return false;
  }

  private isSameOrDescendantPath(targetPath: string, candidateAncestorPath: string): boolean {
    if (samePath(targetPath, candidateAncestorPath)) {
      return true;
    }

    const target = normalizePath(targetPath);
    const ancestor = normalizePath(candidateAncestorPath);
    const prefix = ancestor.endsWith('/') ? ancestor : `${ancestor}/`;
    return target.startsWith(prefix);
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
      host.setPendingRenameDisplayName(entry.path, nextName);
      const destination = await host.apiService.joinPaths([parentPath(entry.path), nextName]);

      if (samePath(entry.path, destination)) {
        return;
      }

      const result = await host.apiService.batchMove([{ source: entry.path, destination, exactDestination: true }], true);
      if (result?.failed && result.failed > 0) {
        const firstError = result.errors?.[0];
        throw new Error(firstError?.error || 'Rename failed');
      }

      const finalStatus = result?.operationId
        ? await this.operationService.waitForOperationSettlement(result.operationId)
        : (result?.succeeded && result.succeeded > 0 ? 'completed' : 'missing');

      if (finalStatus === 'completed') {
        if (host.currentView === 'tree') {
          host.refreshTreeNode(parentPath(entry.path));
        }
        await host.loadDirectory(host.currentPath, { preserveUI: true, focusPath: destination });
        return;
      }

      host.setPendingRenameDisplayName(entry.path, null);
      if (host.currentView === 'tree') {
        host.focusTreeNode(entry.path);
        return;
      }
      await host.loadDirectory(host.currentPath, { preserveUI: true, focusPath: entry.path });
    } catch (error: any) {
      host.confirmDialog.alert(`Rename Failed\n${this.getErrorMessage(error, 'Unknown error')}`);
    } finally {
      host.setPendingRenameDisplayName(entry.path, null);
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

  private pruneNestedSelection(paths: Set<string>): Set<string> {
    const ordered = Array.from(paths)
      .filter(Boolean)
      .sort((left, right) => normalizePath(left).length - normalizePath(right).length);

    const kept: string[] = [];
    for (const candidate of ordered) {
      if (kept.some((existing) => this.isSameOrDescendantPath(candidate, existing))) {
        continue;
      }
      kept.push(candidate);
    }

    return new Set(kept);
  }
}