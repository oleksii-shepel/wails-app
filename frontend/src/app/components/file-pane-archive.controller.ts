import { FileInfo } from '../shared/protocol';
import { TabInfo } from '../shared/protocol-enhanced';
import {
  ArchiveFrame,
  ArchiveService,
  buildArchiveDisplayPath,
  buildArchiveTabTitle,
  isArchive,
  parseArchivePath,
} from '../services/archive.service';
import { WorkspaceService } from '../services/workspace.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { FileType } from '../shared/protocol';

export interface FilePaneArchiveHost {
  get paneId(): 'left' | 'right';
  get currentTabId(): string;
  get currentPath(): string;
  set currentPath(value: string);
  get fsParentDir(): string;
  set fsParentDir(value: string);
  get archiveStack(): ArchiveFrame[];
  set archiveStack(value: ArchiveFrame[]);
  get entries(): FileInfo[];
  set entries(value: FileInfo[]);
  get selectedFiles(): Set<string>;
  set selectedFiles(value: Set<string>);
  get loading(): boolean;
  set loading(value: boolean);
  get error(): string | null;
  set error(value: string | null);
  get tabs(): TabInfo[];
  set tabs(value: TabInfo[]);
  get archiveMoveWarning(): boolean;
  set archiveMoveWarning(value: boolean);
  get archivePendingMoveEntries(): string[];
  set archivePendingMoveEntries(value: string[]);
  get archivePendingMoveDestDir(): string;
  set archivePendingMoveDestDir(value: string);
  archiveService: ArchiveService;
  workspaceService: WorkspaceService;
  confirmDialog: ConfirmDialogService;
  withParentEntry(raw: FileInfo[], inArchive: boolean, fsPath: string): FileInfo[];
  applyFilter(): void;
  loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void>;
  clearArchiveState(): void;
  focusTreeNode(path: string): void;
}

export class FilePaneArchiveController {
  isInArchive(host: FilePaneArchiveHost): boolean {
    return host.archiveStack?.length > 0;
  }

  async restoreArchiveFromPath(host: FilePaneArchiveHost, path: string): Promise<void> {
    host.loading = true;
    host.error = null;
    try {
      const { fsPath, nestedChain, innerPath } = parseArchivePath(path);

      let frame = await host.archiveService.openRootArchive(fsPath);
      const stack: ArchiveFrame[] = [frame];

      for (let index = 0; index < nestedChain.length; index += 1) {
        frame = await host.archiveService.openNestedArchive(frame, nestedChain.slice(0, index + 1));
        stack.push(frame);
      }

      if (innerPath) {
        frame = await host.archiveService.navigateInto(frame, innerPath);
        stack[stack.length - 1] = frame;
      }

      host.archiveStack = stack;
      this.renderArchiveFrame(host, stack[stack.length - 1]);
      setTimeout(() => host.focusTreeNode(host.currentPath), 0);
    } catch (err: any) {
      host.error = err.message || 'Failed to restore archive tab';
    } finally {
      host.loading = false;
    }
  }

