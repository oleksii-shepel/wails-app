import { Component, ElementRef, OnDestroy, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputHistoryService } from '../services/input-history.service';
import { InputHistoryDirective } from '../directives/input-history.directive';
import type { Stream } from '@epikodelabs/streamix';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

interface ConfirmDialogRequest {
  options: any;
  resolve: (value: boolean | string | null) => void;
}

@Component({
  selector: 'app-confirm-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule, InputHistoryDirective],
  template: `
    <ng-container *ngIf="request$ | async as request">
      <div 
        class="confirm-overlay"
        [class.confirm-overlay--alert]="request.options.isAlert"
        (click)="onCancel()"
      >
        <div
          #dialogPanel
          class="confirm-panel"
          [class.confirm-panel--alert]="request.options.isAlert"
          [class.confirm-panel--prompt]="request.options.isPrompt && !request.options.isAlert"
          [attr.role]="request.options.isAlert ? 'alertdialog' : 'dialog'"
          aria-modal="true"
          tabindex="-1"
          [attr.aria-label]="request.options.title || 'Confirmation'"
          (keydown.escape)="onCancel(); $event.preventDefault(); $event.stopPropagation()"
          (mousedown)="$event.stopPropagation()"
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
              <footer class="alert-actions conflict-footer">
                <span class="conflict-footer-label">Press Enter to continue</span>
                <button
                  type="button"
                  class="alert-btn"
                  data-dialog-initial-focus
                  data-dialog-action="confirm"
                  (click)="onConfirm()"
                >
                  {{ request.options.confirmText || 'OK' }}
                </button>
              </footer>
            </div>
          </ng-container>

          <!-- Regular confirmation / prompt dialog layout -->
          <form
            *ngIf="!request.options.isAlert"
            class="confirm-form"
            (ngSubmit)="onConfirm()">
            <div class="confirm-header" *ngIf="request.options.title">
              <h2>{{ request.options.title }}</h2>
            </div>

            <div class="confirm-body">
              <p [class.confirm-body-text--prompt]="request.options.isPrompt">{{ request.options.message }}</p>

              <div class="prompt-block" *ngIf="request.options.isPrompt">
                <input
                  #promptInput
                  class="prompt-input"
                  type="text"
                  name="confirm-prompt-value"
                  [(ngModel)]="draftValue"
                  [placeholder]="request.options.placeholder || ''"
                  [appInputHistory]="request.options.historyKey || null"
                  [appInputHistoryEnabled]="!!request.options.historyKey"
                  data-dialog-initial-focus
                  autocomplete="off"
                />
              </div>
            </div>

            <div class="confirm-actions">
              <button 
                class="btn-cancel" 
                [attr.data-dialog-initial-focus]="request.options.isPrompt ? null : ''"
                data-dialog-action="cancel"
                type="button" 
                (click)="onCancel()"
              >
                {{ request.options.cancelText || 'Cancel' }}
              </button>
              <button
                type="submit"
                data-dialog-action="confirm"
                [class.btn-danger]="request.options.danger"
                [class.btn-confirm]="!request.options.danger"
              >
                {{ request.options.confirmText || 'Confirm' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </ng-container>
  `,
  styles: [`
    :host {
      position: fixed;
      inset: 0;
      z-index: 30000;
      pointer-events: none;
    }

    .confirm-overlay {
      position: fixed;
      inset: 0;
      z-index: 30000;
      display: flex;
      pointer-events: auto;
      align-items: center;
      justify-content: center;
      background: var(--vsc-scrim);
      backdrop-filter: blur(2px);
      padding: var(--vsc-padding-xl);
      animation: fadeIn 0.2s ease;
    }

    /* Alert variant - minimal backdrop, positioned at top-center */
    .confirm-overlay--alert {
      background: transparent;
      align-items: flex-start;
      justify-content: center;
      padding: 20px;
    }

    @keyframes fadeIn {
      from { opacity: 0; }
      to { opacity: 1; }
    }

    @keyframes slideDown {
      from { transform: translateY(-16px); opacity: 0; }
      to { transform: translateY(0); opacity: 1; }
    }

    .confirm-form {
      display: contents;
    }

    .confirm-panel input,
    .confirm-panel button,
    .confirm-panel form {
      pointer-events: auto;
    }

    .confirm-panel {
      width: min(480px, calc(100vw - 2 * var(--vsc-padding-xl)));
      position: relative;
      z-index: 1;
      pointer-events: auto;
      background: var(--vsc-editor-background);
      color: var(--vsc-foreground);
      border: 1px solid var(--vsc-border-bright);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow), 0 20px 48px var(--vsc-scrim);
      overflow: hidden;
      animation: slideDown 0.28s cubic-bezier(0.16, 1, 0.3, 1);
    }

    /* Alert panel - toast-style */
    .confirm-panel--alert {
      width: 380px;
      border-radius: 6px;
      border-color: var(--vsc-warning);
      box-shadow: var(--vsc-widget-shadow), 0 16px 40px var(--vsc-scrim-strong);
      animation: slideDown 0.22s cubic-bezier(0.16, 1, 0.3, 1);
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
      border-bottom: 1px solid var(--vsc-warning-surface);
    }

    .warn-icon {
      width: 16px;
      height: 16px;
      flex-shrink: 0;
      color: var(--vsc-warning);
      margin-top: 1px;
    }

    .conflict-title {
      font-weight: 600;
      font-size: 12px;
      color: var(--vsc-warning);
      line-height: 1.3;
    }

    .conflict-sub {
      font-size: 11px;
      opacity: 0.6;
      margin-top: 1px;
    }

    .alert-actions {
      display: flex;
      align-items: center;
      justify-content: flex-end;
      padding: 10px 14px;
      gap: 10px;
      border-top: 1px solid var(--vsc-border-subtle);
      background: color-mix(in srgb, var(--vsc-panel-background) 88%, var(--vsc-sidebar-background) 12%);
    }

    .conflict-footer-label {
      margin-right: auto;
      color: var(--vsc-description-foreground);
      font-size: 10px;
      line-height: 1.2;
    }

    .alert-btn {
      padding: 5px 16px;
      font-size: 11px;
      font-weight: 500;
      border-radius: 4px;
      border: 1px solid transparent;
      background: var(--vsc-button-background, var(--vsc-button-background));
      color: var(--vsc-button-foreground, var(--vsc-on-accent));
      cursor: pointer;
      transition: background 0.12s, border-color 0.12s;
      outline: none;
    }

    .alert-btn:hover {
      background: var(--vsc-button-hover, var(--vsc-button-hover));
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
      padding: 12px 16px;
      border-bottom: 1px solid var(--vsc-border-subtle);
      background: var(--vsc-titlebar-background);

      h2 {
        margin: 0;
        font-size: 16px;
        font-weight: 600;
        color: var(--vsc-foreground-bright);
        line-height: 1.2;
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

    .prompt-block {
      margin-top: var(--vsc-padding-md);
      padding-top: var(--vsc-padding-md);
      border-top: 1px solid var(--vsc-border-subtle);
    }

    .prompt-input {
      width: 100%;
      height: 30px;
      padding: 0 var(--vsc-padding-md);
      border-radius: var(--vsc-border-radius-sm);
      border: 1px solid var(--vsc-input-border);
      background: var(--vsc-input-background);
      color: var(--vsc-input-foreground);
      font: inherit;
      box-sizing: border-box;
      outline: none;

      &:focus {
        border-color: var(--vsc-input-focus-border);
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
      color: var(--vsc-button-secondary-foreground);

      &:hover {
        background: var(--vsc-button-secondary-hover);
      }

      &:active {
        background: color-mix(in srgb, var(--vsc-button-secondary-hover) 90%, black 10%);
      }
    }

    .btn-confirm {
      background: var(--vsc-button-background);
      color: var(--vsc-button-foreground);

      &:hover {
        background: var(--vsc-button-hover);
      }

      &:active {
        background: color-mix(in srgb, var(--vsc-button-hover) 90%, black 10%);
      }
    }

    .btn-danger {
      background: var(--vsc-error);
      color: var(--vsc-on-accent);

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
  @ViewChild('dialogPanel') private dialogPanel?: ElementRef<HTMLElement>;
  @ViewChild('promptInput') private promptInput?: ElementRef<HTMLInputElement>;

  private readonly destroy$ = createSubject<void>();
  draftValue = '';
  readonly request$: Stream<ConfirmDialogRequest | null> = this.confirmDialogService.activeRequest$;

  constructor(
    private confirmDialogService: ConfirmDialogService,
    private inputHistory: InputHistoryService,
  ) {}

  ngOnInit(): void {
    this.confirmDialogService.activeRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe(request => {
        if (request) {
          this.draftValue = request.options.defaultValue ?? '';
          if (request.options.isPrompt) {
            this.focusPromptInput();
          }
          return;
        }

        this.draftValue = '';
      });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  onCancel(): void {
    const active = this.confirmDialogService.activeRequest$.value;
    // Clicking overlay on alert should close it
    if (active?.options.isAlert) {
      this.onConfirm();
    } else if (active?.options.isPrompt) {
      this.confirmDialogService.resolveActive(null);
    } else {
      this.confirmDialogService.resolveActive(false);
    }
  }

  onConfirm(): void {
    const active = this.confirmDialogService.activeRequest$.value;
    if (active?.options.isPrompt) {
      const value = this.draftValue;
      if (active.options.historyKey && value.trim()) {
        this.inputHistory.push(active.options.historyKey, value);
      }
      this.confirmDialogService.resolveActive(value);
      return;
    }
    this.confirmDialogService.resolveActive(true);
  }

  private focusPromptInput(attempt = 0): void {
    requestAnimationFrame(() => {
      const input = this.promptInput?.nativeElement;

      if (!input) {
        if (attempt < 8) {
          setTimeout(() => this.focusPromptInput(attempt + 1), 16);
        }
        return;
      }

      input.focus({ preventScroll: true });
      const end = input.value.length;
      input.setSelectionRange(end, end);
    });
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