import { Injectable } from '@angular/core';
import { createBehaviorSubject, type Stream } from '@epikodelabs/streamix';

export interface ConfirmDialogOptions {
  title?: string;
  message: string;
  confirmText?: string;
  cancelText?: string;
  danger?: boolean;
  isAlert?: boolean; // If true, only shows OK button (no cancel)
}

interface ConfirmDialogRequest {
  options: Required<Pick<ConfirmDialogOptions, 'message'>> & ConfirmDialogOptions;
  resolve: (value: boolean) => void;
}

@Injectable({ providedIn: 'root' })
export class ConfirmDialogService {
  public readonly activeRequest$ = createBehaviorSubject<ConfirmDialogRequest | null>(null);
  
  private readonly queue: ConfirmDialogRequest[] = [];

  confirm(options: ConfirmDialogOptions | string): Promise<boolean> {
    const normalized = this.normalizeOptions(options);

    return new Promise<boolean>((resolve) => {
      this.queue.push({ options: normalized, resolve });
      this.tryOpenNext();
    });
  }

  alert(options: ConfirmDialogOptions | string): Promise<void> {
    const normalized = this.normalizeOptions(options);
    normalized.isAlert = true;
    normalized.confirmText = normalized.confirmText ?? 'OK';

    return new Promise<void>((resolve) => {
      this.queue.push({
        options: normalized,
        resolve: () => resolve(),
      });
      this.tryOpenNext();
    });
  }

  resolveActive(confirmed: boolean): void {
    const active = this.activeRequest$.value;
    if (!active) return;

    active.resolve(confirmed);
    this.activeRequest$.next(null);
    this.tryOpenNext();
  }

  private tryOpenNext(): void {
    if (this.activeRequest$.value) return;
    const next = this.queue.shift() ?? null;
    this.activeRequest$.next(next);
  }

  private normalizeOptions(options: ConfirmDialogOptions | string): Required<Pick<ConfirmDialogOptions, 'message'>> & ConfirmDialogOptions {
    if (typeof options === 'string') {
      return {
        message: options,
        confirmText: 'Confirm',
        cancelText: 'Cancel',
      };
    }

    return {
      ...options,
      message: options.message,
      confirmText: options.confirmText ?? 'Confirm',
      cancelText: options.cancelText ?? 'Cancel',
    };
  }
}
