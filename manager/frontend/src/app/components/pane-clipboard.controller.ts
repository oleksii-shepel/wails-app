import type { FileInfo } from '../shared/protocol';
import type { ArchiveFrame } from '../services/archive.service';
import { ArchiveService } from '../services/archive.service';
import { ApiService } from '../services/api.service';
import { FileClipboardService } from '../services/file-clipboard.service';
import { OperationHistoryService } from '../services/operation-history.service';
import { ActionFeedbackService } from '../services/action-feedback.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';

export interface PaneClipboardHost {
  readonly paneId: 'left' | 'right';
  readonly path: string;
  readonly selectedEntries: readonly FileInfo[];
  readonly canCut: boolean;
  readonly canPaste: boolean;
  readonly currentArchiveFrame: ArchiveFrame | null;
  refresh(): Promise<void>;
  focusContent?(): void;
}

/** Shared pane-local clipboard behavior for directory-capable pane implementations. */
export class PaneClipboardController {
  constructor(
    private readonly clipboard: FileClipboardService,
    private readonly api: ApiService,
    private readonly archive: ArchiveService,
    private readonly history: OperationHistoryService,
    private readonly feedback: ActionFeedbackService,
    private readonly confirm: ConfirmDialogService,
  ) {}

  copy(host: PaneClipboardHost): void {
    const entries = host.selectedEntries;
    if (!entries.length) return;

    this.clipboard.set({
      operation: 'copy',
      paths: entries.map(entry => entry.path),
      sourcePaneId: host.paneId,
      archiveFrame: this.cloneArchiveFrame(host.currentArchiveFrame),
    });
  }

  cut(host: PaneClipboardHost): void {
    if (!host.canCut) return;
    const entries = host.selectedEntries;
    if (!entries.length) return;

    this.clipboard.set({
      operation: 'cut',
      paths: entries.map(entry => entry.path),
      sourcePaneId: host.paneId,
      archiveFrame: this.cloneArchiveFrame(host.currentArchiveFrame),
    });
  }

  async paste(host: PaneClipboardHost): Promise<void> {
    const payload = this.clipboard.payload;
    if (!payload || !host.canPaste) return;

    try {
      if (payload.archiveFrame) {
        const result = payload.operation === 'copy'
          ? await this.archive.copy(payload.archiveFrame, payload.paths, host.path)
          : await this.archive.move(payload.archiveFrame, payload.paths, host.path);

        if (payload.operation === 'cut' && result.failed === 0 && result.sourceRemoved) {
          this.clipboard.clear();
        }

        await host.refresh();

        if (result.failed > 0) {
          await this.confirm.alert({
            title: 'Paste Completed with Errors',
            message: `${result.failed} archive entr${result.failed === 1 ? 'y' : 'ies'} could not be ${payload.operation === 'copy' ? 'extracted' : 'moved'}.`,
          });
        }
        return;
      }

      const items = await Promise.all(payload.paths.map(async source => ({
        source,
        destination: await this.api.joinPaths([host.path, this.leafName(source)]),
      })));

      const result = payload.operation === 'copy'
        ? await this.api.batchCopy(items, true)
        : await this.api.batchMove(items, true);

      const affected = result.affectedPaths ?? [];
      if (
        result.failed === 0 &&
        result.skipped === 0 &&
        affected.length === result.succeeded &&
        affected.length > 0
      ) {
        this.history.record({
          label: `${payload.operation === 'copy' ? 'Paste copy' : 'Paste move'} ${affected.length} item${affected.length === 1 ? '' : 's'}`,
          kind: payload.operation === 'copy' ? 'paste-copy' : 'paste-move',
          affectedPaths: affected.map(item => item.destination),
          summary: `${affected.length} item${affected.length === 1 ? '' : 's'}`,
          undo: async () => {
            if (payload.operation === 'copy') {
              const deletion = await this.api.batchDelete(
                affected.map(item => item.destination),
                true,
                false,
              );
              if (deletion.failed > 0 || deletion.skipped > 0) {
                throw new Error('Undo paste copy failed.');
              }
              return;
            }

            const back = await this.api.batchMove(
              affected.map(item => ({
                source: item.destination,
                destination: item.source,
                exactDestination: true,
              })),
              true,
            );
            if (back.failed > 0 || back.skipped > 0) {
              throw new Error('Undo paste move failed.');
            }
          },
          redo: async () => {
            const forward = affected.map(item => ({
              source: item.source,
              destination: item.destination,
              exactDestination: true,
            }));
            const redone = payload.operation === 'copy'
              ? await this.api.batchCopy(forward, true)
              : await this.api.batchMove(forward, true);
            if (redone.failed > 0 || redone.skipped > 0) {
              throw new Error('Redo paste operation failed.');
            }
          },
        });
      } else if (result.succeeded > 0 || result.failed > 0 || result.skipped > 0) {
        this.history.recordOutcome({
          label: `${payload.operation === 'copy' ? 'Paste copy' : 'Paste move'} ${result.total} item${result.total === 1 ? '' : 's'}`,
          kind: payload.operation === 'copy' ? 'paste-copy' : 'paste-move',
          reversible: false,
          affectedPaths: affected.map(item => item.destination),
          state: result.failed > 0 || result.skipped > 0 ? 'partial' : 'completed',
          summary: `${result.succeeded} succeeded · ${result.skipped} skipped · ${result.failed} failed`,
        });
      }

      if (payload.operation === 'cut' && result.failed === 0) {
        this.clipboard.clear();
      }

      if (result.failed === 0 && result.skipped === 0) {
        this.feedback.success(
          payload.operation === 'copy' ? 'Paste completed' : 'Move completed',
          `${result.succeeded} item${result.succeeded === 1 ? '' : 's'}`,
        );
      } else {
        this.feedback.warning(
          'Paste completed with issues',
          `${result.succeeded} succeeded · ${result.skipped} skipped · ${result.failed} failed`,
        );
      }

      // ApiService publishes filesystem copy/move mutations. The pane observer
      // reconciles the destination without a second eager directory reload.
      host.focusContent?.();

      if (result.failed > 0 || result.skipped > 0) {
        await this.confirm.alert({
          title: 'Paste Completed with Issues',
          message: [
            result.failed > 0
              ? `${result.failed} item${result.failed === 1 ? '' : 's'} failed.`
              : '',
            result.skipped > 0
              ? `${result.skipped} item${result.skipped === 1 ? '' : 's'} were skipped.`
              : '',
          ].filter(Boolean).join('\n'),
        });
      }
    } catch (error) {
      await this.confirm.alert({
        title: 'Paste Failed',
        message: error instanceof Error ? error.message : String(error),
      });
    }
  }

  private cloneArchiveFrame(frame: ArchiveFrame | null): ArchiveFrame | null {
    if (!frame) return null;
    return {
      ...frame,
      nestedChain: [...(frame.nestedChain ?? [])],
      listing: {
        ...frame.listing,
        entries: (frame.listing.entries ?? []).map(entry => ({ ...entry })),
      },
    };
  }

  private leafName(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/').replace(/\/+$/, '');
    const index = normalized.lastIndexOf('/');
    return index >= 0 ? normalized.slice(index + 1) : normalized;
  }
}