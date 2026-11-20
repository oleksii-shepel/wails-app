import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, ElementRef, ViewChild } from '@angular/core';
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
  changed: boolean;
}

interface PatternPreset {
  id: string;
  label: string;
  pattern: string;
}

interface PatternTokenChip {
  token: string;
  label: string;
}

const PATTERN_PRESETS: readonly PatternPreset[] = [
  { id: 'original', label: 'Original', pattern: '{name}{.ext}' },
  { id: 'sequential', label: 'Sequential', pattern: '{n:03}_{name}{.ext}' },
  { id: 'date', label: 'Date Prefix', pattern: '{date}_{name}{.ext}' },
  { id: 'lower', label: 'lowercase', pattern: '{lower:name}{.ext}' },
  { id: 'kebab', label: 'kebab-case', pattern: '{kebab:name}{.ext}' },
  { id: 'snake', label: 'snake_case', pattern: '{snake:name}{.ext}' },
  { id: 'title', label: 'Title Case', pattern: '{title:name}{.ext}' },
];

const TOKEN_CHIPS: readonly PatternTokenChip[] = [
  { token: '{name}', label: '{name}' },
  { token: '{.ext}', label: '{.ext}' },
  { token: '{n:03}', label: '{n:03}' },
  { token: '{date}', label: '{date}' },
  { token: '{kebab:name}', label: '{kebab:name}' },
  { token: '{snake:name}', label: '{snake:name}' },
  { token: '{title:name}', label: '{title:name}' },
  { token: '{upper:name}', label: '{upper:name}' },
];

