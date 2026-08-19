import { Injectable } from '@angular/core';

export type WorkPriority =
  | 'visible'
  | 'interactive'
  | 'prefetch'
  | 'background'
  | 'idle';

export type WorkKind =
  | 'directory'
  | 'search'
  | 'thumbnail'
  | 'metadata'
  | 'hashing'
  | 'provider'
  | 'viewer'
  | 'default';

export interface WorkProgress {
  readonly completed?: number;
  readonly total?: number;
  readonly message?: string;
  readonly units?: string;
}

export interface WorkSnapshot {
  readonly id: number;
  readonly key: string | null;
  readonly scope: string | null;
  readonly generation: number | null;
  readonly priority: WorkPriority;
  readonly kind: WorkKind;
  readonly state: 'queued' | 'running' | 'completed' | 'cancelled' | 'failed';
  readonly createdAt: number;
  readonly startedAt?: number;
  readonly finishedAt?: number;
  readonly progress?: WorkProgress;
}

export interface WorkContext {
  readonly signal: AbortSignal;
  report(progress: WorkProgress): void;
  yield(priority?: WorkPriority): Promise<void>;
}

export interface WorkRequest<T> {
  readonly key?: string;
  readonly scope?: string;
  readonly generation?: number;
  readonly priority: WorkPriority;
  readonly kind?: WorkKind;
  readonly run: (context: WorkContext) => Promise<T> | T;
}

export interface WorkHandle<T> {
  readonly id: number;
  readonly promise: Promise<T>;
  cancel(): void;
  snapshot(): WorkSnapshot;
  onProgress(listener: (progress: WorkProgress) => void): () => void;
}

interface ScopeState {
  generation: number;
  controller: AbortController;
}

interface InternalWork<T = unknown> {
  id: number;
  key: string | null;
  scope: string | null;
  generation: number | null;
  priority: WorkPriority;
  kind: WorkKind;
  sequence: number;
  createdAt: number;
  startedAt?: number;
  finishedAt?: number;
  state: WorkSnapshot['state'];
  progress?: WorkProgress;
  controller: AbortController;
  inheritedAbortCleanup?: () => void;
  run: (context: WorkContext) => Promise<T> | T;
  resolve: (value: T) => void;
  reject: (error: unknown) => void;
  promise: Promise<T>;
  progressListeners: Set<(progress: WorkProgress) => void>;
}

const PRIORITY_ORDER: Record<WorkPriority, number> = {
  visible: 0,
  interactive: 1,
  prefetch: 2,
  background: 3,
  idle: 4,
};

const DEFAULT_KIND_LIMITS: Readonly<Record<WorkKind, number>> = {
  directory: 2,
  search: 2,
  thumbnail: 4,
  metadata: 4,
  hashing: 2,
  provider: 4,
  viewer: 3,
  default: 4,
};

@Injectable({ providedIn: 'root' })
export class WorkSchedulerService {
  private readonly queue: InternalWork<any>[] = [];
  private readonly activeByKind = new Map<WorkKind, number>();
  private readonly keyed = new Map<string, InternalWork<any>>();
  private readonly scopes = new Map<string, ScopeState>();
  private readonly snapshots = new Map<number, WorkSnapshot>();
  private readonly listeners = new Set<(snapshot: WorkSnapshot) => void>();

  private active = 0;
  private nextId = 1;
  private sequence = 0;

  private readonly maxConcurrency = 8;
  private readonly kindLimits = DEFAULT_KIND_LIMITS;

  beginGeneration(scope: string): number {
    const previous = this.scopes.get(scope);
    previous?.controller.abort();

    const generation = (previous?.generation ?? 0) + 1;
    const controller = new AbortController();
    this.scopes.set(scope, { generation, controller });

    this.cancelQueued(work =>
      work.scope === scope &&
      work.generation !== null &&
      work.generation !== generation
    );
    return generation;
  }

  cancelScope(scope: string): void {
    const state = this.scopes.get(scope);
    state?.controller.abort();
    this.scopes.delete(scope);

    this.cancelQueued(work => work.scope === scope);
  }

  isCurrent(scope: string, generation: number): boolean {
    return this.scopes.get(scope)?.generation === generation;
  }

