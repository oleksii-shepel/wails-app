import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';

import { ApiService } from './api.service';
import type { BatchItem, BatchResult } from '../shared/protocol-enhanced';

export type PowerTransferKind = 'copy' | 'move';
export type PowerTransferStatus =
  | 'queued'
  | 'running'
  | 'completed'
  | 'failed'
  | 'cancelled';

export interface PowerTransferJob {
  readonly id: string;
  readonly kind: PowerTransferKind;
  readonly items: readonly BatchItem[];
  readonly createdAt: number;
  readonly attempt: number;
  readonly status: PowerTransferStatus;
  readonly result?: BatchResult;
  readonly error?: string;
}

@Injectable({ providedIn: 'root' })
export class PowerFileOperationsService {
  private readonly jobs = new Map<string, PowerTransferJob>();
  private paused = false;
  private runningId: string | null = null;

  readonly jobs$ = createBehaviorSubject<readonly PowerTransferJob[]>([]);
  readonly paused$ = createBehaviorSubject<boolean>(false);
  private readonly waiters = new Map<
    string,
    Array<(job: PowerTransferJob) => void>
  >();

  constructor(private readonly api: ApiService) {}

  get isPaused(): boolean {
    return this.paused;
  }

  get pendingCount(): number {
    let count = 0;
    for (const job of this.jobs.values()) {
      if (job.status === 'queued') count += 1;
    }
    return count;
  }

  snapshot(): readonly PowerTransferJob[] {
    return [...this.jobs.values()]
      .sort((a, b) => a.createdAt - b.createdAt)
      .map(job => ({
        ...job,
        items: job.items.map(item => ({ ...item })),
      }));
  }

  enqueue(
    kind: PowerTransferKind,
    items: readonly BatchItem[],
  ): string {
    if (!items.length) throw new Error('Transfer queue requires at least one item.');

    const id =
      globalThis.crypto?.randomUUID?.() ??
      `transfer-${Date.now()}-${Math.random().toString(16).slice(2)}`;

    this.jobs.set(id, {
      id,
      kind,
      items: items.map(item => ({ ...item })),
      createdAt: Date.now(),
      attempt: 1,
      status: 'queued',
    });

    this.emit();
    void this.pump();
    return id;
  }

  waitFor(id: string): Promise<PowerTransferJob> {
    const current = this.jobs.get(id);
    if (
      current &&
      (
        current.status === 'completed' ||
        current.status === 'failed' ||
        current.status === 'cancelled'
      )
    ) {
      return Promise.resolve({ ...current, items: current.items.map(item => ({ ...item })) });
    }

    return new Promise(resolve => {
      const waiters = this.waiters.get(id) ?? [];
      waiters.push(resolve);
      this.waiters.set(id, waiters);
    });
  }

  pauseQueue(): void {
    this.paused = true;
    this.paused$.next(true);
  }

  resumeQueue(): void {
    if (!this.paused) return;
    this.paused = false;
    this.paused$.next(false);
    void this.pump();
  }

  cancelQueued(id: string): boolean {
    const job = this.jobs.get(id);
    if (!job || job.status !== 'queued') return false;
    const cancelled = { ...job, status: 'cancelled' as const };
    this.jobs.set(id, cancelled);
    this.settle(cancelled);
    this.emit();
    return true;
  }

  retry(id: string): boolean {
    const job = this.jobs.get(id);
    if (!job || (job.status !== 'failed' && job.status !== 'cancelled')) {
      return false;
    }

    this.jobs.set(id, {
      ...job,
      status: 'queued',
      attempt: job.attempt + 1,
      result: undefined,
      error: undefined,
    });
    this.emit();
    void this.pump();
    return true;
  }

  dismiss(id: string): boolean {
    const job = this.jobs.get(id);
    if (!job || job.status === 'running' || job.status === 'queued') {
      return false;
    }
    this.jobs.delete(id);
    this.emit();
    return true;
  }

  private async pump(): Promise<void> {
    if (this.paused || this.runningId) return;

    const next = [...this.jobs.values()]
      .sort((a, b) => a.createdAt - b.createdAt)
      .find(job => job.status === 'queued');

    if (!next) return;

    this.runningId = next.id;
    this.jobs.set(next.id, { ...next, status: 'running' });
    this.emit();

    try {
      const result =
        next.kind === 'copy'
          ? await this.api.batchCopy([...next.items], true)
          : await this.api.batchMove([...next.items], true);

      const failed = result.failed > 0 || result.skipped > 0;
      const settled: PowerTransferJob = {
        ...next,
        status: failed ? 'failed' : 'completed',
        result,
        error: failed
          ? `${result.failed} failed · ${result.skipped} skipped`
          : undefined,
      };
      this.jobs.set(next.id, settled);
      this.settle(settled);
    } catch (error) {
      const settled: PowerTransferJob = {
        ...next,
        status: 'failed',
        error: error instanceof Error ? error.message : String(error),
      };
      this.jobs.set(next.id, settled);
      this.settle(settled);
    } finally {
      this.runningId = null;
      this.emit();
      if (!this.paused) queueMicrotask(() => void this.pump());
    }
  }

  private settle(job: PowerTransferJob): void {
    const waiters = this.waiters.get(job.id);
    if (!waiters?.length) return;
    this.waiters.delete(job.id);
    const snapshot = {
      ...job,
      items: job.items.map(item => ({ ...item })),
    };
    for (const resolve of waiters) resolve(snapshot);
  }

  private emit(): void {
    this.jobs$.next(this.snapshot());
  }
}
