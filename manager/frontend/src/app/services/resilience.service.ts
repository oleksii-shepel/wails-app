import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';

export type ResilienceState =
  | 'online'
  | 'degraded'
  | 'offline'
  | 'reconnecting';

export interface RetryPolicy {
  readonly maxAttempts: number;
  readonly initialDelayMs: number;
  readonly maxDelayMs: number;
  readonly multiplier: number;
  readonly jitter: number;
}

export interface SourceHealth {
  readonly key: string;
  readonly state: ResilienceState;
  readonly consecutiveFailures: number;
  readonly lastSuccessAt: number;
  readonly lastFailureAt: number;
  readonly lastError: string;
  readonly nextRetryAt: number;
}

const DEFAULT_RETRY: RetryPolicy = Object.freeze({
  maxAttempts: 3,
  initialDelayMs: 180,
  maxDelayMs: 2500,
  multiplier: 2,
  jitter: 0.2,
});

@Injectable({ providedIn: 'root' })
export class ResilienceService {
  private readonly health = new Map<string, SourceHealth>();

  readonly health$ = createBehaviorSubject<readonly SourceHealth[]>([]);

  snapshot(key: string): SourceHealth {
    return this.health.get(key) ?? {
      key,
      state: this.browserOffline() ? 'offline' : 'online',
      consecutiveFailures: 0,
      lastSuccessAt: 0,
      lastFailureAt: 0,
      lastError: '',
      nextRetryAt: 0,
    };
  }

  async execute<T>(
    key: string,
    operation: (signal: AbortSignal) => Promise<T>,
    options: {
      readonly signal?: AbortSignal;
      readonly policy?: Partial<RetryPolicy>;
      readonly retryable?: (error: unknown) => boolean;
    } = {},
  ): Promise<T> {
    const policy: RetryPolicy = {
      ...DEFAULT_RETRY,
      ...(options.policy ?? {}),
    };
    const retryable = options.retryable ?? (() => true);

    let lastError: unknown;
    for (let attempt = 1; attempt <= Math.max(1, policy.maxAttempts); attempt += 1) {
      this.throwIfAborted(options.signal);

      if (this.browserOffline()) {
        this.markFailure(key, new Error('Offline'), true);
        throw new Error('Offline');
      }

      if (attempt > 1) {
        this.patch(key, { state: 'reconnecting' });
      }

      const controller = new AbortController();
      const forwardAbort = () => controller.abort(options.signal?.reason);
      options.signal?.addEventListener('abort', forwardAbort, { once: true });

      try {
        const value = await operation(controller.signal);
        this.markSuccess(key);
        return value;
      } catch (error) {
        lastError = error;
        if (controller.signal.aborted || options.signal?.aborted) {
          throw error;
        }

        const shouldRetry = retryable(error) && attempt < policy.maxAttempts;
        this.markFailure(key, error, !shouldRetry);

        if (!shouldRetry) throw error;

        const delay = this.retryDelay(policy, attempt);
        this.patch(key, {
          state: 'degraded',
          nextRetryAt: Date.now() + delay,
        });
        await this.delay(delay, options.signal);
      } finally {
        options.signal?.removeEventListener('abort', forwardAbort);
      }
    }

    throw lastError;
  }

  markSuccess(key: string): void {
    this.health.set(key, {
      ...this.snapshot(key),
      state: 'online',
      consecutiveFailures: 0,
      lastSuccessAt: Date.now(),
      lastError: '',
      nextRetryAt: 0,
    });
    this.emit();
  }

  markFailure(key: string, error: unknown, terminal = false): void {
    const previous = this.snapshot(key);
    const failures = previous.consecutiveFailures + 1;
    this.health.set(key, {
      ...previous,
      state: terminal || this.browserOffline() ? 'offline' : 'degraded',
      consecutiveFailures: failures,
      lastFailureAt: Date.now(),
      lastError: error instanceof Error ? error.message : String(error),
    });
    this.emit();
  }

  clear(key: string): void {
    if (!this.health.delete(key)) return;
    this.emit();
  }

  private retryDelay(policy: RetryPolicy, attempt: number): number {
    const base = Math.min(
      policy.maxDelayMs,
      policy.initialDelayMs * Math.pow(policy.multiplier, Math.max(0, attempt - 1)),
    );
    const spread = Math.max(0, Math.min(1, policy.jitter));
    const jitter = 1 + (Math.random() * 2 - 1) * spread;
    return Math.max(0, Math.round(base * jitter));
  }

  private async delay(ms: number, signal?: AbortSignal): Promise<void> {
    if (ms <= 0) return;
    await new Promise<void>((resolve, reject) => {
      const abort = () => {
        clearTimeout(handle);
        reject(new DOMException('Cancelled', 'AbortError'));
      };
      const handle = setTimeout(() => {
        signal?.removeEventListener('abort', abort);
        resolve();
      }, ms);
      signal?.addEventListener('abort', abort, { once: true });
    });
  }

  private throwIfAborted(signal?: AbortSignal): void {
    if (signal?.aborted) {
      throw new DOMException('Cancelled', 'AbortError');
    }
  }

  private browserOffline(): boolean {
    return typeof navigator !== 'undefined' && navigator.onLine === false;
  }

  private patch(key: string, patch: Partial<SourceHealth>): void {
    this.health.set(key, { ...this.snapshot(key), ...patch, key });
    this.emit();
  }

  private emit(): void {
    this.health$.next(
      [...this.health.values()].sort((a, b) => a.key.localeCompare(b.key)),
    );
  }
}