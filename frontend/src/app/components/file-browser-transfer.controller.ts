import type { ApiService } from '../services/api.service';
import type { ArchiveService, ArchiveFrame } from '../services/archive.service';
import type { ConfirmDialogService } from '../services/confirm-dialog.service';
import type { FilePaneBrowserCommands } from './file-pane-browser-commands.factory';

export interface FileBrowserTransferPane {
  selectedFiles: Set<string>;
  archiveStack: ArchiveFrame[];
  getDestinationDir(): string;
  getTreeArchiveSelectionContext(): { archivePath: string; innerPaths: string[] } | null;
  initiatePendingMove(entries: string[], destDir: string): void;
  isInArchive(): boolean;
  browserCommands: Pick<FilePaneBrowserCommands, 'navigation'>;
}

export interface FileBrowserTransferHost {
  leftPane?: FileBrowserTransferPane;
  rightPane?: FileBrowserTransferPane;
  apiService: ApiService;
  archiveService: ArchiveService;
  confirmDialog: ConfirmDialogService;
  buildDestinationPath(sourcePath: string, targetDir: string): Promise<string>;
}

export class FileBrowserTransferController {
  async copy(host: FileBrowserTransferHost, sourceSide: 'left' | 'right', paths: string[]): Promise<void> {
    const { src, dst } = this.resolvePanes(host, sourceSide);
    if (!src || !dst || paths.length === 0) return;
    const destDir = dst.getDestinationDir();

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
  }

  async move(host: FileBrowserTransferHost, sourceSide: 'left' | 'right', paths: string[]): Promise<void> {
    const { src, dst } = this.resolvePanes(host, sourceSide);
    if (!src || !dst || paths.length === 0) return;
    const destDir = dst.getDestinationDir();

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
  }

  private resolvePanes(host: FileBrowserTransferHost, sourceSide: 'left' | 'right'): { src?: FileBrowserTransferPane; dst?: FileBrowserTransferPane } {
    return sourceSide === 'left'
      ? { src: host.leftPane, dst: host.rightPane }
      : { src: host.rightPane, dst: host.leftPane };
  }

  private archiveError(result: any): string | null {
    if (!result?.failed) return null;
    return (result.errors ?? []).map((error: any) => error.error).join('\n');
  }

  private bothArchive(src?: FileBrowserTransferPane, dst?: FileBrowserTransferPane): boolean {
    return !!src?.isInArchive() && !!dst?.isInArchive();
  }
}