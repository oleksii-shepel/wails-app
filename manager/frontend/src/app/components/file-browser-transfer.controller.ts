import type { ApiService } from '../services/api.service';
import type { ArchiveService, ArchiveFrame } from '../services/archive.service';
import type { ConfirmDialogService } from '../services/confirm-dialog.service';
import type { FilePaneBrowserCommands } from './file-pane-browser-commands.factory';
import { BatchItemError, operationErrorMessage } from '../shared/protocol-enhanced';
import type { SourceCapabilities } from '../services/source-capabilities';
import { canTransferBetween, transferBlockedMessage } from '../services/transfer-capabilities';

export interface FileBrowserTransferPane {
  currentPath: string;
  selectedFiles: Set<string>;
  archiveStack: ArchiveFrame[];
  sourceCapabilities: SourceCapabilities;
  getDestinationDir(): string;
  getTreeArchiveSelectionContext(): { archivePath: string; innerPaths: string[] } | null;
  initiatePendingMove(entries: string[], destDir: string): void;
  isInArchive(): boolean;
  browserCommands: Pick<FilePaneBrowserCommands, 'navigation'>;
}

export interface FileBrowserTransferHost {
  leftPane?: FileBrowserTransferPane;
  rightPane?: FileBrowserTransferPane;
  get confirmBeforeCopy(): boolean;
  get confirmBeforeMove(): boolean;
  apiService: ApiService;
  archiveService: ArchiveService;
  confirmDialog: ConfirmDialogService;
  buildDestinationPath(sourcePath: string, targetDir: string): Promise<string>;
}

export interface FileBrowserTransferOptions {
  skipConfirmation?: boolean;
}

export class FileBrowserTransferController {
  async copy(host: FileBrowserTransferHost, sourceSide: 'left' | 'right', paths: string[], options: FileBrowserTransferOptions = {}): Promise<void> {
    const { src, dst } = this.resolvePanes(host, sourceSide);
    if (!src || !dst || paths.length === 0) return;
    const destDir = this.resolveCrossPaneDestination(dst);
    const decision = canTransferBetween('copy', src.sourceCapabilities, dst.sourceCapabilities);
    if (!decision.allowed) { host.confirmDialog.alert(transferBlockedMessage('copy', decision)); return; }

    if (host.confirmBeforeCopy && !options.skipConfirmation) {
      const confirmed = await host.confirmDialog.confirm({
        title: 'Copy to Other Pane',
        message: `Copy ${paths.length} item(s) to ${destDir}?`,
        confirmText: 'Copy',
      });
      if (!confirmed) return;
    }

    if (src.isInArchive()) {
      if (this.bothArchive(src, dst)) {
        host.confirmDialog.alert('Cannot copy between two archive panes\nOpen a real folder in one pane first.');
        return;
      }
      const frame = src.archiveStack[src.archiveStack.length - 1];
      const result = await host.archiveService.copy(frame, paths, destDir);
      const err = this.archiveError(result);
      if (err) host.confirmDialog.alert(`Copy Completed with Errors\n${err}`);
      await dst.browserCommands.navigation.refresh();
      return;
    }

    const treeArchive = src.getTreeArchiveSelectionContext();
    if (treeArchive) {
      const result = await host.archiveService.copyByContext(treeArchive.archivePath, treeArchive.innerPaths, destDir);
      const err = this.archiveError(result);
      if (err) host.confirmDialog.alert(`Copy Errors\n${err}`);
      await dst.browserCommands.navigation.refresh();
      return;
    }

    const items = await Promise.all(
      paths.map(async srcPath => ({
        source: srcPath,
        destination: await host.buildDestinationPath(srcPath, destDir),
      }))
    );
    const result = await host.apiService.batchCopy(items, true);
    const err = this.archiveError(result);
    if (err) host.confirmDialog.alert(`Copy Errors\n${err}`);
    await dst.browserCommands.navigation.refresh();
  }

