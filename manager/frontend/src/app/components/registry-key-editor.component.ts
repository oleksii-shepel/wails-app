import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  OnDestroy,
  OnInit,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil } from '@epikodelabs/streamix';
import {
  RegistryKeyEditorService,
  type RegistryKeyEditRequest,
} from '../services/registry-key-editor.service';

@Component({
  selector: 'app-registry-key-editor',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      *ngIf="request"
      class="registry-key-overlay"
      data-input-owned
      (click)="cancel()">
      <section
        class="registry-key-dialog"
        role="dialog"
        aria-modal="true"
        [attr.aria-label]="request.title"
        data-input-owned
        (click)="$event.stopPropagation()"
        (mousedown)="$event.stopPropagation()"
        (mouseup)="$event.stopPropagation()"
        (dblclick)="$event.stopPropagation()"
        (keydown)="onDialogKeyDown($event)"
        (keyup)="$event.stopPropagation()">
        <header>
          <strong>{{ request.title }}</strong>
          <button type="button" class="close" aria-label="Close" (click)="cancel()">×</button>
        </header>

        <form (ngSubmit)="save()" data-input-owned>
          <label>
            <span>Name</span>
            <input
              #nameInput
              name="registry-key-name"
              [(ngModel)]="name"
              autocomplete="off"
              spellcheck="false"
              data-input-owned
              (click)="$event.stopPropagation()"
              (mousedown)="$event.stopPropagation()"
              (mouseup)="$event.stopPropagation()"
              (keydown)="$event.stopPropagation()"
              (keyup)="$event.stopPropagation()" />
          </label>

          <footer>
            <button
            type="button"
            (mousedown)="$event.stopPropagation()"
            (click)="cancel(); $event.stopPropagation()">
            Cancel
          </button>
            <button
              type="submit"
              class="primary"
              [disabled]="!name.trim()"
              (mousedown)="$event.stopPropagation()"
              (click)="$event.stopPropagation()">
              {{ request.confirmText }}
            </button>
          </footer>
        </form>
      </section>
    </div>
  `,
  styles: [`
    :host {
      position: fixed;
      inset: 0;
      z-index: 40000;
      pointer-events: none;
    }

    .registry-key-overlay {
      position: fixed;
      inset: 0;
      display: grid;
      place-items: center;
      padding: 24px;
      background: var(--vsc-scrim);
      pointer-events: auto;
    }

    .registry-key-dialog {
      box-sizing: border-box;
      width: min(460px, calc(100vw - 48px));
      background: var(--vsc-editor-background);
      color: var(--vsc-foreground);
      border: 1px solid var(--vsc-border-bright);
      border-radius: 6px;
      box-shadow: var(--vsc-widget-shadow), 0 20px 48px var(--vsc-scrim);
      pointer-events: auto;
    }

    header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 12px 14px;
      border-bottom: 1px solid var(--vsc-border);
    }

    .close {
      width: 28px;
      height: 28px;
      padding: 0;
      border: 0;
      background: transparent;
      color: inherit;
      cursor: pointer;
    }

    form {
      display: grid;
      gap: 14px;
      padding: 14px;
      pointer-events: auto;
    }

    label {
      display: grid;
      grid-template-columns: 64px minmax(0, 1fr);
      gap: 10px;
      align-items: center;
    }

    label > span {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
    }

    input {
      box-sizing: border-box;
      width: 100%;
      height: 28px;
      padding: 4px 8px;
      border: 1px solid var(--vsc-input-border, var(--vsc-border));
      border-radius: 3px;
      background: var(--vsc-input-background);
      color: var(--vsc-input-foreground, var(--vsc-foreground));
      font: inherit;
      pointer-events: auto;
    }

    input:focus {
      outline: 1px solid var(--vsc-focus-border, var(--vsc-accent-blue));
      outline-offset: -1px;
    }

    footer {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
    }

    button {
      min-height: 28px;
      padding: 4px 12px;
      border: 1px solid var(--vsc-button-border, var(--vsc-border));
      border-radius: 3px;
      background: var(--vsc-button-secondary-background, var(--vsc-panel-background));
      color: var(--vsc-button-secondary-foreground, var(--vsc-foreground));
      cursor: pointer;
      pointer-events: auto;
    }

    button.primary {
      background: var(--vsc-button-background);
      color: var(--vsc-button-foreground);
    }

    button:disabled {
      opacity: .45;
      cursor: default;
    }
  `],
})
export class RegistryKeyEditorComponent implements OnInit, OnDestroy {
  @ViewChild('nameInput')
  private nameInput?: ElementRef<HTMLInputElement>;

  request: RegistryKeyEditRequest | null = null;
  name = '';

  private readonly destroy$ = createSubject<void>();

  constructor(
    private readonly editor: RegistryKeyEditorService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    this.editor.active$
      .pipe(takeUntil(this.destroy$))
      .subscribe(active => {
        this.request = active?.request ?? null;
        this.name = active?.request.name ?? '';
        this.cdr.markForCheck();

        if (active) {
          requestAnimationFrame(() => {
            const input = this.nameInput?.nativeElement;
            if (!input) return;
            input.focus({ preventScroll: true });
            const end = input.value.length;
            input.setSelectionRange(end, end);
          });
        }
      });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  save(): void {
    const name = this.name.trim();
    if (!name) return;
    this.editor.resolve(name);
  }

  cancel(): void {
    this.editor.resolve(null);
  }

  onDialogKeyDown(event: KeyboardEvent): void {
    event.stopPropagation();

    if (event.key === 'Escape') {
      event.preventDefault();
      this.cancel();
    }
  }
}
