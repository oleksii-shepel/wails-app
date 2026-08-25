import { Injectable } from '@angular/core';
import { formatPathsInText } from '../components/address-bar.utils';
import { QueuedDialogGate } from './dialog-gate';

export type ConfirmDialogTone = 'warning' | 'info' | 'success' | 'error';

export interface ConfirmDialogDetailRow {
  label: string;
  value: string | readonly string[];
  monospace?: boolean;
}

export interface ConfirmDialogDetailSection {
  title?: string;
  rows: readonly ConfirmDialogDetailRow[];
}

export interface ConfirmDialogOptions {
  title?: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
  isAlert?: boolean; // If true, only shows OK button (no cancel)
  isPrompt?: boolean;
  defaultValue?: string;
  placeholder?: string;
  /** When set, typed values are persisted and offered as autocomplete suggestions. */
  historyKey?: string;
  /** Optional structured content for information/alert dialogs. */
  details?: readonly ConfirmDialogDetailSection[];
  /** Visual treatment for alert dialogs. Existing alerts default to warning. */
  tone?: ConfirmDialogTone;
  /** Set false to suppress the keyboard hint in alert footers. */
  footerHint?: string | false;
}

interface ConfirmDialogRequest {
  kind: 'confirm' | 'alert' | 'prompt';
  options: Required<Pick<ConfirmDialogOptions, 'message'>> & ConfirmDialogOptions;
  resolves: Array<(value: boolean | string | null) => void>;
  resolve(result: boolean | string | null): void;
}

@Injectable({ providedIn: 'root' })
export class ConfirmDialogService {
  private readonly gate = new QueuedDialogGate<ConfirmDialogRequest, boolean | string | null>();
  public readonly activeRequest$ = this.gate.activeRequest$;
  public readonly mountedRequest$ = this.gate.mountedRequest$;
  public readonly closingRequest$ = this.gate.closingRequest$;

  confirm(options: ConfirmDialogOptions | string): Promise<boolean> {
    const normalized = this.normalizeOptions(options);

    return new Promise<boolean>((resolve) => {
      this.enqueueOrJoin('confirm', normalized, (value) => resolve(value === true));
    });
  }

  alert(options: ConfirmDialogOptions | string): Promise<void> {
    const normalized = this.normalizeOptions(options);
    normalized.isAlert = true;
    normalized.confirmText = 'OK';
    normalized.tone ??= 'warning';

    return new Promise<void>((resolve) => {
      this.enqueueOrJoin('alert', normalized, () => resolve());
    });
  }

  prompt(options: ConfirmDialogOptions | string): Promise<string | null> {
    const normalized = this.normalizeOptions(options);
    normalized.isPrompt = true;

    return new Promise<string | null>((resolve) => {
      this.enqueueOrJoin('prompt', normalized, (value) => resolve(typeof value === 'string' ? value : null));
    });
  }

  resolveActive(result: boolean | string | null): void {
    this.gate.beginClose(result);
  }

  finishClose(): void {
    this.gate.finishClose();
  }

  private enqueueOrJoin(
    kind: ConfirmDialogRequest['kind'],
    options: Required<Pick<ConfirmDialogOptions, 'message'>> & ConfirmDialogOptions,
    resolve: (value: boolean | string | null) => void,
  ): void {
    const duplicate = this.findDuplicate(kind, options);
    if (duplicate) {
      duplicate.resolves.push(resolve);
      return;
    }

    this.gate.enqueue({
      kind,
      options,
      resolves: [resolve],
      resolve(result) {
        for (const entry of this.resolves) {
          entry(result);
        }
      },
    });
  }

  private findDuplicate(
    kind: ConfirmDialogRequest['kind'],
    options: Required<Pick<ConfirmDialogOptions, 'message'>> & ConfirmDialogOptions,
  ): ConfirmDialogRequest | null {
    const active = this.gate.activeRequest;
    if (active && this.requestsMatch(active, kind, options)) {
      return active;
    }

    return this.gate.queuedRequests.find(request => this.requestsMatch(request, kind, options)) ?? null;
  }

  private requestsMatch(
    request: ConfirmDialogRequest,
    kind: ConfirmDialogRequest['kind'],
    options: Required<Pick<ConfirmDialogOptions, 'message'>> & ConfirmDialogOptions,
  ): boolean {
    return request.kind === kind &&
      JSON.stringify(request.options) === JSON.stringify(options);
  }

  private normalizeOptions(options: ConfirmDialogOptions | string): Required<Pick<ConfirmDialogOptions, 'message'>> & ConfirmDialogOptions {
    if (typeof options === 'string') {
      return {
        message: formatPathsInText(options),
        confirmText: 'Confirm',
        cancelText: 'Cancel',
      };
    }

    return {
      ...options,
      title: options.title ? formatPathsInText(options.title) : options.title,
      message: formatPathsInText(options.message),
      confirmText: options.confirmText ?? 'Confirm',
      cancelText: options.cancelText ?? 'Cancel',
    };
  }
}