  schedule<T>(request: WorkRequest<T>): WorkHandle<T> {
    const kind = request.kind ?? 'default';
    const scope = request.scope ?? null;
    const generation = request.generation ?? null;

    if (
      scope &&
      generation !== null &&
      !this.isCurrent(scope, generation)
    ) {
      return this.cancelledHandle<T>(
        request.key ?? null,
        scope,
        generation,
        request.priority,
        kind,
      );
    }

    const dedupKey = request.key?.trim() || null;
    if (dedupKey) {
      const existing = this.keyed.get(dedupKey);
      if (
        existing &&
        (existing.state === 'queued' || existing.state === 'running')
      ) {
        return this.handleFor(existing as InternalWork<T>);
      }
    }

    let resolve!: (value: T) => void;
    let reject!: (error: unknown) => void;
    const promise = new Promise<T>((res, rej) => {
      resolve = res;
      reject = rej;
    });

    const controller = new AbortController();
    let inheritedAbortCleanup: (() => void) | undefined;

    if (scope && generation !== null) {
      const scopeState = this.scopes.get(scope);
      if (scopeState) {
        const forwardAbort = () => controller.abort(scopeState.controller.signal.reason);
        if (scopeState.controller.signal.aborted) {
          controller.abort(scopeState.controller.signal.reason);
        } else {
          scopeState.controller.signal.addEventListener('abort', forwardAbort, { once: true });
          inheritedAbortCleanup = () =>
            scopeState.controller.signal.removeEventListener('abort', forwardAbort);
        }
      }
    }

    const work: InternalWork<T> = {
      id: this.nextId++,
      key: dedupKey,
      scope,
      generation,
      priority: request.priority,
      kind,
      sequence: this.sequence++,
      createdAt: performance.now(),
      state: 'queued',
      controller,
      inheritedAbortCleanup,
      run: request.run,
      resolve,
      reject,
      promise,
      progressListeners: new Set(),
    };

    if (dedupKey) this.keyed.set(dedupKey, work);
    this.queue.push(work as InternalWork<any>);
    this.sortQueue();
    this.publish(work);
    this.pump();

    return this.handleFor(work);
  }

  async yield(
    priority: WorkPriority = 'interactive',
    signal?: AbortSignal,
  ): Promise<void> {
    if (signal?.aborted) throw this.abortError();

    if (priority === 'idle' && typeof window !== 'undefined') {
      await new Promise<void>((resolve, reject) => {
        let settled = false;
        const finish = () => {
          if (settled) return;
          settled = true;
          cleanup();
          resolve();
        };
        const abort = () => {
          if (settled) return;
          settled = true;
          cleanup();
          reject(this.abortError());
        };

        const ric = (window as Window & {
          requestIdleCallback?: (
            callback: IdleRequestCallback,
            options?: IdleRequestOptions,
          ) => number;
          cancelIdleCallback?: (handle: number) => void;
        }).requestIdleCallback;

        const cic = (window as Window & {
          cancelIdleCallback?: (handle: number) => void;
        }).cancelIdleCallback;

        const useIdleCallback = typeof ric === 'function';
        const handle = useIdleCallback
          ? ric!(() => finish(), { timeout: 250 })
          : window.setTimeout(finish, 32);

        const cleanup = () => {
          signal?.removeEventListener('abort', abort);
          if (useIdleCallback) {
            cic?.(handle);
          } else {
            clearTimeout(handle);
          }
        };

        signal?.addEventListener('abort', abort, { once: true });
      });
      return;
    }

    await new Promise<void>((resolve, reject) => {
      const abort = () => {
        cleanup();
        reject(this.abortError());
      };
      const handle = setTimeout(() => {
        cleanup();
        resolve();
      }, priority === 'background' ? 8 : 0);
      const cleanup = () => {
        clearTimeout(handle);
        signal?.removeEventListener('abort', abort);
      };
      signal?.addEventListener('abort', abort, { once: true });
    });
  }

  subscribe(listener: (snapshot: WorkSnapshot) => void): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  currentSnapshots(): readonly WorkSnapshot[] {
    return [...this.snapshots.values()]
      .sort((a, b) => a.id - b.id);
  }

  stats(kind?: WorkKind): {
    readonly queued: number;
    readonly running: number;
    readonly completed: number;
    readonly failed: number;
    readonly cancelled: number;
  } {
    const snapshots = [...this.snapshots.values()]
      .filter(snapshot => !kind || snapshot.kind === kind);

    const count = (state: WorkSnapshot['state']) =>
      snapshots.filter(snapshot => snapshot.state === state).length;

    return {
      queued: count('queued'),
      running: count('running'),
      completed: count('completed'),
      failed: count('failed'),
      cancelled: count('cancelled'),
    };
  }

  private pump(): void {
    while (this.active < this.maxConcurrency) {
      const index = this.nextRunnableIndex();
      if (index < 0) return;

      const work = this.queue.splice(index, 1)[0];
      if (work.controller.signal.aborted) {
        this.finishCancelled(work);
        continue;
      }

      this.active += 1;
      this.activeByKind.set(
        work.kind,
        (this.activeByKind.get(work.kind) ?? 0) + 1,
      );
      work.state = 'running';
      work.startedAt = performance.now();
      this.publish(work);

      void this.run(work);
    }
  }

