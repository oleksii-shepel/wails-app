import type { ApiService } from '../services/api.service';
import type { ConfirmDialogService } from '../services/confirm-dialog.service';
import { BatchItemError, operationErrorMessage } from '../shared/protocol-enhanced';
import type { SourceCapabilities } from '../services/source-capabilities';
import { canReceiveTransfer, transferBlockedMessage } from '../services/transfer-capabilities';

export interface FileBrowserClipboardPane {
  currentPath: string;
  selectedFiles: Set<string>;
  sourceCapabilities: SourceCapabilities;
}

export interface FileBrowserClipboardHost {
  getActivePane(): FileBrowserClipboardPane | undefined;
  getActivePaneId(): 'left' | 'right';
  apiService: ApiService;
  confirmDialog: ConfirmDialogService;
  buildDestinationPath(sourcePath: string, targetDir: string): Promise<string>;
  runCrossPaneCopy(sourceSide: 'left' | 'right', paths: string[]): Promise<void>;
  runCrossPaneMove(sourceSide: 'left' | 'right', paths: string[]): Promise<void>;
}

export class FileBrowserClipboardController {
  copySelectedToClipboard(host: FileBrowserClipboardHost): { operation: 'copy' | 'cut'; paths: string[] } | null {
    const pane = host.getActivePane();
    if (!pane?.selectedFiles.size) {
      return null;
    }
    return { operation: 'copy', paths: Array.from(pane.selectedFiles) };
  }

  cutSelectedToClipboard(host: FileBrowserClipboardHost): { operation: 'copy' | 'cut'; paths: string[] } | null {
    const pane = host.getActivePane();
    if (!pane?.selectedFiles.size) {
      return null;
    }
    return { operation: 'cut', paths: Array.from(pane.selectedFiles) };
  }

  async pasteFromClipboard(
    host: FileBrowserClipboardHost,
    clipboard: { operation: 'copy' | 'cut'; paths: string[] } | null,
  ): Promise<{ operation: 'copy' | 'cut'; paths: string[] } | null> {
    if (!clipboard) {
      return clipboard;
    }

    const targetPane = host.getActivePane();
    if (!targetPane) {
      return clipboard;
    }

    const operation = clipboard.operation === 'copy' ? 'copy' : 'move';
    const destinationDecision = canReceiveTransfer(targetPane.sourceCapabilities);
    if (!destinationDecision.allowed) {
      host.confirmDialog.alert(transferBlockedMessage(operation, destinationDecision));
      return clipboard;
    }

    try {
      const items = await Promise.all(clipboard.paths.map(async sourcePath => ({
        source: sourcePath,
        destination: await host.buildDestinationPath(sourcePath, targetPane.currentPath),
      })));

      const result = clipboard.operation === 'copy'
        ? await host.apiService.batchCopy(items, true)
        : await host.apiService.batchMove(items, true);

      if (result.failed > 0) {
        const errorMsg = (result.errors as BatchItemError[] ?? []).map(e =>
          `${e.source}: ${operationErrorMessage(e.code)}${e.code === 'unknown' && e.error ? ` (${e.error})` : ''}`
        ).join('\n');
        host.confirmDialog.alert(`Paste Completed with ${result.failed} Errors\n${errorMsg}`);
      }
    } catch (error: any) {
      host.confirmDialog.alert(`Paste Error\n${error.message}`);
    }

    return clipboard.operation === 'cut' ? null : clipboard;
  }

  async copyToOtherPaneCurrent(host: FileBrowserClipboardHost): Promise<void> {
    const pane = host.getActivePane();
    const paths = Array.from(pane?.selectedFiles || []);
    if (paths.length === 0) {
      return;
    }
    await host.runCrossPaneCopy(host.getActivePaneId(), paths);
  }

  async moveToOtherPaneCurrent(host: FileBrowserClipboardHost): Promise<void> {
    const pane = host.getActivePane();
    const paths = Array.from(pane?.selectedFiles || []);
    if (paths.length === 0) {
      return;
    }
    await host.runCrossPaneMove(host.getActivePaneId(), paths);
  }
}