@Component({
  selector: 'app-batch-rename-dialog',
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  styleUrls: ['./batch-rename-dialog.component.scss', './tools-dialog-remote-theme.scss'],
  template: `
    <ng-container *ngIf="request$ | async as request">
      <div class="br-overlay palette-dialog-backdrop dialog-group" [class.br-overlay--closing]="isClosing" [class.dialog-group--closing]="isClosing" (click)="onCancel()" (animationend)="onAnimationEnd($event)">
        <div
          #dialogPanel
          class="br-panel palette-dialog-surface dialog-surface"
          role="dialog"
          aria-modal="true"
          aria-label="Batch Rename"
          tabindex="-1"
          (keydown.tab)="onTabKey($event)"
          (click)="$event.stopPropagation()"
          (keydown.escape)="onCancel(); $event.preventDefault(); $event.stopPropagation()">
          <header class="br-header">
            <div class="br-title-group">
              <h2>Batch Rename</h2>
              <span class="br-subtitle">{{ request.items.length }} selected · {{ changedCount }} changed</span>
            </div>
            <span class="dialog-close" aria-label="Close tab" title="Close (Esc)" (click)="onCancel()" role="button" tabindex="0" (keydown.enter)="onCancel()" (keydown.space)="onCancel(); $event.preventDefault()"> × </span>
          </header>

          <div class="br-body">
            <div class="br-controls">
              <div class="br-presets" role="group" aria-label="Pattern presets">
                <button
                  *ngFor="let preset of presets"
                  type="button"
                  class="br-chip"
                  [class.active]="pattern === preset.pattern"
                  (click)="applyPreset(preset.pattern)">
                  {{ preset.label }}
                </button>
              </div>

              <div class="br-field">
                <label for="br-pattern">Pattern</label>
                <input
                  #patternInput
                  id="br-pattern"
                  class="br-input br-input--mono"
                  type="text"
                  [value]="pattern"
                  (input)="onPatternInput($event)"
                  placeholder="e.g. {date}_{name}{.ext}"
                  data-dialog-initial-focus
                />
                <div class="br-token-row" role="group" aria-label="Insert template token">
                  <span class="br-token-label">Insert:</span>
                  <button
                    *ngFor="let chip of tokenChips"
                    type="button"
                    class="br-token-btn"
                    (click)="insertToken(chip.token)">
                    {{ chip.label }}
                  </button>
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
                    [value]="startAt"
                    (input)="onStartAtInput($event)"
                  />
                </div>
                <div class="br-field br-field--small">
                  <label for="br-step">Step</label>
                  <input
                    id="br-step"
                    class="br-input"
                    type="number"
                    min="1"
                    [value]="increment"
                    (input)="onIncrementInput($event)"
                  />
                </div>
                <div class="br-field br-field--find">
                  <label for="br-find">Find</label>
                  <input
                    id="br-find"
                    class="br-input"
                    type="text"
                    [value]="findText"
                    (input)="onFindInput($event)"
                    placeholder="Text or pattern"
                  />
                </div>
                <div class="br-field br-field--find">
                  <label for="br-replace">Replace with</label>
                  <input
                    id="br-replace"
                    class="br-input"
                    type="text"
                    [value]="replaceText"
                    (input)="onReplaceInput($event)"
                    placeholder="Replacement"
                  />
                </div>
              </div>

              <div class="br-toggle-row">
                <label class="br-check">
                  <input type="checkbox" [checked]="useRegex" (change)="onRegexToggle($event)" />
                  <span>Use Regular Expression</span>
                </label>
                <label class="br-check">
                  <input type="checkbox" [checked]="matchCase" (change)="onMatchCaseToggle($event)" />
                  <span>Match Case</span>
                </label>
                <span class="br-regex-error" *ngIf="regexError">{{ regexError }}</span>
              </div>
            </div>

            <div class="br-preview">
              <div class="br-preview-header">
                <span>Preview · {{ changedCount }} of {{ preview.length }} will change</span>
                <span *ngIf="conflictCount > 0" class="br-conflict-badge">
                  {{ conflictCount }} conflict(s)
                </span>
              </div>
              <div class="br-preview-list">
                <div
                  *ngFor="let row of preview; let i = index"
                  class="br-preview-row"
                  [class.br-preview-row--conflict]="row.conflict"
                  [class.br-preview-row--changed]="row.changed && !row.conflict"
                >
                  <span class="br-preview-index">{{ i + 1 }}</span>
                  <span class="br-preview-old" [title]="row.entry.name">{{ row.entry.name }}</span>
                  <span class="br-preview-arrow" aria-hidden="true">
                    <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
                      <path d="M8.22 2.97a.75.75 0 0 1 1.06 0l4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.75.75 0 0 1-1.06-1.06l2.97-2.97H2.75a.75.75 0 0 1 0-1.5h8.44L8.22 4.03a.75.75 0 0 1 0-1.06z"/>
                    </svg>
                  </span>
                  <span class="br-preview-new" [title]="row.newName">{{ row.newName || '(empty)' }}</span>
                  <span class="br-preview-status" [class.conflict]="row.conflict" [class.changed]="row.changed && !row.conflict">
                    {{ row.conflict ? 'Collision' : row.changed ? 'Renamed' : 'Unchanged' }}
                  </span>
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
              [disabled]="conflictCount > 0 || !!regexError || preview.length === 0 || changedCount === 0"
              (click)="onApply()"
            >
              Rename {{ changedCount > 0 ? changedCount : '' }}
            </button>
          </footer>
        </div>
      </div>
    </ng-container>
  `,
})
export class BatchRenameDialogComponent {
  @ViewChild('dialogPanel') dialogPanel?: ElementRef<HTMLElement>;
  @ViewChild('patternInput') patternInput?: ElementRef<HTMLInputElement>;

  readonly request$: Stream<BatchRenameRequest | null> = this.service.mountedRequest$;
  readonly presets = PATTERN_PRESETS;
  readonly tokenChips = TOKEN_CHIPS;

  pattern = '{name}{.ext}';
  startAt = 1;
  increment = 1;
  findText = '';
  replaceText = '';
  useRegex = false;
  matchCase = false;
  regexError = '';
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
        this.pattern = '{name}{.ext}';
        this.startAt = 1;
        this.increment = 1;
        this.findText = '';
        this.replaceText = '';
        this.useRegex = false;
        this.matchCase = false;
        this.regexError = '';
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

  get changedCount(): number {
    return this.preview.filter((row) => row.changed).length;
  }

