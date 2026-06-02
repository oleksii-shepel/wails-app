import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import type { FileInfo } from '../shared/protocol';

export interface BatchRenameItem {
  entry: FileInfo;
  newName: string;
}

export interface BatchRenameOptions {
  pattern: string;
  startAt: number;
  increment: number;
}

export interface BatchRenameRequest {
  items: BatchRenameItem[];
  currentPath: string;
  resolve: (value: BatchRenameItem[] | null) => void;
}

@Injectable({ providedIn: 'root' })
export class BatchRenameDialogService {
  public readonly activeRequest$ = createBehaviorSubject<BatchRenameRequest | null>(null);

  private queue: BatchRenameRequest[] = [];

  openDialog(items: BatchRenameItem[], currentPath: string): Promise<BatchRenameItem[] | null> {
    return new Promise<BatchRenameItem[] | null>((resolve) => {
      this.queue.push({ items, currentPath, resolve });
      this.tryOpenNext();
    });
  }

  resolveActive(result: BatchRenameItem[] | null): void {
    const active = this.activeRequest$.value;
    if (!active) return;
    active.resolve(result);
    this.activeRequest$.next(null);
    this.tryOpenNext();
  }

  private tryOpenNext(): void {
    if (this.activeRequest$.value || this.queue.length === 0) return;
    this.activeRequest$.next(this.queue.shift()!);
  }
}
