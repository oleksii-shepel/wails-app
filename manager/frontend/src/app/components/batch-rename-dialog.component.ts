import { CommonModule } from '@angular/common';
import { Component, ElementRef, ViewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil, type Stream } from '@epikodelabs/streamix';
import {
  BatchRenameDialogService,
  type BatchRenameItem,
  type BatchRenameRequest,
} from '../services/batch-rename-dialog.service';
import { applyBatchRenameTemplate } from '../services/batch-rename-template';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { moveFocusWithin } from '../utils/focus-navigation.utils';

interface PreviewRow {
  entry: BatchRenameItem['entry'];
  newName: string;
  conflict: boolean;
}

@Component({
  selector: 'app-batch-rename-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule],
  styleUrls: ['./batch-rename-dialog.component.scss'],
  template: `
    <ng-container *ngIf="request$ | async as request">
      <div class="br-overlay" [class.br-overlay--closing]="isClosing" (click)="onCancel()" (animationend)="onAnimationEnd($event)">
        <div
          #dialogPanel
          class="br-panel"
          role="dialog"
          aria-modal="true"
          aria-label="Batch Rename"
          tabindex="-1"
          (keydown.tab)="onTabKey($event)"
          (click)="$event.stopPropagation()"
        >
          <header class="br-header">
            <h2>Batch Rename</h2>
            <span class="br-subtitle">{{ request.items.length }} item(s) selected</span>
          </header>

          <div class="br-body">
            <div class="br-controls">
              <div class="br-field">
                <label for="br-pattern">Pattern</label>
                <input
                  id="br-pattern"
                  class="br-input"
                  type="text"
                  [(ngModel)]="pattern"
                  (input)="onPatternChange()"
                  placeholder="e.g. photo_###&"
                  data-dialog-initial-focus
                />
                <div class="br-hint">
                  <code>*</code>/<code>{{ '{' }}name{{ '}' }}</code> = name,
                  <code>&amp;</code>/<code>{{ '{' }}.ext{{ '}' }}</code> = extension,
                  <code>#</code>/<code>{{ '{' }}n:03{{ '}' }}</code> = counter,
                  <code>{{ '{' }}date{{ '}' }}</code>, <code>{{ '{' }}upper:name{{ '}' }}</code>,
                  <code>{{ '{' }}lower:name{{ '}' }}</code>, <code>{{ '{' }}replace:old=&gt;new{{ '}' }}</code>
                </div>
              </div>

              <div class="br-row">
                <div class="br-field br-field--small">
                  <label for="br-start">Start at</label>
                  <input
                    id="br-start"
                    class="br-input"
                    type="number"
                    min="0"
                    [(ngModel)]="startAt"
                    (input)="onPatternChange()"
                  />
                </div>
                <div class="br-field br-field--small">
                  <label for="br-step">Step</label>
                  <input
                    id="br-step"
                    class="br-input"
                    type="number"
                    min="1"
                    [(ngModel)]="increment"
                    (input)="onPatternChange()"
                  />
                </div>
              </div>
            </div>

            <div class="br-preview">
              <div class="br-preview-header">
                <span>Preview</span>
                <span *ngIf="conflictCount > 0" class="br-conflict-badge">
                  {{ conflictCount }} conflict(s)
                </span>
              </div>
              <div class="br-preview-list">
                <div
                  *ngFor="let row of preview"
                  class="br-preview-row"
                  [class.br-preview-row--conflict]="row.conflict"
                >
                  <span class="br-preview-old" [title]="row.entry.name">{{ row.entry.name }}</span>
                  <span class="br-preview-arrow">
                    <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
                      <path d="M8.22 2.97a.75.75 0 0 1 1.06 0l4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.75.75 0 0 1-1.06-1.06l2.97-2.97H2.75a.75.75 0 0 1 0-1.5h8.44L8.22 4.03a.75.75 0 0 1 0-1.06z"/>
                    </svg>
                  </span>
                  <span class="br-preview-new" [title]="row.newName">{{ row.newName }}</span>
                </div>
              </div>
            </div>
          </div>

          <footer class="br-footer">
            <button
              type="button"
              class="br-btn br-btn--secondary"
              (click)="onCancel()"
            >
              Cancel
            </button>
            <button
              type="button"
              class="br-btn br-btn--primary"
              [disabled]="conflictCount > 0 || preview.length === 0"
              (click)="onApply()"
            >
              Rename
            </button>
          </footer>
        </div>
      </div>
    </ng-container>
  `,
})
export class BatchRenameDialogComponent {
  @ViewChild('dialogPanel') dialogPanel?: ElementRef<HTMLElement>;

