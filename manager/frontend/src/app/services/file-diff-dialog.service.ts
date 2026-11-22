import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import type { FileInfo } from '../shared/protocol';

export interface FileDiffDialogRequest {
  readonly leftPath?: string;
  readonly rightPath?: string;
  readonly candidates?: readonly FileInfo[];
}

@Injectable({ providedIn: 'root' })
export class FileDiffDialogService {
  readonly activeRequest$ = createBehaviorSubject<FileDiffDialogRequest | null>(null);
  readonly closingRequest$ = createBehaviorSubject<FileDiffDialogRequest | null>(null);
  readonly mountedRequest$ = createBehaviorSubject<FileDiffDialogRequest | null>(null);

  open(request: FileDiffDialogRequest = {}): void {
    this.closingRequest$.next(null);
    this.activeRequest$.next(request);
    this.mountedRequest$.next(request);
  }

  close(): void {
    const current = this.activeRequest$.value;
    if (!current) return;
    this.closingRequest$.next(current);
    this.activeRequest$.next(null);
  }

  finishClose(): void {
    if (this.activeRequest$.value) return;
    this.closingRequest$.next(null);
    this.mountedRequest$.next(null);
  }
}
