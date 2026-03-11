import { Component, OnDestroy, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import type { Stream } from '@epikodelabs/streamix';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

interface ConfirmDialogRequest {
  options: any;
  resolve: (value: boolean) => void;
}

@Component({
  selector: 'app-confirm-dialog',
  standalone: true,
  imports: [CommonModule],
  template: `
    <ng-container *ngIf="request$ | async as request">
      <div 
        class="confirm-overlay" 
        [class.confirm-overlay--alert]="request.options.isAlert"
        (click)="onCancel()"
      >
        <div
          class="confirm-panel"
          [class.confirm-panel--alert]="request.options.isAlert"
          role="alertdialog"
          aria-modal="true"
          [attr.aria-label]="request.options.title || 'Confirmation'"
          (click)="$event.stopPropagation()"
        >
          <!-- Alert toast-style layout -->
          <ng-container *ngIf="request.options.isAlert">
            <div class="conflict-body">
              <div class="conflict-heading">
                <svg class="warn-icon" viewBox="0 0 16 16" fill="currentColor">
                  <path d="M7.56 1h.88l6.54 12.26-.44.74H1.44L1 13.26Zm.44 2L2.28 13h11.44ZM8 11a.75.75 0 1 1 0 1.5A.75.75 0 0 1 8 11zm.75-5v4.5h-1.5V6h1.5z"/>
                </svg>
                <div>
                  <div class="conflict-title">{{ getAlertTitle(request.options) }}</div>
                  <div class="conflict-sub" *ngIf="getAlertSubtitle(request.options)">{{ getAlertSubtitle(request.options) }}</div>
                </div>
              </div>
              <div class="alert-actions">
                <button type="button" class="alert-btn" (click)="onConfirm()">
                  {{ request.options.confirmText || 'OK' }}
                </button>
              </div>
            </div>
          </ng-container>

          <!-- Regular confirmation dialog layout -->
          <ng-container *ngIf="!request.options.isAlert">
            <div class="confirm-header" *ngIf="request.options.title">
              <h2>{{ request.options.title }}</h2>
            </div>

            <div class="confirm-body">
              <p>{{ request.options.message }}</p>
            </div>

            <div class="confirm-actions">
              <button 
                class="btn-cancel" 
                type="button" 
                (click)="onCancel()"
              >
                {{ request.options.cancelText || 'Cancel' }}
              </button>
              <button
                type="button"
                [class.btn-danger]="request.options.danger"
                [class.btn-confirm]="!request.options.danger"
                (click)="onConfirm()"
              >
                {{ request.options.confirmText || 'Confirm' }}
              </button>
            </div>
          </ng-container>
        </div>
      </div>
    </ng-container>
  `,
  styles: [`
    .confirm-overlay {
      position: fixed;
      inset: 0;
      z-index: var(--vsc-z-modal);
      display: flex;
      align-items: center;
      justify-content: center;
      background: rgba(0, 0, 0, 0.4);
      padding: var(--vsc-padding-xl);
      animation: fadeIn 0.15s ease;
    }

    /* Alert variant - minimal backdrop, positioned at top-center */
    .confirm-overlay--alert {
      background: transparent;
      align-items: flex-start;
      justify-content: center;
      padding: 20px;
    }

    @keyframes fadeIn {
      from {
        opacity: 0;
      }
      to {
        opacity: 1;
      }
    }

    @keyframes slideIn {
      from {
        transform: translateY(-8px);
        opacity: 0;
      }
      to {
        transform: translateY(0);
        opacity: 1;
      }
    }

    @keyframes slideInFromTop {
      from {
        opacity: 0;
        transform: translateY(8px) scale(0.98);
      }
      to {
        opacity: 1;
        transform: translateY(0) scale(1);
      }
    }

    .confirm-panel {
      width: min(480px, calc(100vw - 2 * var(--vsc-padding-xl)));
      background: var(--vsc-panel-background);
      color: var(--vsc-foreground);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45);
      overflow: hidden;
      animation: slideIn 0.2s ease;
    }

    /* Alert panel - toast-style */
    .confirm-panel--alert {
      width: 380px;
      border-radius: 6px;
      border-color: #e8a94d;
      box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45);
      animation: slideInFromTop 0.18s cubic-bezier(0.16, 1, 0.3, 1);
      padding: 0;
    }

    /* ── Alert body (toast-style) ── */
    .conflict-body {
      width: 100%;
      display: flex;
      flex-direction: column;
      gap: 0;
    }

    .conflict-heading {
      display: flex;
      align-items: flex-start;
      gap: 10px;
      padding: 12px 14px 10px;
      border-bottom: 1px solid rgba(232, 169, 77, 0.2);
    }

    .warn-icon {
      width: 16px;
      height: 16px;
      flex-shrink: 0;
      color: #e8a94d;
      margin-top: 1px;
    }

    .conflict-title {
      font-weight: 600;
      font-size: 12px;
      color: #e8a94d;
      line-height: 1.3;
    }

    .conflict-sub {
      font-size: 11px;
      opacity: 0.6;
      margin-top: 1px;
    }

    .alert-actions {
      display: flex;
      justify-content: flex-end;
      padding: 10px 14px;
      gap: 6px;
    }

    .alert-btn {
      padding: 5px 16px;
      font-size: 11px;
      font-weight: 500;
      border-radius: 4px;
      border: 1px solid transparent;
      background: var(--vsc-button-background, #0e639c);
      color: #fff;
      cursor: pointer;
      transition: background 0.12s, border-color 0.12s;
      outline: none;
    }

    .alert-btn:hover {
      background: var(--vsc-button-hover, #1177bb);
    }

    .alert-btn:active {
      background: color-mix(in srgb, var(--vsc-button-hover) 90%, black 10%);
    }

    .alert-btn:focus-visible {
      outline: 1px solid var(--vsc-input-focus-border);
      outline-offset: 2px;
    }

    /* ── Regular confirmation dialog styles ── */
    .confirm-header {
      padding: var(--vsc-padding-md) var(--vsc-padding-lg);
      border-bottom: 1px solid var(--vsc-border-subtle);

      h2 {
        margin: 0;
        font-size: var(--vsc-font-size);
        font-weight: 600;
        color: var(--vsc-foreground-bright);
      }
    }

    .confirm-body {
      padding: var(--vsc-padding-lg);

      p {
        margin: 0;
        white-space: pre-line;
        line-height: 1.45;
        font-size: var(--vsc-font-size);
      }
    }

    .confirm-actions {
      display: flex;
      justify-content: flex-end;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-md) var(--vsc-padding-lg);
      border-top: 1px solid var(--vsc-border-subtle);
      background: color-mix(in srgb, var(--vsc-panel-background) 85%, var(--vsc-sidebar-background) 15%);
    }

    .btn-cancel,
    .btn-confirm,
    .btn-danger {
      min-width: 86px;
      height: 28px;
      padding: 0 var(--vsc-padding-md);
      border-radius: var(--vsc-border-radius-sm);
      border: 1px solid transparent;
      font-size: var(--vsc-font-size);
      cursor: pointer;
      transition: all var(--vsc-transition-fast);
      outline: none;

      &:focus-visible {
        outline: 1px solid var(--vsc-input-focus-border);
        outline-offset: 2px;
      }
    }

    .btn-cancel {
      background: var(--vsc-button-secondary);
      color: var(--vsc-foreground);

      &:hover {
        background: var(--vsc-button-secondary-hover);
      }

      &:active {
        background: color-mix(in srgb, var(--vsc-button-secondary-hover) 90%, black 10%);
      }
    }

    .btn-confirm {
      background: var(--vsc-button-background);
      color: var(--vsc-foreground-bright);

      &:hover {
        background: var(--vsc-button-hover);
      }

      &:active {
        background: color-mix(in srgb, var(--vsc-button-hover) 90%, black 10%);
      }
    }

    .btn-danger {
      background: var(--vsc-error);
      color: white;

      &:hover {
        filter: brightness(1.08);
      }

      &:active {
        filter: brightness(0.95);
      }
    }
  `]
})
export class ConfirmDialogComponent {
  private readonly destroy$ = createSubject<void>();
  private releaseDialogContext: (() => void) | null = null;
  readonly request$: Stream<ConfirmDialogRequest | null> = this.confirmDialogService.activeRequest$;

  constructor(
    private confirmDialogService: ConfirmDialogService,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
  ) {}

  ngOnInit(): void {
    this.inputEngine.registerCommand('dialog.confirm.accept', () => this.onConfirm());
    this.inputEngine.registerCommand('dialog.confirm.cancel', () => this.onCancel());

    this.confirmDialogService.activeRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe(request => {
        if (request) {
          this.releaseDialogContext ??= this.inputContexts.acquire('dialog.confirm');
          return;
        }
        this.releaseDialogContext?.();
        this.releaseDialogContext = null;
      });
  }

  ngOnDestroy(): void {
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.inputEngine.unregisterCommand('dialog.confirm.accept');
    this.inputEngine.unregisterCommand('dialog.confirm.cancel');
    this.destroy$.next();
    this.destroy$.complete();
  }

  onCancel(): void {
    const active = this.confirmDialogService.activeRequest$.value;
    // Clicking overlay on alert should close it
    if (active?.options.isAlert) {
      this.onConfirm();
    } else {
      this.confirmDialogService.resolveActive(false);
    }
  }

  onConfirm(): void {
    this.confirmDialogService.resolveActive(true);
  }

  getAlertTitle(options: any): string {
    if (options.title) {
      return options.title;
    }
    const lines = options.message.split('\n');
    return lines[0] || 'Alert';
  }

  getAlertSubtitle(options: any): string {
    if (options.title) {
      return options.message;
    }
    const lines = options.message.split('\n');
    return lines.slice(1).join('\n');
  }
}