  readonly request$: Stream<BatchRenameRequest | null> = this.service.mountedRequest$;

  pattern = '*';
  startAt = 1;
  increment = 1;
  preview: PreviewRow[] = [];
  conflictCount = 0;

  private readonly destroy$ = createSubject<void>();
  private releaseDialogContext: (() => void) | null = null;

  constructor(
    private readonly service: BatchRenameDialogService,
    private readonly inputContexts: InputContextService,
    private readonly inputEngine: InputEngineService,
  ) {
    this.inputEngine.registerCommand('dialog.batchRename.apply', () => this.onApply());
    this.inputEngine.registerCommand('dialog.batchRename.close', () => this.onCancel());
    this.inputEngine.registerCommand('dialog.batchRename.focusNext', () => this.moveDialogFocus(1));
    this.inputEngine.registerCommand('dialog.batchRename.focusPrev', () => this.moveDialogFocus(-1));

    this.service.mountedRequest$.pipe(takeUntil(this.destroy$)).subscribe((req) => {
      if (req) {
        this.pattern = '*';
        this.startAt = 1;
        this.increment = 1;
        this.rebuildPreview(req);
        this.releaseDialogContext ??= this.inputContexts.acquire('dialog.batchRename');
        this.focusInitialControl();
      } else {
        this.releaseDialogContext?.();
        this.releaseDialogContext = null;
      }
    });
  }

  ngOnDestroy(): void {
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.inputEngine.unregisterCommand('dialog.batchRename.apply');
    this.inputEngine.unregisterCommand('dialog.batchRename.close');
    this.inputEngine.unregisterCommand('dialog.batchRename.focusNext');
    this.inputEngine.unregisterCommand('dialog.batchRename.focusPrev');
    this.destroy$.next();
    this.destroy$.complete();
  }

  onTabKey(event: Event): void {
    if (!(event instanceof KeyboardEvent)) {
      return;
    }
    if (!this.dialogPanel?.nativeElement) {
      return;
    }
    moveFocusWithin(this.dialogPanel.nativeElement, event.shiftKey ? -1 : 1);
    event.preventDefault();
    event.stopPropagation();
  }

  onPatternChange(): void {
    const req = this.service.activeRequest$.value;
    if (req) {
      this.rebuildPreview(req);
    }
  }

  private rebuildPreview(req: BatchRenameRequest): void {
    const seen = new Set<string>();
    const rows: PreviewRow[] = [];
    let counter = Math.max(0, Math.floor(Number(this.startAt) || 0));
    const step = Math.max(1, Math.floor(Number(this.increment) || 1));

    // Determine counter width from pattern
    let counterWidth = 1;
    const pat = this.pattern;
    for (let i = 0; i < pat.length; i++) {
      if (pat[i] === '#') {
        let w = 1;
        while (i + w < pat.length && pat[i + w] === '#') w++;
        counterWidth = Math.max(counterWidth, w);
      }
    }

    for (const item of req.items) {
      const newName = applyBatchRenameTemplate(pat, {
        originalName: item.entry.name,
        index: rows.length,
        counter,
        counterWidth,
      });
      const conflict = seen.has(newName) || newName === '';
      seen.add(newName);
      rows.push({ entry: item.entry, newName, conflict });
      counter += step;
    }

    this.preview = rows;
    this.conflictCount = rows.filter((r) => r.conflict).length;
  }

  onCancel(): void {
    this.service.resolveActive(null);
  }

  onApply(): void {
    const req = this.service.activeRequest$.value;
    if (!req || this.conflictCount > 0) return;

    const result: BatchRenameItem[] = this.preview.map((row) => ({
      entry: row.entry,
      newName: row.newName,
    }));
    this.service.resolveActive(result);
  }

  private focusInitialControl(): void {
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

  private moveDialogFocus(delta: number): void {
    if (!this.dialogPanel?.nativeElement) {
      return;
    }
    moveFocusWithin(this.dialogPanel.nativeElement, delta);
  }

  get isClosing(): boolean {
    return !!this.service.closingRequest$.value;
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) {
      return;
    }
    this.service.finishClose();
  }
}
