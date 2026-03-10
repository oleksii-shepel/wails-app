import type { ApiService } from '../services/api.service';
import type { ConfirmDialogService } from '../services/confirm-dialog.service';

export interface FileBrowserDropPane {
  getDestinationDir(): string;
  isInArchive(): boolean;
  onRefresh(): void | Promise<void>;
}

export interface FileBrowserDropHost {
  getActivePane(): FileBrowserDropPane | undefined;
  getLeftPane(): FileBrowserDropPane | undefined;
  getRightPane(): FileBrowserDropPane | undefined;
  getPanesContainer(): HTMLElement | undefined;
  getLeftPaneWidth(): number;
  apiService: ApiService;
  confirmDialog: ConfirmDialogService;
  buildDestinationPath(sourcePath: string, targetDir: string): Promise<string>;
}

export class FileBrowserDropController {
  async handleNativeFileDrop(host: FileBrowserDropHost, x: number, y: number, paths: string[]): Promise<void> {
    if (!paths.length) {
      return;
    }

    const sourcePaths = paths
      .map(path => (path || '').trim())
      .filter(path => path.length > 0);
    if (!sourcePaths.length) {
      return;
    }

    const targetPane = this.resolveDropTargetPane(host, x, y);
    if (!targetPane) {
      return;
    }

    if (targetPane.isInArchive()) {
      host.confirmDialog.alert('Drag & Drop Not Supported In Archive\nDrop into a filesystem folder instead.');
      return;
    }

    const destinationDir = targetPane.getDestinationDir();
    if (!destinationDir) {
      return;
    }

    try {
      const items = await Promise.all(
        sourcePaths.map(async sourcePath => ({
          source: sourcePath,
          destination: await host.buildDestinationPath(sourcePath, destinationDir),
        }))
      );

      await host.apiService.batchCopy(items, true);
      await targetPane.onRefresh();
    } catch (error: any) {
      host.confirmDialog.alert(`Drag & Drop Failed\n${error?.message ?? String(error)}`);
    }
  }

  private resolveDropTargetPane(host: FileBrowserDropHost, x: number, y: number): FileBrowserDropPane | undefined {
    const container = host.getPanesContainer();
    if (!container) {
      return host.getActivePane();
    }

    const rect = container.getBoundingClientRect();
    const insideContainer = x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom;
    if (!insideContainer) {
      return host.getActivePane();
    }

    const splitX = rect.left + (rect.width * host.getLeftPaneWidth() / 100);
    return x <= splitX ? host.getLeftPane() : host.getRightPane();
  }
}