  async copyRelative(
    host: FileBrowserTransferHost,
    sourceSide: 'left' | 'right',
    items: readonly { source: string; relativePath: string }[],
  ): Promise<void> {
    const { src, dst } = this.resolvePanes(host, sourceSide);
    if (!src || !dst || items.length === 0) return;
    const destDir = this.resolveCrossPaneDestination(dst);
    const decision = canTransferBetween('copy', src.sourceCapabilities, dst.sourceCapabilities);
    if (!decision.allowed) { host.confirmDialog.alert(transferBlockedMessage('copy', decision)); return; }
    if (src.isInArchive() || dst.isInArchive()) {
      host.confirmDialog.alert('Recursive synchronization is available only between regular folder panes.');
      return;
    }

    const batch = await Promise.all(items.map(async item => ({
      source: item.source,
      destination: await host.apiService.joinPaths([destDir, ...item.relativePath.split('/').filter(Boolean)]),
    })));
    const result = await host.apiService.batchCopy(batch, true);
    const err = this.archiveError(result);
    if (err) host.confirmDialog.alert(`Copy Errors\n${err}`);
    await dst.browserCommands.navigation.refresh();
  }

  async move(host: FileBrowserTransferHost, sourceSide: 'left' | 'right', paths: string[]): Promise<void> {
    const { src, dst } = this.resolvePanes(host, sourceSide);
    if (!src || !dst || paths.length === 0) return;
    const destDir = this.resolveCrossPaneDestination(dst);
    const decision = canTransferBetween('move', src.sourceCapabilities, dst.sourceCapabilities);
    if (!decision.allowed) { host.confirmDialog.alert(transferBlockedMessage('move', decision)); return; }

    if (host.confirmBeforeMove) {
      const confirmed = await host.confirmDialog.confirm({
        title: 'Move to Other Pane',
        message: `Move ${paths.length} item(s) to ${destDir}?`,
        confirmText: 'Move',
      });
      if (!confirmed) return;
    }

    if (src.isInArchive()) {
      if (this.bothArchive(src, dst)) {
        host.confirmDialog.alert('Cannot move between two archive panes\nOpen a real folder in one pane first.');
        return;
      }
      src.initiatePendingMove(paths, destDir);
      return;
    }

    const treeArchive = src.getTreeArchiveSelectionContext();
    if (treeArchive) {
      const result = await host.archiveService.moveByContext(treeArchive.archivePath, treeArchive.innerPaths, destDir);
      const err = this.archiveError(result);
      if (err) host.confirmDialog.alert(`Move Errors\n${err}`);
      await src.browserCommands.navigation.refresh();
      await dst.browserCommands.navigation.refresh();
      return;
    }

    const items = await Promise.all(
      paths.map(async srcPath => ({
        source: srcPath,
        destination: await host.buildDestinationPath(srcPath, destDir),
      }))
    );
    const result = await host.apiService.batchMove(items, true);
    const err = this.archiveError(result);
    if (err) host.confirmDialog.alert(`Move Errors\n${err}`);
    await src.browserCommands.navigation.refresh();
    await dst.browserCommands.navigation.refresh();
  }

  private resolvePanes(host: FileBrowserTransferHost, sourceSide: 'left' | 'right'): { src?: FileBrowserTransferPane; dst?: FileBrowserTransferPane } {
    return sourceSide === 'left'
      ? { src: host.leftPane, dst: host.rightPane }
      : { src: host.rightPane, dst: host.leftPane };
  }

  private resolveCrossPaneDestination(pane: FileBrowserTransferPane): string {
    return pane.currentPath || pane.getDestinationDir();
  }

  private archiveError(result: any): string | null {
    if (!result?.failed) return null;
    return (result.errors as BatchItemError[] ?? []).map(e => {
      const label = operationErrorMessage(e.code);
      const detail = e.code === 'unknown' && e.error ? ` (${e.error})` : '';
      return `${e.source}: ${label}${detail}`;
    }).join('\n');
  }

  private bothArchive(src?: FileBrowserTransferPane, dst?: FileBrowserTransferPane): boolean {
    return !!src?.isInArchive() && !!dst?.isInArchive();
  }
}