import { EventsOn } from 'wailsjs/runtime';
import type { FileInfo } from '../../shared/protocol';
import {
  ApiService,
  type DirectoryStreamOptions,
  type DirectoryStreamWindow,
} from '../../services/api.service';

export interface DirStreamMetaEvent {
  streamId: string;
  paneId: string;
  path: string;
  showHidden: boolean;
  /** Final logical extent when known at stream start; -1 means unknown. */
  sourceCount: number;
  /** Initial visible extent when known; -1 means indexing determines it. */
  totalCount: number;
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
  sourceCount: number;
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
  onDone: (event: DirStreamDoneEvent) => void;
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
  private pendingBatchEvent: DirStreamBatchEvent | null = null;
  private flushHandle: ReturnType<typeof setTimeout> | null = null;

  private waiters = new Map<string, { resolve: () => void; reject: (err: Error) => void }>();
  private terminals = new Map<string, Error | null>();
  private eventDisposers: Array<() => void> = [];

  constructor(private api: ApiService) {}

  bind(paneId: 'left' | 'right', handlers: DirListStreamHandlers): void {
    this.paneId = paneId;
    this.handlers = handlers;

    this.eventDisposers.forEach(dispose => dispose());
    this.eventDisposers = [
      EventsOn('fs:listStream:meta', (event: DirStreamMetaEvent | null) => {
        this.handleMeta(event);
      }),
      EventsOn('fs:listStream:batch', (event: DirStreamBatchEvent | null) => {
        this.handleBatch(event);
      }),
      EventsOn('fs:listStream:done', (event: DirStreamDoneEvent | null) => {
        this.handleDone(event);
      }),
      EventsOn('fs:listStream:error', (event: DirStreamErrorEvent | null) => {
        this.handleError(event);
      }),
    ];
  }

  async unbind(): Promise<void> {
    await this.cancelActive();
    this.rejectAllWaiters(new Error('Pane disposed'));
    this.handlers = null;
    this.eventDisposers.forEach(dispose => dispose());
    this.eventDisposers = [];
  }

  getActiveStreamId(): string | null {
    return this.activeStreamId;
  }

  async readWindow(streamId: string, offset: number, limit: number): Promise<DirectoryStreamWindow> {
    return await this.api.readListDirectoryStreamWindow(streamId, offset, limit);
  }

  async start(path: string, showHidden: boolean, options?: DirectoryStreamOptions): Promise<string> {
    await this.cancelActive();
    this.baseEntries = [];
    this.metaSeen = false;
    this.terminals.clear();

    const streamId = await this.api.startListDirectoryStream(this.paneId, path, showHidden, options);
    this.activeStreamId = streamId;
    return streamId;
  }

  waitFor(streamId: string): Promise<void> {
    if (this.terminals.has(streamId)) {
      const terminal = this.terminals.get(streamId) ?? null;
      this.terminals.delete(streamId);
      return terminal ? Promise.reject(terminal) : Promise.resolve();
    }

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
    this.pendingBatchEvent = null;
    this.cancelScheduledFlush();

    try {
      await this.api.cancelListDirectoryStream(id);
    } catch {
      // ignore
    }
    this.rejectWaiter(id, new Error('Directory stream cancelled'));
  }

  private resolveWaiter(streamId: string): void {
    const waiter = this.waiters.get(streamId);
    if (!waiter) {
      this.terminals.set(streamId, null);
      return;
    }
    this.waiters.delete(streamId);
    waiter.resolve();
  }

  private rejectWaiter(streamId: string, err: Error): void {
    const waiter = this.waiters.get(streamId);
    if (!waiter) {
      this.terminals.set(streamId, err);
      return;
    }
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

    const incoming = event.entries || [];
    if (incoming.length > 0) {
      this.baseEntries.push(...incoming);
    }

    // Wails may deliver many 256-entry batches in one renderer turn. Collapse
    // them into at most one Angular-facing update per animation frame. The
    // accumulation array is intentionally stable; consumers must treat it as
    // read-only.
    this.pendingBatchEvent = event;
    this.scheduleFlush();
  }

  private scheduleFlush(): void {
    if (this.flushHandle !== null) return;

    this.flushHandle = setTimeout(() => {
      this.flushHandle = null;
      this.flushPendingBatch();
    }, 0);
  }

  private flushPendingBatch(): void {
    const event = this.pendingBatchEvent;
    const handlers = this.handlers;
    if (!event || !handlers) return;

    this.pendingBatchEvent = null;
    handlers.onBatch(event, this.baseEntries);
  }

  private cancelScheduledFlush(): void {
    if (this.flushHandle === null) return;
    clearTimeout(this.flushHandle);
    this.flushHandle = null;
  }

  private handleDone(event: DirStreamDoneEvent | null): void {
    if (!event || event.paneId !== this.paneId) return;
    if (!this.activeStreamId || event.streamId !== this.activeStreamId) return;
    if (!this.handlers) return;

    this.flushPendingBatch();
    this.handlers.onDone(event);
    this.resolveWaiter(event.streamId);

    // Reset internal state after completion so stale events don't apply.
    this.activeStreamId = null;
    this.metaSeen = false;
    this.pendingBatchEvent = null;
    this.cancelScheduledFlush();
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
    this.pendingBatchEvent = null;
    this.cancelScheduledFlush();
    this.baseEntries = [];
  }
}