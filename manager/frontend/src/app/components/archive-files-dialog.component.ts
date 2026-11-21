import { ChangeDetectionStrategy, Component, ElementRef, OnDestroy, OnInit, ViewChild } from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import {
  ArchiveFilesDialogService,
  type ArchiveFilesDialogRequest,
} from '../services/archive-files-dialog.service';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';

const ARCHIVE_EXTENSION_PATTERN = /\.(?:tar\.gz|zip|tgz|tar|7z)$/i;

@Component({
  selector: 'app-archive-files-dialog',
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container *ngIf="activeRequest as request">
      <div class="dialog-backdrop palette-dialog-backdrop dialog-group" [class.dialog-backdrop--closing]="isClosing" [class.dialog-group--closing]="isClosing" (click)="dialogs.close()" (animationend)="onAnimationEnd($event)">
        <section
          class="dialog-panel palette-dialog-surface dialog-surface"
          role="dialog"
          aria-modal="true"
          [attr.aria-label]="request.title"
          (click)="$event.stopPropagation()" (keydown.escape)="dialogs.close(); $event.preventDefault(); $event.stopPropagation()">
          <header class="dialog-header">
            <div>
              <h2>{{ request.title }}</h2>
              <p>{{ request.message }}</p>
            </div>
            <span class="dialog-close" aria-label="Close tab" title="Close (Esc)" (click)="dialogs.close()" role="button" tabindex="0" (keydown.enter)="dialogs.close()" (keydown.space)="dialogs.close(); $event.preventDefault()"> × </span>
          </header>

          <div class="dialog-body">
            <label class="field">
              <span>Archive name</span>
              <input
                #nameInput
                type="text"
                [value]="draftValue"
                (input)="onDraftInput($event)"
                (keydown.enter)="submit()"
                (keydown.escape)="dialogs.close()"
                spellcheck="false"
                autocomplete="off" />
            </label>

            <div class="preset-row" role="group" aria-label="Archive format presets">
              <span class="preset-label">Presets:</span>
              <button type="button" class="preset-btn" [class.active]="hasExtension('.zip')" (click)="applyExtension('.zip')">.zip</button>
              <button type="button" class="preset-btn" [class.active]="hasExtension('.tar.gz')" (click)="applyExtension('.tar.gz')">.tar.gz</button>
              <button type="button" class="preset-btn" (click)="applyDatedPreset()">+ Date stamp</button>
            </div>

            <div *ngIf="validationError" class="validation-note" role="alert">
              {{ validationError }}
            </div>

            <div class="actions">
              <button type="button" (click)="dialogs.close()">Cancel</button>
              <button type="button" class="primary" [disabled]="!!validationError" (click)="submit()">{{ request.confirmText }}</button>
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
      color: var(--vsc-foreground);
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
    .dialog-body { display: grid; gap: 12px; padding: 16px; }
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
    .preset-row { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; }
    .preset-label { font-size: 11px; color: var(--vsc-foreground-dim); }
    .preset-btn {
      min-height: 24px;
      padding: 0 8px;
      border: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      border-radius: var(--vsc-border-radius-sm, 4px);
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
    }
    .preset-btn.active {
      color: var(--vsc-foreground-bright);
      border-color: color-mix(in srgb, var(--vsc-accent-blue) 55%, var(--vsc-border));
      background: color-mix(in srgb, var(--vsc-accent-blue) 14%, transparent);
    }
    .validation-note {
      font-size: 11px;
      color: var(--vsc-error-foreground, var(--vsc-error));
      padding: 4px 8px;
      border-radius: var(--vsc-border-radius-sm, 4px);
      background: var(--vsc-error-surface);
    }
    .actions { display: flex; justify-content: flex-end; gap: 8px; padding-top: 4px; }
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
    button:disabled { opacity: 0.45; cursor: not-allowed; }
    button:focus-visible { outline: 1px solid var(--vsc-input-focus-border); outline-offset: 1px; }
    .primary {
      background: var(--vsc-button-background, var(--vsc-accent-blue));
      color: var(--vsc-button-foreground, var(--vsc-on-accent));
      border-color: transparent;
    }
  `],
})
export class ArchiveFilesDialogComponent implements OnInit, OnDestroy {
  @ViewChild('nameInput') private nameInput?: ElementRef<HTMLInputElement>;

  draftValue = '';
  private seededRequest: ArchiveFilesDialogRequest | null = null;
  private readonly destroy$ = createSubject<void>();
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;

  constructor(readonly dialogs: ArchiveFilesDialogService) {}

  ngOnInit(): void {
    this.dialogs.activeRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe((request) => {
        if (request) {
          if (this.modalStack && this.hostElement && !this.unregisterModal) {
            this.unregisterModal = this.modalStack.register({
              id: 'archive-files-dialog',
              element: this.hostElement.nativeElement,
              dismiss: () => this.dialogs.close(),
              dismissible: () => !this.isClosing,
            });
          }
        } else {
          this.unregisterModal?.();
          this.unregisterModal = null;
        }
      });
  }

  ngOnDestroy(): void {
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.destroy$.next();
    this.destroy$.complete();
  }

  get activeRequest(): ArchiveFilesDialogRequest | null {
    const request = this.dialogs.mountedRequest$.value;
    if (request && request !== this.seededRequest) {
      this.seededRequest = request;
      this.draftValue = request.defaultValue;
      this.selectStemOnOpen();
    } else if (!request) {
      this.seededRequest = null;
      this.draftValue = '';
    }
    return request;
  }

  get validationError(): string {
    const trimmed = this.draftValue.trim();
    if (!trimmed) {
      return 'Enter a non-empty archive filename.';
    }
    if (trimmed.includes('/') || trimmed.includes('\\') || trimmed === '.' || trimmed === '..') {
      return 'Archive name cannot contain path separators.';
    }
    return '';
  }

  get isClosing(): boolean {
    return !!this.dialogs.closingRequest$.value;
  }

  onDraftInput(event: Event): void {
    this.draftValue = (event.target as HTMLInputElement).value;
  }

  hasExtension(ext: string): boolean {
    return this.draftValue.trim().toLowerCase().endsWith(ext.toLowerCase());
  }

  applyExtension(ext: string): void {
    const stem = this.stripExtension(this.draftValue.trim()) || 'archive';
    this.draftValue = `${stem}${ext}`;
    this.focusInputAtEnd();
  }

  applyDatedPreset(): void {
    const trimmed = this.draftValue.trim() || 'archive.zip';
    const match = ARCHIVE_EXTENSION_PATTERN.exec(trimmed);
    const ext = match ? match[0] : '.zip';
    const stem = this.stripExtension(trimmed) || 'archive';
    const now = new Date();
    const dateTag = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}-${String(now.getDate()).padStart(2, '0')}`;
    const cleanedStem = stem.replace(/-\d{4}-\d{2}-\d{2}$/, '');
    this.draftValue = `${cleanedStem}-${dateTag}${ext}`;
    this.focusInputAtEnd();
  }

  submit(): void {
    if (this.validationError) return;
    this.dialogs.submit(this.draftValue.trim());
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) {
      return;
    }
    this.dialogs.finishClose();
  }

  private stripExtension(value: string): string {
    return value.replace(ARCHIVE_EXTENSION_PATTERN, '');
  }

  private selectStemOnOpen(): void {
    requestAnimationFrame(() => {
      const input = this.nameInput?.nativeElement;
      if (!input) return;
      input.focus({ preventScroll: true });
      const val = input.value;
      const match = ARCHIVE_EXTENSION_PATTERN.exec(val);
      const end = match ? match.index : val.length;
      input.setSelectionRange(0, Math.max(0, end));
    });
  }

  private focusInputAtEnd(): void {
    requestAnimationFrame(() => {
      const input = this.nameInput?.nativeElement;
      if (!input) return;
      input.focus({ preventScroll: true });
    });
  }
}