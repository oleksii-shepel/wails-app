import { Injectable } from '@angular/core';
import { createSubject } from '@epikodelabs/streamix';

import type { ArchiveFrame } from './archive.service';

export type FileClipboardOperation = 'copy' | 'cut';

export interface FileClipboardPayload {
  operation: FileClipboardOperation;
  paths: string[];
  sourcePaneId: 'left' | 'right';
  archiveFrame: ArchiveFrame | null;
  createdAt: number;
}

@Injectable({ providedIn: 'root' })
export class FileClipboardService {
  private readonly stateSubject = createSubject<FileClipboardPayload | null>();

  payload: FileClipboardPayload | null = null;
  readonly state$ = this.stateSubject;

  set(payload: Omit<FileClipboardPayload, 'createdAt'>): void {
    this.payload = {
      ...payload,
      paths: [...payload.paths],
      archiveFrame: payload.archiveFrame
        ? {
            ...payload.archiveFrame,
            nestedChain: [...(payload.archiveFrame.nestedChain ?? [])],
          }
        : null,
      createdAt: Date.now(),
    };
    this.stateSubject.next(this.payload);
  }

  clear(): void {
    this.payload = null;
    this.stateSubject.next(null);
  }
}
