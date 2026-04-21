import { Component, ElementRef, OnDestroy, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { InputHistoryService } from '../services/input-history.service';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import { DialogFocusTrapDirective } from '../directives/dialog-focus-trap.directive';
import { moveFocusWithin } from '../directives/focus-trap.utils';
import type { Stream } from '@epikodelabs/streamix';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

interface ConfirmDialogRequest {
  options: any;
  resolve: (value: boolean | string | null) => void;
}

@Component({
  selector: 'app-confirm-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogFocusTrapDirective],
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
          [appTrapFocus]="true"
          [attr.role]="request.options.isAlert ? 'alertdialog' : 'dialog'"
          aria-modal="true"
          tabindex="-1"
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
                <button
                  type="button"
                  class="alert-btn"
                  data-dialog-initial-focus
                  data-dialog-action="confirm"
                  (click)="onConfirm()"
                >
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
              <p [class.confirm-body-text--prompt]="request.options.isPrompt">{{ request.options.message }}</p>

              <div class="prompt-block" *ngIf="request.options.isPrompt">
                <input
                  class="prompt-input"
                  type="text"
                  [(ngModel)]="draftValue"
                  [placeholder]="request.options.placeholder || ''"
                  [attr.list]="request.options.historyKey ? 'prompt-history-list' : null"
                  autocomplete="off"
                  data-dialog-initial-focus
                  (keydown)="onPromptKeyDown($event)"
                />
                <datalist *ngIf="request.options.historyKey && promptHistory.length" id="prompt-history-list">
                  <option *ngFor="let item of promptHistory" [value]="item"></option>
                </datalist>
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
                type="button"
                data-dialog-action="confirm"
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
      background: rgba(0, 0, 0, 0.45);
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

    .confirm-panel {
      width: min(480px, calc(100vw - 2 * var(--vsc-padding-xl)));
      background: var(--vsc-editor-background);
      color: var(--vsc-foreground);
      border: 1px solid var(--vsc-border-bright);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow), 0 20px 48px rgba(0, 0, 0, 0.45);
      overflow: hidden;
      animation: slideDown 0.28s cubic-bezier(0.16, 1, 0.3, 1);
    }

    /* Alert panel - toast-style */
    .confirm-panel--alert {
      width: 380px;
      border-radius: 6px;
      border-color: #e8a94d;
      box-shadow: var(--vsc-widget-shadow), 0 16px 40px rgba(0, 0, 0, 0.5);
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
  @ViewChild('dialogPanel') private dialogPanel?: ElementRef<HTMLElement>;

  private readonly destroy$ = createSubject<void>();
  private releaseDialogContext: (() => void) | null = null;
  draftValue = '';
  promptHistory: string[] = [];
  readonly request$: Stream<ConfirmDialogRequest | null> = this.confirmDialogService.activeRequest$;

  constructor(
    private confirmDialogService: ConfirmDialogService,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
    private overlayFreeze: NativeOverlayFreezeService,
    private inputHistory: InputHistoryService,
  ) {}

  ngOnInit(): void {
    this.inputEngine.registerCommand('dialog.confirm.accept', () => this.onKeyboardAccept());
    this.inputEngine.registerCommand('dialog.confirm.cancel', () => this.onCancel());
    this.inputEngine.registerCommand('dialog.confirm.focusNext', () => this.moveDialogFocus(1));
    this.inputEngine.registerCommand('dialog.confirm.focusPrev', () => this.moveDialogFocus(-1));
    this.inputEngine.registerCommand('dialog.confirm.focusNextArrow', () => this.moveDialogFocusWithArrows(1));
    this.inputEngine.registerCommand('dialog.confirm.focusPrevArrow', () => this.moveDialogFocusWithArrows(-1));

    this.confirmDialogService.activeRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe(request => {
        if (request) {
          this.draftValue = request.options.defaultValue ?? '';
          this.promptHistory = request.options.historyKey
            ? this.inputHistory.get(request.options.historyKey)
            : [];
          this.releaseDialogContext ??= this.inputContexts.acquire('dialog.confirm');
          this.overlayFreeze.acquire('dialog.confirm');
          return;
        }
        this.draftValue = '';
        this.promptHistory = [];
        this.releaseDialogContext?.();
        this.releaseDialogContext = null;
        this.overlayFreeze.release('dialog.confirm');
      });
  }

  ngOnDestroy(): void {
    this.overlayFreeze.release('dialog.confirm');
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.inputEngine.unregisterCommand('dialog.confirm.accept');
    this.inputEngine.unregisterCommand('dialog.confirm.cancel');
    this.inputEngine.unregisterCommand('dialog.confirm.focusNext');
    this.inputEngine.unregisterCommand('dialog.confirm.focusPrev');
    this.inputEngine.unregisterCommand('dialog.confirm.focusNextArrow');
    this.inputEngine.unregisterCommand('dialog.confirm.focusPrevArrow');
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

  private onKeyboardAccept(): void {
    const active = this.confirmDialogService.activeRequest$.value;
    if (!active) {
      return;
    }

    if (active.options.isPrompt) {
      this.onConfirm();
      return;
    }

    const focusedAction = this.getFocusedDialogAction();
    if (focusedAction === 'cancel') {
      this.onCancel();
      return;
    }

    this.onConfirm();
  }

  private getFocusedDialogAction(): 'confirm' | 'cancel' | null {
    const activeElement = document.activeElement;
    if (!(activeElement instanceof HTMLElement)) {
      return null;
    }

    const actionElement = activeElement.closest('[data-dialog-action]');
    if (!(actionElement instanceof HTMLElement)) {
      return null;
    }

    const action = actionElement.dataset['dialogAction'];
    return action === 'cancel' || action === 'confirm' ? action : null;
  }

  onPromptKeyDown(event: KeyboardEvent): void {
    if (event.key === 'Enter') {
      event.preventDefault();
      this.onConfirm();
      return;
    }

    if (event.key === 'Escape') {
      event.preventDefault();
      this.onCancel();
    }
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

  private moveDialogFocus(delta: number): void {
    const panel = this.dialogPanel?.nativeElement;
    if (!panel) {
      return;
    }

    moveFocusWithin(panel, delta);
  }

  private moveDialogFocusWithArrows(delta: number): void {
    if (!this.shouldUseActionArrowNavigation()) {
      return;
    }

    this.moveActionFocus(delta);
  }

  private moveActionFocus(delta: number): void {
    const panel = this.dialogPanel?.nativeElement;
    if (!panel) {
      return;
    }

    moveFocusWithin(panel, delta, '[data-dialog-action]');
  }

  private shouldUseActionArrowNavigation(): boolean {
    const panel = this.dialogPanel?.nativeElement;
    const activeElement = document.activeElement;
    if (!panel || !(activeElement instanceof HTMLElement) || !panel.contains(activeElement)) {
      return false;
    }

    return !!activeElement.closest('[data-dialog-action]');
  }
}
