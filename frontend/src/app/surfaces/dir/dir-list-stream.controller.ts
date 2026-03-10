import { EventsOff, EventsOn } from 'wailsjs/runtime';
import type { FileInfo } from '../../shared/protocol';
import { ApiService } from '../../services/api.service';

export interface DirStreamMetaEvent {
  streamId: string;
  paneId: string;
  path: string;
  showHidden: boolean;
}

export interface DirStreamBatchEvent {
  streamId: string;
  paneId: string;
  path: string;
  entries: FileInfo[];
}

export interface DirStreamDoneEvent {
  streamId: string;
  paneId: string;
  path: string;
  entryCount: number;
  totalSize: number;
  durationMs: number;
}

export interface DirStreamErrorEvent {
  streamId: string;
  paneId: string;
  path: string;
  error: string;
}

export interface DirListStreamHandlers {
  onMeta: (event: DirStreamMetaEvent, isFirstMeta: boolean) => void;
  onBatch: (event: DirStreamBatchEvent, baseEntries: FileInfo[]) => void;
  onDone: (event: DirStreamDoneEvent, baseEntries: FileInfo[]) => void;
  onError: (event: DirStreamErrorEvent) => void;
}

/**
 * Pane-scoped controller that owns the Wails list-dir stream lifecycle:
 * start/cancel/wait + event filtering + base-entry accumulation.
 *
 * FilePaneComponent stays responsible for converting base entries into
 * view-model entries (parent row, filtering, cursor restore, etc).
 */
export class DirListStreamController {
  private paneId: 'left' | 'right' = 'left';
  private handlers: DirListStreamHandlers | null = null;

  private activeStreamId: string | null = null;
  private metaSeen = false;
  private baseEntries: FileInfo[] = [];

  private waiters = new Map<string, { resolve: () => void; reject: (err: Error) => void }>();

  constructor(private api: ApiService) {}

  bind(paneId: 'left' | 'right', handlers: DirListStreamHandlers): void {
    this.paneId = paneId;
    this.handlers = handlers;

    EventsOn('fs:listStream:meta', (event: DirStreamMetaEvent | null) => {
      this.handleMeta(event);
    });
    EventsOn('fs:listStream:batch', (event: DirStreamBatchEvent | null) => {
      this.handleBatch(event);
    });
    EventsOn('fs:listStream:done', (event: DirStreamDoneEvent | null) => {
      this.handleDone(event);
    });
    EventsOn('fs:listStream:error', (event: DirStreamErrorEvent | null) => {
      this.handleError(event);
    });
  }

  async unbind(): Promise<void> {
    await this.cancelActive();
    this.rejectAllWaiters(new Error('Pane disposed'));
    this.handlers = null;
    EventsOff(
      'fs:listStream:meta',
      'fs:listStream:batch',
      'fs:listStream:done',
      'fs:listStream:error',
    );
  }

  getActiveStreamId(): string | null {
    return this.activeStreamId;
  }

  async start(path: string, showHidden: boolean): Promise<string> {
    await this.cancelActive();
    this.baseEntries = [];
    this.metaSeen = false;

    const streamId = await this.api.startListDirectoryStream(this.paneId, path, showHidden);
    this.activeStreamId = streamId;
    return streamId;
  }

  waitFor(streamId: string): Promise<void> {
    return new Promise<void>((resolve, reject) => {
      this.waiters.set(streamId, { resolve, reject });
    });
  }

  async cancelActive(): Promise<void> {
    const id = this.activeStreamId;
    if (!id) return;

    this.activeStreamId = null;
    this.metaSeen = false;
    this.baseEntries = [];

    try {
      await this.api.cancelListDirectoryStream(id);
    } catch {
      // ignore
    }
    this.rejectWaiter(id, new Error('Directory stream cancelled'));
  }

  private resolveWaiter(streamId: string): void {
    const waiter = this.waiters.get(streamId);
    if (!waiter) return;
    this.waiters.delete(streamId);
    waiter.resolve();
  }

  private rejectWaiter(streamId: string, err: Error): void {
    const waiter = this.waiters.get(streamId);
    if (!waiter) return;
    this.waiters.delete(streamId);
    waiter.reject(err);
  }

  private rejectAllWaiters(err: Error): void {
    for (const [id, waiter] of this.waiters.entries()) {
      this.waiters.delete(id);
      waiter.reject(err);
    }
  }

  private handleMeta(event: DirStreamMetaEvent | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (!this.activeStreamId || event.streamId !== this.activeStreamId) return;
    if (!this.handlers) return;

    const isFirst = !this.metaSeen;
    this.metaSeen = true;
    this.handlers.onMeta(event, isFirst);
  }

  private handleBatch(event: DirStreamBatchEvent | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (!this.activeStreamId || event.streamId !== this.activeStreamId) return;
    if (!this.handlers) return;

    this.baseEntries = this.baseEntries.concat(event.entries || []);
    this.handlers.onBatch(event, this.baseEntries.slice());
  }

  private handleDone(event: DirStreamDoneEvent | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (!this.activeStreamId || event.streamId !== this.activeStreamId) return;
    if (!this.handlers) return;

    const base = this.baseEntries.slice();
    this.handlers.onDone(event, base);
    this.resolveWaiter(event.streamId);

    // Reset internal state after completion so stale events don't apply.
    this.activeStreamId = null;
    this.metaSeen = false;
    this.baseEntries = [];
  }

  private handleError(event: DirStreamErrorEvent | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (!this.activeStreamId || event.streamId !== this.activeStreamId) return;
    if (!this.handlers) return;

    this.handlers.onError(event);
    this.rejectWaiter(event.streamId, new Error(event.error || 'Failed to load directory'));

    this.activeStreamId = null;
    this.metaSeen = false;
    this.baseEntries = [];
  }
}