  private async run(work: InternalWork<any>): Promise<void> {
    const context: WorkContext = {
      signal: work.controller.signal,
      report: progress => {
        if (work.controller.signal.aborted) return;
        work.progress = { ...progress };
        for (const listener of work.progressListeners) listener(work.progress);
        this.publish(work);
      },
      yield: priority => this.yield(priority ?? work.priority, work.controller.signal),
    };

    try {
      const result = await work.run(context);
      if (work.controller.signal.aborted) {
        this.finishCancelled(work);
      } else {
        work.state = 'completed';
        work.finishedAt = performance.now();
        work.resolve(result);
        this.finish(work);
      }
    } catch (error) {
      if (
        work.controller.signal.aborted ||
        (error instanceof DOMException && error.name === 'AbortError')
      ) {
        this.finishCancelled(work);
      } else {
        work.state = 'failed';
        work.finishedAt = performance.now();
        work.reject(error);
        this.finish(work);
      }
    }
  }

  private finishCancelled(work: InternalWork<any>): void {
    work.state = 'cancelled';
    work.finishedAt = performance.now();
    work.reject(this.abortError());
    this.finish(work);
  }

  private finish(work: InternalWork<any>): void {
    work.inheritedAbortCleanup?.();
    if (work.key && this.keyed.get(work.key) === work) {
      this.keyed.delete(work.key);
    }

    if (work.startedAt !== undefined) {
      this.active = Math.max(0, this.active - 1);
      this.activeByKind.set(
        work.kind,
        Math.max(0, (this.activeByKind.get(work.kind) ?? 1) - 1),
      );
    }

    this.publish(work);
    this.pruneSnapshots();
    this.pump();
  }

  private nextRunnableIndex(): number {
    for (let index = 0; index < this.queue.length; index += 1) {
      const work = this.queue[index];
      const limit = this.kindLimits[work.kind];
      if ((this.activeByKind.get(work.kind) ?? 0) < limit) {
        return index;
      }
    }
    return -1;
  }

  private sortQueue(): void {
    this.queue.sort(
      (left, right) =>
        PRIORITY_ORDER[left.priority] - PRIORITY_ORDER[right.priority] ||
        left.sequence - right.sequence,
    );
  }

  private cancelQueued(predicate: (work: InternalWork<any>) => boolean): void {
    for (let index = this.queue.length - 1; index >= 0; index -= 1) {
      const work = this.queue[index];
      if (!predicate(work)) continue;
      this.queue.splice(index, 1);
      work.controller.abort();
      this.finishCancelled(work);
    }
  }

  private handleFor<T>(work: InternalWork<T>): WorkHandle<T> {
    return {
      id: work.id,
      promise: work.promise,
      cancel: () => {
        work.controller.abort();
        if (work.state === 'queued') {
          const index = this.queue.indexOf(work as InternalWork<any>);
          if (index >= 0) this.queue.splice(index, 1);
          this.finishCancelled(work as InternalWork<any>);
        }
      },
      snapshot: () => this.snapshotOf(work),
      onProgress: listener => {
        work.progressListeners.add(listener);
        if (work.progress) listener(work.progress);
        return () => work.progressListeners.delete(listener);
      },
    };
  }

  private cancelledHandle<T>(
    key: string | null,
    scope: string,
    generation: number,
    priority: WorkPriority,
    kind: WorkKind,
  ): WorkHandle<T> {
    const id = this.nextId++;
    const snapshot: WorkSnapshot = {
      id,
      key,
      scope,
      generation,
      priority,
      kind,
      state: 'cancelled',
      createdAt: performance.now(),
      finishedAt: performance.now(),
    };
    this.snapshots.set(id, snapshot);
    const promise = Promise.reject<T>(this.abortError());
    // Avoid an unhandled rejection if the caller only examines the handle.
    void promise.catch(() => {});
    return {
      id,
      promise,
      cancel() {},
      snapshot: () => snapshot,
      onProgress: () => () => {},
    };
  }

  private snapshotOf(work: InternalWork<any>): WorkSnapshot {
    return {
      id: work.id,
      key: work.key,
      scope: work.scope,
      generation: work.generation,
      priority: work.priority,
      kind: work.kind,
      state: work.state,
      createdAt: work.createdAt,
      startedAt: work.startedAt,
      finishedAt: work.finishedAt,
      progress: work.progress,
    };
  }

  private publish(work: InternalWork<any>): void {
    const snapshot = this.snapshotOf(work);
    this.snapshots.set(work.id, snapshot);
    for (const listener of this.listeners) listener(snapshot);
  }

  private pruneSnapshots(): void {
    const terminal = [...this.snapshots.values()]
      .filter(snapshot =>
        snapshot.state === 'completed' ||
        snapshot.state === 'cancelled' ||
        snapshot.state === 'failed'
      )
      .sort((a, b) => (a.finishedAt ?? 0) - (b.finishedAt ?? 0));

    while (terminal.length > 256) {
      const oldest = terminal.shift();
      if (oldest) this.snapshots.delete(oldest.id);
    }
  }

  private abortError(): DOMException {
    return new DOMException('Work cancelled.', 'AbortError');
  }
}