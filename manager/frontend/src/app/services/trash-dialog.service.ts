import { Injectable } from '@angular/core';
import { Subject } from 'rxjs';
import { QueuedDialogGate } from './dialog-gate';

export interface TrashDialogRequest {
  resolve(value: void): void;
}

@Injectable({ providedIn: 'root' })
export class TrashDialogService {
  private readonly gate = new QueuedDialogGate<TrashDialogRequest, void>();
  readonly activeRequest$ = this.gate.activeRequest$;
  readonly mountedRequest$ = this.gate.mountedRequest$;
  readonly closingRequest$ = this.gate.closingRequest$;
  readonly mutated$ = new Subject<void>();

  open(): Promise<void> {
    return new Promise<void>((resolve) => {
      this.gate.enqueue({ resolve });
    });
  }

  close(): void {
    this.gate.beginClose(undefined);
  }

  finishClose(): void {
    this.gate.finishClose();
  }

  notifyMutated(): void {
    this.mutated$.next();
  }
}