  onTabKey(event: Event): void {
    if (!(event instanceof KeyboardEvent) || !this.dialogPanel?.nativeElement) {
      return;
    }
    moveFocusWithin(this.dialogPanel.nativeElement, event.shiftKey ? -1 : 1);
    event.preventDefault();
    event.stopPropagation();
  }

  applyPreset(nextPattern: string): void {
    this.pattern = nextPattern;
    this.onPatternChange();
  }

  insertToken(token: string): void {
    const input = this.patternInput?.nativeElement;
    if (input && typeof input.selectionStart === 'number' && typeof input.selectionEnd === 'number') {
      const start = input.selectionStart;
      const end = input.selectionEnd;
      this.pattern = `${this.pattern.slice(0, start)}${token}${this.pattern.slice(end)}`;
      this.onPatternChange();
      requestAnimationFrame(() => {
        input.focus();
        input.setSelectionRange(start + token.length, start + token.length);
      });
      return;
    }
    this.pattern += token;
    this.onPatternChange();
  }

  onPatternInput(event: Event): void {
    this.pattern = (event.target as HTMLInputElement).value;
    this.onPatternChange();
  }

  onStartAtInput(event: Event): void {
    this.startAt = Number((event.target as HTMLInputElement).value);
    this.onPatternChange();
  }

  onIncrementInput(event: Event): void {
    this.increment = Number((event.target as HTMLInputElement).value);
    this.onPatternChange();
  }

  onFindInput(event: Event): void {
    this.findText = (event.target as HTMLInputElement).value;
    this.onPatternChange();
  }

  onReplaceInput(event: Event): void {
    this.replaceText = (event.target as HTMLInputElement).value;
    this.onPatternChange();
  }

  onRegexToggle(event: Event): void {
    this.useRegex = (event.target as HTMLInputElement).checked;
    this.onPatternChange();
  }

  onMatchCaseToggle(event: Event): void {
    this.matchCase = (event.target as HTMLInputElement).checked;
    this.onPatternChange();
  }

  onPatternChange(): void {
    const req = this.service.activeRequest$.value;
    if (req) {
      this.rebuildPreview(req);
    }
  }

  private compileFinder(): ((input: string) => string) | null {
    this.regexError = '';
    if (!this.findText) {
      return null;
    }

    if (this.useRegex) {
      try {
        const flags = this.matchCase ? 'g' : 'gi';
        const rx = new RegExp(this.findText, flags);
        return (value: string) => value.replace(rx, this.replaceText);
      } catch (err) {
        this.regexError = err instanceof Error ? err.message : 'Invalid regular expression';
        return null;
      }
    }

    const needle = this.findText;
    if (this.matchCase) {
      return (value: string) => value.split(needle).join(this.replaceText);
    }
    const escaped = needle.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const rx = new RegExp(escaped, 'gi');
    return (value: string) => value.replace(rx, this.replaceText);
  }

  private rebuildPreview(req: BatchRenameRequest): void {
    const seen = new Set<string>();
    const rows: PreviewRow[] = [];
    let counter = Math.max(0, Math.floor(Number(this.startAt) || 0));
    const step = Math.max(1, Math.floor(Number(this.increment) || 1));

    let counterWidth = 1;
    const pat = this.pattern;
    for (let i = 0; i < pat.length; i++) {
      if (pat[i] === '#') {
        let w = 1;
        while (i + w < pat.length && pat[i + w] === '#') w++;
        counterWidth = Math.max(counterWidth, w);
      }
    }

    const replacer = this.compileFinder();

    for (const item of req.items) {
      let newName = applyBatchRenameTemplate(pat, {
        originalName: item.entry.name,
        index: rows.length,
        counter,
        counterWidth,
      });
      if (replacer) {
        newName = replacer(newName);
      }
      const conflict = seen.has(newName) || newName.trim() === '';
      seen.add(newName);
      rows.push({
        entry: item.entry,
        newName,
        conflict,
        changed: newName !== item.entry.name,
      });
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
    if (!req || this.conflictCount > 0 || this.regexError || this.changedCount === 0) return;

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