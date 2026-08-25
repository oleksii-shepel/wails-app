import { Injectable } from '@angular/core';
import type { FileInfo } from '../shared/protocol';
import { QueuedDialogGate } from './dialog-gate';

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
  resolve(value: BatchRenameItem[] | null): void;
}

@Injectable({ providedIn: 'root' })
export class BatchRenameDialogService {
  private readonly gate = new QueuedDialogGate<BatchRenameRequest, BatchRenameItem[] | null>();
  public readonly activeRequest$ = this.gate.activeRequest$;
  public readonly mountedRequest$ = this.gate.mountedRequest$;
  public readonly closingRequest$ = this.gate.closingRequest$;

  openDialog(items: BatchRenameItem[], currentPath: string): Promise<BatchRenameItem[] | null> {
    return new Promise<BatchRenameItem[] | null>((resolve) => {
      this.gate.enqueue({
        items,
        currentPath,
        resolve,
      });
    });
  }

  resolveActive(result: BatchRenameItem[] | null): void {
    this.gate.beginClose(result);
  }

  finishClose(): void {
    this.gate.finishClose();
  }
}