  async onArchiveSegmentClick(host: FilePaneArchiveHost, virtualPath: string): Promise<void> {
    if (!this.isInArchive(host)) {
      host.currentPath = virtualPath;
      host.clearArchiveState();
      await this.restoreArchiveFromPath(host, virtualPath);
      return;
    }

    const fsNorm = (host.archiveStack[0]?.fsPath ?? '').replace(/\\/g, '/');
    const vNorm = virtualPath.replace(/\\/g, '/');
    const afterFs = vNorm.startsWith(fsNorm) ? vNorm.slice(fsNorm.length).replace(/^\//, '') : '';

    if (!afterFs) {
      const rootFrame = host.archiveStack[0];
      host.loading = true;
      try {
        const frame = await host.archiveService.navigateInto(rootFrame, '');
        host.archiveStack = [{ ...frame, innerPath: '' }];
        this.renderArchiveFrame(host, { ...frame, innerPath: '' });
      } catch (err: any) {
        host.error = err.message;
      } finally {
        host.loading = false;
      }
      return;
    }

    const parts = afterFs.split('/').filter(Boolean);
    host.loading = true;
    try {
      let frame = await host.archiveService.navigateInto(host.archiveStack[0], '');
      let stack: ArchiveFrame[] = [{ ...frame, innerPath: '' }];

      for (const part of parts) {
        const isNested = frame.listing.entries.some(
          entry => entry.name === part && (entry.type === FileType.ARCHIVE || isArchive(entry.name)),
        );
        if (isNested) {
          const nestedChain = [...frame.nestedChain, part];
          frame = await host.archiveService.openNestedArchive(frame, nestedChain);
          stack.push(frame);
        } else {
          const innerPath = frame.innerPath ? `${frame.innerPath}/${part}` : part;
          frame = await host.archiveService.navigateInto(stack[stack.length - 1], innerPath);
          stack = [...stack.slice(0, -1), frame];
        }
      }

      host.archiveStack = stack;
      this.renderArchiveFrame(host, stack[stack.length - 1]);
    } catch (err: any) {
      host.error = err.message || 'Failed to navigate to archive segment';
    } finally {
      host.loading = false;
    }
  }

  async openRootArchive(host: FilePaneArchiveHost, fsPath: string): Promise<void> {
    host.loading = true;
    host.error = null;
    const exitPath = host.currentPath;
    try {
      const frame = await host.archiveService.openRootArchive(fsPath);
      host.archiveStack = [frame];
      host.fsParentDir = exitPath;
      this.renderArchiveFrame(host, frame);
    } catch (err: any) {
      host.error = err.message || 'Failed to open archive';
    } finally {
      host.loading = false;
    }
  }

  async archiveNavigateInto(host: FilePaneArchiveHost, folderInnerPath: string): Promise<void> {
    const currentFrame = host.archiveStack[host.archiveStack.length - 1];
    host.loading = true;
    host.error = null;
    try {
      const newFrame = await host.archiveService.navigateInto(currentFrame, folderInnerPath);
      host.archiveStack = [...host.archiveStack.slice(0, -1), newFrame];
      this.renderArchiveFrame(host, newFrame);
    } catch (err: any) {
      host.error = err.message || 'Failed to navigate into folder';
    } finally {
      host.loading = false;
    }
  }

  async openNestedArchive(host: FilePaneArchiveHost, nestedChain: string[]): Promise<void> {
    const currentFrame = host.archiveStack[host.archiveStack.length - 1];
    host.loading = true;
    host.error = null;
    try {
      const newFrame = await host.archiveService.openNestedArchive(currentFrame, nestedChain);
      host.archiveStack = [...host.archiveStack, newFrame];
      this.renderArchiveFrame(host, newFrame);
    } catch (err: any) {
      host.error = err.message || 'Failed to open nested archive';
    } finally {
      host.loading = false;
    }
  }

  async navigateArchiveUp(host: FilePaneArchiveHost): Promise<void> {
    const currentFrame = host.archiveStack[host.archiveStack.length - 1];
    host.loading = true;
    host.error = null;
    try {
      const parentFrame = await host.archiveService.navigateUp(currentFrame);
      if (parentFrame !== null) {
        host.archiveStack = [...host.archiveStack.slice(0, -1), parentFrame];
        this.renderArchiveFrame(host, parentFrame);
        return;
      }

      const poppedStack = host.archiveStack.slice(0, -1);
      if (poppedStack.length === 0) {
        const exitPath = host.fsParentDir || host.currentPath;
        const archivePath = host.archiveStack[0]?.fsPath ?? '';
        host.clearArchiveState();
        await host.loadDirectory(exitPath, { focusPath: archivePath });
        return;
      }

      host.archiveStack = poppedStack;
      this.renderArchiveFrame(host, poppedStack[poppedStack.length - 1]);
    } catch (err: any) {
      host.error = err.message || 'Failed to navigate up';
    } finally {
      host.loading = false;
    }
  }

  exitArchive(host: FilePaneArchiveHost): void {
    if (this.isInArchive(host) && host.currentTabId) {
      host.workspaceService.saveArchiveState(host.paneId, host.currentTabId, {
        path: host.currentPath,
        archiveStack: host.archiveStack,
        fsDirPath: host.archiveStack[0]?.fsPath ?? '',
        fsParentDir: host.fsParentDir,
      });
    }
    host.archiveStack = [];
  }

  async refreshArchive(host: FilePaneArchiveHost): Promise<void> {
    const frame = host.archiveStack[host.archiveStack.length - 1];
    host.loading = true;
    host.error = null;
    try {
      const refreshed = await host.archiveService.refreshFrame(frame);
      host.archiveStack = [...host.archiveStack.slice(0, -1), refreshed];
      this.renderArchiveFrame(host, refreshed);
    } catch (err: any) {
      host.error = err.message || 'Failed to refresh archive';
    } finally {
      host.loading = false;
    }
  }

  renderArchiveFrame(host: FilePaneArchiveHost, frame: ArchiveFrame): void {
    const rawEntries = host.archiveService.listingToFileInfos(frame.listing);
    host.entries = host.withParentEntry(rawEntries, true, '');
    host.selectedFiles = new Set<string>();
    host.applyFilter();

    const fsPath = host.archiveStack[0]?.fsPath ?? '';
    const virtualPath = buildArchiveDisplayPath(fsPath, host.archiveStack);
    host.currentPath = virtualPath;

    const title = buildArchiveTabTitle(fsPath, host.archiveStack);
    host.workspaceService.updateTabPathAndTitle(host.paneId, host.currentTabId, virtualPath, title);
    host.tabs = host.workspaceService.getTabs(host.paneId);
  }

  initiatePendingMove(host: FilePaneArchiveHost, entries: string[], destDir: string): void {
    host.archivePendingMoveEntries = entries;
    host.archivePendingMoveDestDir = destDir;
    host.archiveMoveWarning = true;
  }

  async executePendingMove(host: FilePaneArchiveHost): Promise<void> {
    host.archiveMoveWarning = false;
    const frame = host.archiveStack[host.archiveStack.length - 1];
    const entries = host.archivePendingMoveEntries;
    const destDir = host.archivePendingMoveDestDir;
    host.archivePendingMoveEntries = [];
    host.archivePendingMoveDestDir = '';

    try {
      const result = await host.archiveService.move(frame, entries, destDir);

      if (!result.sourceRemoved) {
        const softMsg = (result.errors ?? []).find((error: any) =>
          error.error.includes('NOT removed'),
        );
        host.confirmDialog.alert(
          `Move Partially Complete\n` +
          `${result.succeeded} file(s) extracted, but source NOT removed:\n` +
          (softMsg?.error ?? 'Nested archive deletion is not supported.'),
        );
        return;
      }

      if (result.failed > 0) {
        host.confirmDialog.alert('Move Errors\n' + (result.errors ?? []).map((error: any) => error.error).join('\n'));
      }

      await this.refreshArchive(host);
    } catch (err: any) {
      host.confirmDialog.alert(`Move Failed\n${err.message}`);
    }
  }

  async deleteSelectedFromArchive(host: FilePaneArchiveHost, effective: Set<string>): Promise<boolean> {
    if (!this.isInArchive(host)) {
      return false;
    }

    const frame = host.archiveStack[host.archiveStack.length - 1];
    const entries = Array.from(effective);

    if (frame.nestedChain.length > 0) {
      host.confirmDialog.alert(
        'Nested Archive Limitation\n' +
        'Exit back to the top-level archive first.',
      );
      return true;
    }

    const archiveDeleteConfirmed = await host.confirmDialog.confirm({
      title: 'Delete from Archive',
      message:
        `Delete ${entries.length} item(s) from the archive?\n` +
        `This rewrites the archive file on disk and cannot be undone.`,
      confirmText: 'Delete',
      danger: true,
    });
    if (!archiveDeleteConfirmed) {
      return true;
    }

    try {
      const result = await host.archiveService.delete(frame, entries);
      if (result.failed > 0) {
        host.confirmDialog.alert('Delete Failed\n' + (result.errors ?? []).map((error: any) => error.error).join('\n'));
      }
    } catch (err: any) {
      host.confirmDialog.alert(`Delete Error\n${err.message}`);
      return true;
    }

    await this.refreshArchive(host);
    return true;
  }
}