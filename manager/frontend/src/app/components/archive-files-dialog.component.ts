import { ChangeDetectionStrategy, Component } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import {
  ArchiveFilesDialogService,
  type ArchiveFilesDialogRequest,
} from '../services/archive-files-dialog.service';

@Component({
  selector: 'app-archive-files-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container *ngIf="activeRequest as request">
      <div class="dialog-backdrop" [class.dialog-backdrop--closing]="isClosing" (click)="dialogs.close()" (animationend)="onAnimationEnd($event)">
        <section
          class="dialog-panel"
          role="dialog"
          aria-modal="true"
          [attr.aria-label]="request.title"
          (click)="$event.stopPropagation()">
          <header class="dialog-header">
            <div>
              <h2>{{ request.title }}</h2>
              <p>{{ request.message }}</p>
            </div>
            <button type="button" class="dialog-close-btn" aria-label="Close" title="Close" (click)="dialogs.close()">×</button>
          </header>

          <div class="dialog-body">
            <label class="field">
              <span>Archive name</span>
              <input
                #nameInput
                type="text"
                [(ngModel)]="draftValue"
                (keydown.enter)="submit()"
                (keydown.escape)="dialogs.close()"
                spellcheck="false"
                autocomplete="off" />
            </label>

            <div class="actions">
              <button type="button" (click)="dialogs.close()">Cancel</button>
              <button type="button" class="primary" (click)="submit()">{{ request.confirmText }}</button>
            </div>
          </div>
        </section>
      </div>
    </ng-container>
  `,
  styles: [`
    :host { position: fixed; inset: 0; z-index: 30013; }
    .dialog-backdrop { position: fixed; inset: 0; display: flex; align-items: center; justify-content: center; background: var(--vsc-scrim); backdrop-filter: blur(2px); padding: 24px; animation: fadeIn 0.2s ease; }
    .dialog-backdrop--closing { animation: fadeOut 0.2s ease forwards; pointer-events: none; }
    .dialog-panel {
      width: min(560px, calc(100vw - 48px));
      overflow: hidden;
      border: 1px solid var(--vsc-border-bright, var(--vsc-neutral-overlay-medium));
      border-radius: var(--vsc-border-radius-lg, 8px);
      background: var(--vsc-editor-background, var(--vsc-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
      box-shadow: var(--vsc-widget-shadow), 0 32px 64px var(--vsc-scrim-soft), 0 0 0 1px var(--vsc-neutral-overlay-subtle) inset;
      animation: slideDown 0.28s cubic-bezier(0.16, 1, 0.3, 1);
    }
    .dialog-backdrop--closing .dialog-panel { animation: slideUp 0.2s ease forwards; }
    @keyframes fadeIn { from { opacity: 0; } to { opacity: 1; } }
    @keyframes fadeOut { from { opacity: 1; } to { opacity: 0; } }
    @keyframes slideDown { from { transform: translateY(-16px); opacity: 0; } to { transform: translateY(0); opacity: 1; } }
    @keyframes slideUp { from { transform: translateY(0); opacity: 1; } to { transform: translateY(-16px); opacity: 0; } }
    .dialog-header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      gap: 12px;
      padding: 12px 16px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-titlebar-background);
    }
    .dialog-header h2, .dialog-header p { margin: 0; }
    .dialog-header h2 { color: var(--vsc-foreground-bright); font-size: 14px; line-height: 1.15; font-weight: 600; }
    .dialog-header p { margin-top: 4px; color: var(--vsc-foreground-dim); font-size: 12px; }
    .dialog-body { display: grid; gap: 16px; padding: 16px; }
    .field { display: grid; gap: 6px; font-size: 12px; }
    .field span { color: var(--vsc-foreground-bright); font-size: 12px; font-weight: 500; line-height: 1.3; }
    .field input {
      width: 100%;
      height: 28px;
      border: 1px solid var(--vsc-input-border, var(--vsc-neutral-overlay-strong));
      border-radius: var(--vsc-border-radius-sm, 4px);
      background: var(--vsc-input-background, var(--vsc-control-background));
      color: inherit;
      padding: 0 8px;
      font: inherit;
      box-sizing: border-box;
      transition: border-color var(--vsc-transition-fast), box-shadow var(--vsc-transition-fast);
    }
    .field input:hover { border-color: color-mix(in srgb, var(--vsc-input-focus-border) 40%, var(--vsc-input-border)); }
    .field input:focus { border-color: var(--vsc-input-focus-border); box-shadow: 0 0 0 2px color-mix(in srgb, var(--vsc-input-focus-border) 20%, transparent); outline: none; }
    .actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 2px; }
    button {
      border: 1px solid transparent;
      background: var(--vsc-button-secondary, transparent);
      color: inherit;
      border-radius: var(--vsc-border-radius-sm, 4px);
      min-height: 28px;
      padding: 0 12px;
      font: inherit;
      cursor: pointer;
      transition: background-color var(--vsc-transition-fast), border-color var(--vsc-transition-fast), color var(--vsc-transition-fast);
    }
    button:hover:not(:disabled) { background: var(--vsc-button-secondary-hover, var(--vsc-list-hover-background)); }
    button:focus-visible { outline: 1px solid var(--vsc-input-focus-border); outline-offset: 1px; }
    .primary {
      background: var(--vsc-button-background, var(--vsc-accent-blue));
      color: var(--vsc-button-foreground, var(--vsc-on-accent, #fff));
      border-color: transparent;
    }
  `],
})
export class ArchiveFilesDialogComponent {
  draftValue = '';
  private seededRequest: ArchiveFilesDialogRequest | null = null;

  constructor(readonly dialogs: ArchiveFilesDialogService) {}

  get activeRequest(): ArchiveFilesDialogRequest | null {
    const request = this.dialogs.mountedRequest$.value;
    if (request && request !== this.seededRequest) {
      this.seededRequest = request;
      this.draftValue = request.defaultValue;
    } else if (!request) {
      this.seededRequest = null;
      this.draftValue = '';
    }
    return request;
  }

  get isClosing(): boolean {
    return !!this.dialogs.closingRequest$.value;
  }

  submit(): void {
    this.dialogs.submit(this.draftValue);
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) {
      return;
    }
    this.dialogs.finishClose();
  }
}
