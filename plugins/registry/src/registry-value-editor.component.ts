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
  RegistryValueEditorService,
  type RegistryValueEditRequest,
} from './registry-value-editor.service';

@Component({
  selector: 'app-registry-value-editor',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      *ngIf="request"
      class="registry-editor-overlay"
      data-input-owned
      (click)="cancel()"
      (keydown)="onKeyDown($event)">
      <section
        class="registry-editor"
        role="dialog"
        aria-modal="true"
        aria-label="Edit Registry Value"
        data-input-owned
        (click)="$event.stopPropagation()"
        (mousedown)="$event.stopPropagation()">
        <header>
          <div>
            <strong>Edit Registry Value</strong>
            <span>{{ request.name }}</span>
          </div>
          <button type="button" class="close" aria-label="Close" (click)="cancel()">×</button>
        </header>

        <form (ngSubmit)="save()" data-input-owned>
          <label>
            <span>Type</span>
            <select
              name="registry-value-type"
              [(ngModel)]="type"
              data-input-owned>
              <option *ngFor="let item of registryTypes" [value]="item">
                {{ item }}
              </option>
            </select>
          </label>

          <label class="data-field">
            <span>Data</span>
            <textarea
              #dataInput
              name="registry-value-data"
              [(ngModel)]="data"
              rows="6"
              spellcheck="false"
              autocomplete="off"
              data-input-owned></textarea>
          </label>

          <footer>
            <button type="button" (click)="cancel()">Cancel</button>
            <button type="submit" class="primary">Save</button>
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

    .registry-editor-overlay {
      position: fixed;
      inset: 0;
      display: grid;
      place-items: center;
      padding: 24px;
      background: var(--vsc-scrim);
      pointer-events: auto;
    }

    .registry-editor {
      box-sizing: border-box;
      width: min(560px, calc(100vw - 48px));
      max-height: calc(100vh - 48px);
      overflow: auto;
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
      gap: 16px;
      padding: 12px 14px;
      border-bottom: 1px solid var(--vsc-border);
    }

    header > div {
      display: grid;
      gap: 2px;
      min-width: 0;
    }

    header span {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
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
      gap: 12px;
      padding: 14px;
      pointer-events: auto;
    }

    label {
      display: grid;
      grid-template-columns: 72px minmax(0, 1fr);
      gap: 10px;
      align-items: center;
    }

    label > span {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
    }

    input,
    select,
    textarea {
      box-sizing: border-box;
      width: 100%;
      border: 1px solid var(--vsc-input-border, var(--vsc-border));
      border-radius: 3px;
      background: var(--vsc-input-background);
      color: var(--vsc-input-foreground, var(--vsc-foreground));
      font: inherit;
      pointer-events: auto;
    }

    input,
    select {
      height: 28px;
      padding: 4px 8px;
    }

    textarea {
      min-height: 120px;
      padding: 7px 8px;
      resize: vertical;
      font-family: var(--vsc-editor-font-family, monospace);
    }

    input:focus,
    select:focus,
    textarea:focus {
      outline: 1px solid var(--vsc-focus-border, var(--vsc-accent-blue));
      outline-offset: -1px;
    }

    .data-field {
      align-items: start;
    }

    .data-field > span {
      padding-top: 7px;
    }

    footer {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
      padding-top: 2px;
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
  `],
})
export class RegistryValueEditorComponent implements OnInit, OnDestroy {
  @ViewChild('dataInput')
  private dataInput?: ElementRef<HTMLTextAreaElement>;

  readonly registryTypes = [
    'REG_SZ',
    'REG_EXPAND_SZ',
    'REG_BINARY',
    'REG_DWORD',
    'REG_DWORD_BIG_ENDIAN',
    'REG_MULTI_SZ',
    'REG_QWORD',
    'REG_NONE',
  ] as const;

  request: RegistryValueEditRequest | null = null;
  type = 'REG_SZ';
  data = '';

  private readonly destroy$ = createSubject<void>();

  constructor(
    private readonly editor: RegistryValueEditorService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    this.editor.active$
      .pipe(takeUntil(this.destroy$))
      .subscribe(active => {
        this.request = active?.request ?? null;
        this.type = active?.request.type ?? 'REG_SZ';
        this.data = active?.request.data ?? '';
        this.cdr.markForCheck();

        if (active) {
          requestAnimationFrame(() => {
            const input = this.dataInput?.nativeElement;
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
    if (!this.request) return;
    this.editor.resolve({
      type: this.type,
      data: this.data,
    });
  }

  cancel(): void {
    this.editor.resolve(null);
  }

  onKeyDown(event: KeyboardEvent): void {
    if (event.key !== 'Escape') return;
    event.preventDefault();
    event.stopPropagation();
    this.cancel();
  }
}