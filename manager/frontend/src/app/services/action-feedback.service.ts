import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';

export type FeedbackKind = 'success' | 'warning' | 'error' | 'info';

export interface FeedbackItem {
  readonly id: string;
  readonly kind: FeedbackKind;
  readonly title: string;
  readonly message?: string;
  readonly createdAt: number;
  readonly persistent: boolean;
}

@Injectable({ providedIn: 'root' })
export class ActionFeedbackService {
  private readonly itemsSubject =
    createBehaviorSubject<readonly FeedbackItem[]>([]);
  readonly items$ = this.itemsSubject;

  private readonly timers = new Map<string, ReturnType<typeof setTimeout>>();
  private sequence = 0;

  show(
    kind: FeedbackKind,
    title: string,
    message?: string,
    options: { persistent?: boolean; timeoutMs?: number } = {},
  ): string {
    const id = `feedback-${Date.now()}-${++this.sequence}`;
    const persistent = options.persistent ?? kind === 'error';
    const item: FeedbackItem = {
      id,
      kind,
      title,
      message,
      createdAt: Date.now(),
      persistent,
    };

    this.itemsSubject.next([...this.itemsSubject.value, item].slice(-6));

    if (!persistent) {
      const timeoutMs = Math.max(1800, options.timeoutMs ?? 4200);
      const timer = setTimeout(() => this.dismiss(id), timeoutMs);
      this.timers.set(id, timer);
    }

    return id;
  }

  success(title: string, message?: string): string {
    return this.show('success', title, message);
  }

  warning(title: string, message?: string): string {
    return this.show('warning', title, message, { timeoutMs: 6000 });
  }

  error(title: string, message?: string): string {
    return this.show('error', title, message, { persistent: true });
  }

  dismiss(id: string): void {
    const timer = this.timers.get(id);
    if (timer) clearTimeout(timer);
    this.timers.delete(id);
    this.itemsSubject.next(this.itemsSubject.value.filter(item => item.id !== id));
  }

  clear(): void {
    for (const timer of this.timers.values()) clearTimeout(timer);
    this.timers.clear();
    this.itemsSubject.next([]);
  }
}