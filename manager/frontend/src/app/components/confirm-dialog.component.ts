import { Component, ElementRef, OnDestroy, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ConfirmDialogService, type ConfirmDialogDetailRow } from '../services/confirm-dialog.service';
import { InputHistoryService } from '../services/input-history.service';
import type { Stream } from '@epikodelabs/streamix';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

interface ConfirmDialogRequest {
  kind: 'confirm' | 'alert' | 'prompt';
  options: any;
  resolves: Array<(value: boolean | string | null) => void>;
}

@Component({
  selector: 'app-confirm-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <ng-container *ngIf="request$ | async as request">
      <div 
        class="confirm-overlay"
        [class.confirm-overlay--alert]="request.options.isAlert"
        [class.confirm-overlay--detailed]="request.options.isAlert && request.options.details?.length"
        (click)="onCancel()"
      >
        <div
          #dialogPanel
          class="confirm-panel"
          data-input-owned
          [class.confirm-panel--alert]="request.options.isAlert"
          [class.confirm-panel--detailed]="request.options.isAlert && request.options.details?.length"
          [attr.data-tone]="request.options.tone || 'warning'"
          [class.confirm-panel--prompt]="request.options.isPrompt && !request.options.isAlert"
          [attr.role]="request.options.isAlert ? 'alertdialog' : 'dialog'"
          aria-modal="true"
          tabindex="-1"
          [attr.aria-label]="request.options.title || 'Confirmation'"
          (keydown.escape)="onCancel(); $event.preventDefault(); $event.stopPropagation()"
          (mousedown)="$event.stopPropagation()"
          (click)="$event.stopPropagation()"
        >
          <!-- Alert / structured information layout -->
          <ng-container *ngIf="request.options.isAlert">
            <div class="conflict-body">
              <div class="conflict-heading">
                <span class="alert-symbol" aria-hidden="true">
                  {{ alertSymbol(request.options.tone) }}
                </span>
                <div class="alert-heading-copy">
                  <div class="conflict-title">{{ getAlertTitle(request.options) }}</div>
                  <div class="conflict-sub" *ngIf="getAlertSubtitle(request.options)">
                    {{ getAlertSubtitle(request.options) }}
                  </div>
                </div>
              </div>

              <div class="detail-scroll" *ngIf="request.options.details?.length">
                <section
                  class="detail-section"
                  *ngFor="let section of request.options.details">
                  <h3 *ngIf="section.title">{{ section.title }}</h3>
                  <dl class="property-grid">
                    <ng-container *ngFor="let row of section.rows">
                      <dt>{{ row.label }}</dt>
                      <dd [class.property-value--mono]="row.monospace">
                        <span
                          class="property-value-line"
                          *ngFor="let value of detailValues(row)">
                          {{ value }}
                        </span>
                      </dd>
                    </ng-container>
                  </dl>
                </section>
              </div>

              <footer class="alert-actions conflict-footer">
                <span
                  class="conflict-footer-label"
                  *ngIf="alertFooterHint(request.options) as footerHint">
                  {{ footerHint }}
                </span>
                <button
                  type="button"
                  class="alert-btn"
                  data-dialog-initial-focus
                  data-dialog-action="confirm"
                  (click)="onConfirm()">
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

    .confirm-overlay--detailed {
      align-items: center;
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

    .confirm-panel--detailed {
      width: min(620px, calc(100vw - 40px));
      max-height: min(720px, calc(100vh - 40px));
    }

    .detail-scroll {
      overflow: auto;
      padding: 4px 14px 12px;
      max-height: min(560px, calc(100vh - 170px));
      background: var(--vsc-editor-background);
    }

    .detail-section {
      padding: 10px 0 2px;
      border-top: 1px solid var(--vsc-border-subtle);
    }

    .detail-section:first-child {
      border-top: 0;
    }

    .detail-section h3 {
      margin: 0 0 7px;
      color: var(--vsc-foreground-bright, var(--vsc-foreground));
      font-size: 11px;
      font-weight: 600;
    }

    .property-grid {
      display: grid;
      grid-template-columns: minmax(120px, 160px) minmax(0, 1fr);
      gap: 4px 14px;
      margin: 0;
      font-size: 11px;
      line-height: 1.45;
    }

    .property-grid dt {
      min-width: 0;
      color: var(--vsc-description-foreground);
      text-align: right;
    }

    .property-grid dd {
      min-width: 0;
      margin: 0;
      color: var(--vsc-foreground);
      overflow-wrap: anywhere;
      user-select: text;
    }

    .property-value-line {
      display: block;
    }

    .property-value--mono {
      font-family: var(--vsc-editor-font-family, Consolas, monospace);
      font-size: 10.5px;
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

    .alert-symbol {
      display: inline-grid;
      place-items: center;
      width: 18px;
      height: 18px;
      flex: 0 0 auto;
      margin-top: 1px;
      border: 1px solid currentColor;
      border-radius: 50%;
      color: var(--vsc-warning);
      font-size: 11px;
      font-weight: 700;
      line-height: 1;
    }

    .alert-heading-copy {
      min-width: 0;
      flex: 1 1 auto;
    }

    .confirm-panel--alert[data-tone='info'] {
      border-color: var(--vsc-accent-blue, var(--vsc-focus-border));
    }

    .confirm-panel--alert[data-tone='info'] .alert-symbol,
    .confirm-panel--alert[data-tone='info'] .conflict-title {
      color: var(--vsc-accent-blue, var(--vsc-focus-border));
    }

    .confirm-panel--alert[data-tone='success'] {
      border-color: var(--vsc-success, #16825d);
    }

    .confirm-panel--alert[data-tone='success'] .alert-symbol,
    .confirm-panel--alert[data-tone='success'] .conflict-title {
      color: var(--vsc-success, #16825d);
    }

    .confirm-panel--alert[data-tone='error'] {
      border-color: var(--vsc-error);
    }

    .confirm-panel--alert[data-tone='error'] .alert-symbol,
    .confirm-panel--alert[data-tone='error'] .conflict-title {
      color: var(--vsc-error);
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
          this.focusInitialControl();
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

  private focusInitialControl(): void {
    if (this.promptInput?.nativeElement) {
      this.focusPromptInput();
      return;
    }

    requestAnimationFrame(() => {
      const panel = this.dialogPanel?.nativeElement;
      if (!panel) {
        return;
      }

      const initial = panel.querySelector<HTMLElement>('[data-dialog-initial-focus]');
      if (initial) {
        initial.focus({ preventScroll: true });
        return;
      }

      panel.focus({ preventScroll: true });
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

  detailValues(row: ConfirmDialogDetailRow): readonly string[] {
    return typeof row.value === 'string'
      ? [row.value]
      : Array.from(row.value, String);
  }

  alertSymbol(tone: string | undefined): string {
    switch (tone) {
      case 'info': return 'i';
      case 'success': return '✓';
      case 'error': return '×';
      default: return '!';
    }
  }

  alertFooterHint(options: any): string | null {
    if (options.footerHint === false) return null;
    if (typeof options.footerHint === 'string') return options.footerHint;
    return options.details?.length ? null : 'Press Enter to continue';
  }
}
