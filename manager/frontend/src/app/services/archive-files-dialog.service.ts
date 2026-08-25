import { Injectable } from '@angular/core';
import { QueuedDialogGate } from './dialog-gate';

export interface ArchiveFilesDialogRequest {
  title: string;
  message: string;
  confirmText: string;
  defaultValue: string;
  resolve(value: string | null): void;
}

@Injectable({ providedIn: 'root' })
export class ArchiveFilesDialogService {
  private readonly gate = new QueuedDialogGate<ArchiveFilesDialogRequest, string | null>();
  readonly activeRequest$ = this.gate.activeRequest$;
  readonly mountedRequest$ = this.gate.mountedRequest$;
  readonly closingRequest$ = this.gate.closingRequest$;

  open(request: Omit<ArchiveFilesDialogRequest, 'resolve'>): Promise<string | null> {
    return new Promise<string | null>((resolve) => {
      this.gate.enqueue({
        ...request,
        resolve,
      });
    });
  }

  submit(value: string): void {
    this.gate.beginClose(value);
  }

  close(): void {
    this.gate.beginClose(null);
  }

  finishClose(): void {
    this.gate.finishClose();
  }
}
