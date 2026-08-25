import { createBehaviorSubject } from '@epikodelabs/streamix';

export interface QueuedDialogRequest<TResult> {
  resolve(result: TResult): void;
}

export class QueuedDialogGate<TRequest extends QueuedDialogRequest<TResult>, TResult> {
  readonly activeRequest$ = createBehaviorSubject<TRequest | null>(null);
  readonly mountedRequest$ = createBehaviorSubject<TRequest | null>(null);
  readonly closingRequest$ = createBehaviorSubject<TRequest | null>(null);

  private readonly queue: TRequest[] = [];
  private pendingClose: { request: TRequest; result: TResult } | null = null;

  get activeRequest(): TRequest | null {
    return this.activeRequest$.value;
  }

  get queuedRequests(): readonly TRequest[] {
    return this.queue;
  }

  enqueue(request: TRequest): void {
    this.queue.push(request);
    this.tryOpenNext();
  }

  beginClose(result: TResult): void {
    const active = this.activeRequest$.value;
    if (!active || this.pendingClose) {
      return;
    }

    this.pendingClose = { request: active, result };
    this.closingRequest$.next(active);
    this.activeRequest$.next(null);
  }

  finishClose(): void {
    if (!this.pendingClose) {
      return;
    }

    const { request, result } = this.pendingClose;
    this.pendingClose = null;
    request.resolve(result);
    this.closingRequest$.next(null);
    this.mountedRequest$.next(null);
    this.tryOpenNext();
  }

  resolveActive(result: TResult): void {
    this.beginClose(result);
    this.finishClose();
  }

  protected tryOpenNext(): void {
    if (this.activeRequest$.value || this.pendingClose) {
      return;
    }

    const next = this.queue.shift() ?? null;
    this.activeRequest$.next(next);
    this.mountedRequest$.next(next);
  }
}
