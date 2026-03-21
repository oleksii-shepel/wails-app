import { Injectable } from '@angular/core';
import { createBehaviorSubject, type Stream } from '@epikodelabs/streamix';
import { formatPathsInText } from '../components/address-bar.utils';

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
}

interface ConfirmDialogRequest {
  options: Required<Pick<ConfirmDialogOptions, 'message'>> & ConfirmDialogOptions;
  resolve: (value: boolean | string | null) => void;
}

@Injectable({ providedIn: 'root' })
export class ConfirmDialogService {
  public readonly activeRequest$ = createBehaviorSubject<ConfirmDialogRequest | null>(null);
  
  private readonly queue: ConfirmDialogRequest[] = [];

  confirm(options: ConfirmDialogOptions | string): Promise<boolean> {
    const normalized = this.normalizeOptions(options);

    return new Promise<boolean>((resolve) => {
      this.queue.push({
        options: normalized,
        resolve: (value) => resolve(value === true),
      });
      this.tryOpenNext();
    });
  }

  alert(options: ConfirmDialogOptions | string): Promise<void> {
    const normalized = this.normalizeOptions(options);
    normalized.isAlert = true;
    normalized.confirmText = 'OK';

    return new Promise<void>((resolve) => {
      this.queue.push({
        options: normalized,
        resolve: () => resolve(),
      });
      this.tryOpenNext();
    });
  }

  prompt(options: ConfirmDialogOptions | string): Promise<string | null> {
    const normalized = this.normalizeOptions(options);
    normalized.isPrompt = true;

    return new Promise<string | null>((resolve) => {
      this.queue.push({
        options: normalized,
        resolve: (value) => resolve(typeof value === 'string' ? value : null),
      });
      this.tryOpenNext();
    });
  }

  resolveActive(result: boolean | string | null): void {
    const active = this.activeRequest$.value;
    if (!active) return;

    active.resolve(result);
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
