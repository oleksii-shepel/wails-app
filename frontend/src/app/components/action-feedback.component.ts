import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  OnDestroy,
  OnInit,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

import {
  ActionFeedbackService,
  type FeedbackItem,
} from '../services/action-feedback.service';

@Component({
  selector: 'app-action-feedback',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section
      class="feedback-stack"
      *ngIf="items.length"
      aria-label="File operation notifications"
      aria-live="polite"
      aria-relevant="additions text">
      <article
        *ngFor="let item of items; trackBy: track"
        class="feedback"
        [class.success]="item.kind === 'success'"
        [class.warning]="item.kind === 'warning'"
        [class.error]="item.kind === 'error'"
        [class.info]="item.kind === 'info'">
        <span class="indicator" aria-hidden="true"></span>
        <div class="copy">
          <strong>{{ item.title }}</strong>
          <span *ngIf="item.message">{{ item.message }}</span>
        </div>
        <button
          type="button"
          title="Dismiss"
          aria-label="Dismiss notification"
          (click)="feedback.dismiss(item.id)">×</button>
      </article>
    </section>
  `,
  styles: [`
    :host{
      position:fixed;
      right:10px;
      bottom:10px;
      z-index:8450;
      pointer-events:none;
    }
    .feedback-stack{
      display:flex;
      width:min(390px,calc(100vw - 20px));
      flex-direction:column;
      gap:6px;
      pointer-events:auto;
    }
    .feedback{
      display:grid;
      grid-template-columns:3px minmax(0,1fr) auto;
      align-items:stretch;
      overflow:hidden;
      border:1px solid var(--vsc-border,var(--vsc-border));
      border-radius:6px;
      background:var(--vsc-popover-background,var(--vsc-dialog-background));
      color:var(--vsc-dialog-foreground,var(--vsc-foreground));
      box-shadow:0 6px 22px var(--vsc-scrim-soft);
      font:11px/1.35 var(--vsc-font-family,system-ui,sans-serif);
    }
    .indicator{background:var(--vsc-info,var(--vsc-accent-blue))}
    .success .indicator{background:var(--vsc-success)}
    .warning .indicator{background:var(--vsc-warning)}
    .error .indicator{background:var(--vsc-error)}
    .copy{
      display:grid;
      min-width:0;
      gap:2px;
      padding:7px 8px;
    }
    .copy strong,.copy span{
      overflow:hidden;
      text-overflow:ellipsis;
      white-space:nowrap;
    }
    .copy span{color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    button{
      width:28px;
      border:0;
      background:transparent;
      color:inherit;
      cursor:pointer;
      font:16px/1 system-ui,sans-serif;
    }
    button:hover{background:var(--vsc-list-hover-background)}
  `],
})
export class ActionFeedbackComponent implements OnInit, OnDestroy {
  items: readonly FeedbackItem[] = [];
  private readonly destroy$ = createSubject<void>();

  constructor(
    public readonly feedback: ActionFeedbackService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    this.feedback.items$
      .pipe(takeUntil(this.destroy$))
      .subscribe(items => {
        this.items = items;
        this.cdr.markForCheck();
      });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  track(_index: number, item: FeedbackItem): string {
    return item.id;
  }
}
