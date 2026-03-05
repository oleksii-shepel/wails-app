import {
  Component, Input, Output, EventEmitter, ViewChild, ElementRef,
  OnInit, OnDestroy, OnChanges, SimpleChanges, ChangeDetectionStrategy, ChangeDetectorRef,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { EMPTY, createSubject, debounce, distinctUntilChanged, filter, fromAny, merge, switchMap, takeUntil } from '@epikodelabs/streamix';
import { PathSuggestion } from '../services/path-history.service';
import { PathHistoryService } from '../services/path-history.service';
import { ApiService } from '../services/api.service';
import { FileType } from '../shared/protocol';
import { highlightMatch, isWindowsPath, normalizePath } from './address-bar.utils';

interface TabSession {
  candidates: PathSuggestion[];
  index: number;
  enteredDir: boolean;
}

@Component({
  selector: 'address-bar-edit',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="ab-wrap">

      <!-- ── Input row ── -->
      <div class="ab-edit" [class.ab-edit--open]="suggestions.length > 0">

        <!-- Folder SVG icon -->
        <span class="ab-icon" aria-hidden="true">
          <svg width="15" height="15" viewBox="0 0 15 15" fill="none">
            <path d="M1 4C1 3.45 1.45 3 2 3H5.5L7 4.5H13C13.55 4.5 14 4.95 14 5.5v7C14 13.1 13.55 13.5 13 13.5H2C1.45 13.5 1 13.1 1 12.5V4z"
                  fill="var(--vsc-folder-fill, #dcb67a)"/>
          </svg>
        </span>

        <input
          #editInput
          class="ab-input"
          type="text"
          [(ngModel)]="value"
          (ngModelChange)="onValueChange($event)"
          (paste)="onPaste($event)"
          (dragover)="onDragOver($event)"
          (drop)="onDrop($event)"
          (keydown)="onKeyDown($event)"
          (blur)="onBlur()"
          [placeholder]="placeholder"
          autocomplete="off"
          spellcheck="false"
        />

        <button
          class="ab-cancel"
          (click)="cancelEdit()"
          title="Cancel (Esc)"
          aria-label="Cancel"
        >
          <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
            <path d="M1 1l8 8M9 1L1 9" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <!-- ── Autocomplete dropdown ── -->
      <div
        class="ab-dropdown"
        *ngIf="suggestions.length > 0"
        role="listbox"
        aria-label="Path suggestions"
        (mousedown)="$event.preventDefault()"
      >
        <!-- Top hairline separator -->
        <div class="ab-separator"></div>

        <div class="ab-scroll">

          <!-- Group: filesystem directories -->
          <ng-container *ngIf="dirSuggestions.length > 0">
            <div class="ab-group-label" role="presentation">
              <svg width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
                <path d="M0.5 2C0.5 1.45 0.95 1 1.5 1H3.5L4.5 2H8.5C9.05 2 9.5 2.45 9.5 3v5.5C9.5 9.05 9.05 9.5 8.5 9.5H1.5C0.95 9.5 0.5 9.05 0.5 8.5V2z"
                      fill="var(--vsc-folder-fill, #dcb67a)" fill-opacity="0.85"/>
              </svg>
              Directories
              <span class="ab-group-rule"></span>
            </div>

            <div
              *ngFor="let s of dirSuggestions; let i = index"
              class="ab-item"
              role="option"
              [attr.aria-selected]="acIdx === globalIndex('dir', i)"
              [class.ab-item--focused]="acIdx === globalIndex('dir', i)"
              (mouseenter)="acIdx = globalIndex('dir', i)"
              (mousedown)="acceptSuggestion(s, $event)"
            >
              <span class="ab-item-icon" aria-hidden="true">
                <svg width="13" height="13" viewBox="0 0 13 13" fill="none">
                  <path d="M1 3C1 2.45 1.45 2 2 2H4.8L6 3.2H11C11.55 3.2 12 3.65 12 4.2v6.3C12 11.05 11.55 11.5 11 11.5H2C1.45 11.5 1 11.05 1 10.5V3z"
                        fill="var(--vsc-folder-fill, #dcb67a)"/>
                </svg>
              </span>
              <span class="ab-item-path" [innerHTML]="highlight(s.path, value)"></span>
              <span class="ab-item-badge ab-item-badge--dir">dir</span>
            </div>
          </ng-container>

          <!-- Group: history / recent -->
          <ng-container *ngIf="histSuggestions.length > 0">
            <div class="ab-group-label" [class.ab-group-label--spaced]="dirSuggestions.length > 0" role="presentation">
              <svg width="10" height="10" viewBox="0 0 10 10" fill="none" aria-hidden="true">
                <circle cx="5" cy="5" r="4.25" stroke="var(--vsc-accent-blue, #007acc)" stroke-width="1.1"/>
                <path d="M5 2.5v2.8l1.8 1" stroke="var(--vsc-accent-blue, #007acc)" stroke-width="1.1" stroke-linecap="round"/>
              </svg>
              Recent
              <span class="ab-group-rule"></span>
            </div>

            <div
              *ngFor="let s of histSuggestions; let i = index"
              class="ab-item"
              role="option"
              [attr.aria-selected]="acIdx === globalIndex('hist', i)"
              [class.ab-item--focused]="acIdx === globalIndex('hist', i)"
              (mouseenter)="acIdx = globalIndex('hist', i)"
              (mousedown)="acceptSuggestion(s, $event)"
            >
              <span class="ab-item-icon ab-item-icon--hist" aria-hidden="true">
                <svg width="13" height="13" viewBox="0 0 13 13" fill="none">
                  <circle cx="6.5" cy="6.5" r="5.25" stroke="var(--vsc-accent-blue, #007acc)" stroke-width="1.1"/>
                  <path d="M6.5 3.5v3.2l2 1.1" stroke="var(--vsc-accent-blue, #007acc)" stroke-width="1.1" stroke-linecap="round"/>
                </svg>
              </span>
              <span class="ab-item-path" [innerHTML]="highlight(s.path, value)"></span>
              <span class="ab-item-badge ab-item-badge--hist">recent</span>
            </div>
          </ng-container>

        </div>

        <!-- Keyboard hint footer -->
        <div class="ab-footer" aria-hidden="true">
          <span class="ab-hint"><kbd>↑↓</kbd> navigate</span>
          <span class="ab-hint"><kbd>↵</kbd> open</span>
          <span class="ab-hint"><kbd>Tab</kbd> complete</span>
          <span class="ab-hint"><kbd>Esc</kbd> cancel</span>
        </div>
      </div>

    </div>
  `,
  styles: [`
    :host { display: block; width: 100%; }

    /* ── Wrapper ────────────────────────────────────────────── */
    .ab-wrap {
      position: relative;
      width: 100%;
    }

    /* ── Input row ──────────────────────────────────────────── */
    .ab-edit {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm, 4px);
      height: var(--vsc-titlebar-height, 35px);
      padding: 0 var(--vsc-padding-sm, 4px) 0 var(--vsc-padding-md, 8px);
      background: var(--vsc-input-background, #3c3c3c);
      border: 1px solid var(--vsc-input-border, #3c3c3c);
      border-radius: var(--vsc-border-radius, 3px);
      box-sizing: border-box;
      width: 100%;
      transition:
        border-color var(--vsc-transition-fast, 80ms ease),
        box-shadow   var(--vsc-transition-fast, 80ms ease);

      &:focus-within {
        border-color: var(--vsc-input-focus-border, #007acc);
        box-shadow: 0 0 0 1px color-mix(in srgb, var(--vsc-input-focus-border, #007acc) 30%, transparent);
      }
    }

    /* When dropdown is open: square off the bottom corners and dissolve the
       bottom border so the dropdown reads as a continuous surface. */
    .ab-edit.ab-edit--open {
      border-bottom-left-radius: 0;
      border-bottom-right-radius: 0;
      border-bottom-color: var(--vsc-panel-background, #252526);
    }

    .ab-edit.ab-edit--open:focus-within {
      border-bottom-color: var(--vsc-input-focus-border, #007acc);
    }

    /* Folder icon */
    .ab-icon {
      flex-shrink: 0;
      width: 16px;
      height: 16px;
      display: grid;
      place-items: center;
      opacity: 0.82;
    }

    /* Monospace path input */
    .ab-input {
      flex: 1;
      min-width: 0;
      background: transparent;
      border: none;
      outline: none;
      color: var(--vsc-input-foreground, #cccccc);
      font-family: var(--vsc-font-family-mono, Consolas, monospace);
      font-size: var(--vsc-font-size, 13px);
      caret-color: var(--vsc-input-focus-border, #007acc);
      padding: 0;
      line-height: 1;

      &::placeholder {
        color: var(--vsc-input-placeholder, #767676);
        opacity: 1;
        font-family: var(--vsc-font-family, sans-serif);
        font-size: var(--vsc-font-size-small, 12px);
        letter-spacing: 0.01em;
      }
    }

    /* Cancel button */
    .ab-cancel {
      flex-shrink: 0;
      width: 22px;
      height: 22px;
      display: grid;
      place-items: center;
      padding: 0;
      background: transparent;
      border: none;
      border-radius: var(--vsc-border-radius-sm, 2px);
      cursor: pointer;
      color: var(--vsc-foreground-dim, #888888);
      transition:
        background var(--vsc-transition-fast, 80ms ease),
        color      var(--vsc-transition-fast, 80ms ease);

      &:hover {
        background: var(--vsc-error, #f48771);
        color: var(--vsc-foreground-bright, #ffffff);
      }
    }

    /* ── Dropdown ───────────────────────────────────────────── */
    .ab-dropdown {
      position: absolute;
      left: 0;
      right: 0;
      top: calc(100% - 1px); /* overlap the dissolved bottom border */
      background: var(--vsc-panel-background, #252526);
      border: 1px solid var(--vsc-input-border, #3c3c3c);
      border-top: none;
      border-bottom-left-radius:  var(--vsc-border-radius, 3px);
      border-bottom-right-radius: var(--vsc-border-radius, 3px);
      box-shadow: var(--vsc-widget-shadow, 0 5px 8px rgba(0,0,0,.36));
      z-index: var(--vsc-z-dropdown, 1000);
      overflow: hidden;
      transform-origin: top center;
      animation: ab-appear 110ms cubic-bezier(0.2, 0, 0.13, 1) both;
    }

    /* Inherit focus ring colour on the dropdown border */
    .ab-wrap:focus-within .ab-dropdown {
      border-color: var(--vsc-input-focus-border, #007acc);
    }

    @keyframes ab-appear {
      from { opacity: 0; transform: scaleY(0.93) translateY(-3px); }
      to   { opacity: 1; transform: scaleY(1)    translateY(0);    }
    }

    /* Hairline separator between input and first item */
    .ab-separator {
      height: 1px;
      margin: 0 10px;
      background: var(--vsc-border, #454545);
      opacity: 0.5;
    }

    /* Scroll area */
    .ab-scroll {
      max-height: 272px;
      overflow-y: auto;
      padding: 3px 0 4px;
      scrollbar-width: thin;
      scrollbar-color: var(--vsc-scrollbar-thumb, #424242) transparent;

      &::-webkit-scrollbar       { width: 6px; }
      &::-webkit-scrollbar-track { background: transparent; }
      &::-webkit-scrollbar-thumb {
        background: var(--vsc-scrollbar-thumb, #424242);
        border-radius: 3px;
        &:hover { background: var(--vsc-scrollbar-thumb-hover, #4f4f4f); }
      }
    }

    /* ── Group label ────────────────────────────────────────── */
    .ab-group-label {
      display: flex;
      align-items: center;
      gap: 5px;
      height: 20px;
      padding: 0 10px;
      font-size: 10px;
      font-weight: 600;
      letter-spacing: 0.07em;
      text-transform: uppercase;
      color: var(--vsc-foreground-dim, #888888);
      opacity: 0.75;
      cursor: default;
      user-select: none;

      &.ab-group-label--spaced { margin-top: 4px; }
    }

    .ab-group-rule {
      flex: 1;
      height: 1px;
      background: var(--vsc-border, #454545);
      opacity: 0.4;
    }

    /* ── Suggestion row ─────────────────────────────────────── */
    .ab-item {
      display: flex;
      align-items: center;
      height: 26px;
      padding: 0 10px;
      gap: 0;
      cursor: pointer;
      border-left: 2px solid transparent;
      user-select: none;
      transition:
        background     var(--vsc-transition-fast, 80ms ease),
        border-color   var(--vsc-transition-fast, 80ms ease);

      &:hover {
        background: var(--vsc-list-hover-background, #2a2d2e);
      }

      &.ab-item--focused {
        background: var(--vsc-list-active-background, #094771);
        border-left-color: var(--vsc-accent-blue, #007acc);

        .ab-item-badge { opacity: 0.55; }
      }
    }

    /* Icon cell */
    .ab-item-icon {
      flex-shrink: 0;
      width: 20px;
      height: 20px;
      display: grid;
      place-items: center;
      margin-right: 6px;
    }

    /* Monospace path text */
    .ab-item-path {
      flex: 1;
      min-width: 0;
      font-family: var(--vsc-font-family-mono, Consolas, monospace);
      font-size: var(--vsc-font-size, 13px);
      color: var(--vsc-foreground, #cccccc);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    /* Match highlight — blue tint, no yellow */
    :host ::ng-deep .ab-item-path mark {
      background: color-mix(in srgb, var(--vsc-accent-blue, #007acc) 22%, transparent);
      color: #569cd6;
      font-weight: 700;
      border-radius: 2px;
      padding: 0 1px;
      /* color-mix fallback for older browsers */
      @supports not (background: color-mix(in srgb, red 1%, blue)) {
        background: var(--vsc-list-focus-background, #062f4a);
      }
    }

    /* Type badge pill */
    .ab-item-badge {
      flex-shrink: 0;
      margin-left: 10px;
      display: inline-flex;
      align-items: center;
      padding: 1px 5px;
      border-radius: 3px;
      font-size: 10px;
      font-weight: 600;
      letter-spacing: 0.04em;
      line-height: 14px;
      transition: opacity var(--vsc-transition-fast, 80ms ease);

      &.ab-item-badge--dir {
        background: color-mix(in srgb, var(--vsc-folder-fill, #dcb67a) 16%, transparent);
        color: var(--vsc-folder-fill, #dcb67a);
        @supports not (background: color-mix(in srgb, red 1%, blue)) {
          background: rgba(220, 182, 122, 0.16);
        }
      }

      &.ab-item-badge--hist {
        background: color-mix(in srgb, var(--vsc-accent-blue, #007acc) 16%, transparent);
        color: color-mix(in srgb, var(--vsc-accent-blue, #007acc) 80%, #fff);
        @supports not (background: color-mix(in srgb, red 1%, blue)) {
          background: rgba(0, 122, 204, 0.16);
          color: #7aaccc;
        }
      }
    }

    /* ── Keyboard hint footer ───────────────────────────────── */
    .ab-footer {
      display: flex;
      align-items: center;
      gap: 12px;
      height: 22px;
      padding: 0 10px;
      border-top: 1px solid var(--vsc-border, #454545);
      opacity: 0.55;
    }

    .ab-hint {
      display: flex;
      align-items: center;
      gap: 4px;
      font-size: 11px;
      color: var(--vsc-foreground-dim, #888888);
      white-space: nowrap;
    }

    kbd {
      display: inline-block;
      padding: 1px 4px;
      background: color-mix(in srgb, var(--vsc-foreground, #cccccc) 8%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 80%, transparent);
      border-bottom-width: 2px;
      border-radius: 2px;
      font-family: var(--vsc-font-family-mono, Consolas, monospace);
      font-size: 10px;
      color: var(--vsc-foreground-dim, #888888);
      line-height: 14px;
    }
  `],
})
export class AddressBarEditComponent implements OnInit, OnDestroy, OnChanges {
  @ViewChild('editInput') editInputRef?: ElementRef<HTMLInputElement>;

  @Input() currentPath = '';
  @Input() placeholder = 'Enter path…';
  @Input() focusOnInit = false;

  @Output() commit = new EventEmitter<string>();
  @Output() cancel = new EventEmitter<void>();

  value = '';
  suggestions: PathSuggestion[] = [];
  acIdx = -1;

  private blurTimer: ReturnType<typeof setTimeout> | undefined;
  private readonly destroy$ = createSubject<void>();
  private tabSession: TabSession | null = null;
  private applyingTab = false;
  private lastInputValue = '';

  private suggestionsTimer: ReturnType<typeof setTimeout> | undefined;
  private suggestionsRequestId = 0;

  private idleSlashTimer: ReturnType<typeof setTimeout> | undefined;
  private idleSlashLastValue = '';

  highlight = highlightMatch;

  /** Filesystem directory suggestions (shown in the "Directories" group). */
  get dirSuggestions(): PathSuggestion[] {
    return this.suggestions.filter(s => s.matchType === 'directory');
  }

  /** History / recent suggestions (shown in the "Recent" group). */
  get histSuggestions(): PathSuggestion[] {
    return this.suggestions.filter(s => s.matchType !== 'directory');
  }

  /**
   * Maps a within-group index back to the flat `suggestions` index used by
   * acIdx, so keyboard navigation and mouse hover stay in sync.
   */
  globalIndex(group: 'dir' | 'hist', localIndex: number): number {
    if (group === 'dir') {
      const item = this.dirSuggestions[localIndex];
      return item ? this.suggestions.indexOf(item) : -1;
    }
    const item = this.histSuggestions[localIndex];
    return item ? this.suggestions.indexOf(item) : -1;
  }

  constructor(
    private pathHistoryService: PathHistoryService,
    private apiService: ApiService,
    private cdr: ChangeDetectorRef,
    private hostEl: ElementRef,
  ) {}

  ngOnInit(): void {
    this.value = this.toDisplayPath(this.currentPath || '');
    this.lastInputValue = this.value;

    // Keep existing UX: show suggestions shortly after entering edit mode.
    if (this.value?.trim()) this.scheduleSuggestions(this.value, true);

    document.addEventListener('mousedown', this.onOutsideClick, { capture: true });
  }

  ngOnDestroy(): void {
    clearTimeout(this.blurTimer);
    clearTimeout(this.suggestionsTimer);
    clearTimeout(this.idleSlashTimer);
    this.destroy$.next();
    this.destroy$.complete();
    document.removeEventListener('mousedown', this.onOutsideClick, { capture: true } as any);
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['focusOnInit']?.currentValue) {
      setTimeout(() => {
        const el = this.editInputRef?.nativeElement;
        if (el) { el.focus(); el.select(); }
      }, 30);
    }

    if (changes['currentPath'] && !changes['currentPath'].firstChange) {
      const next = changes['currentPath'].currentValue || '';
      // Mirror previous behavior from AddressBarComponent: when the underlying
      // path changes while editing, update the edit value too.
      this.writeProgrammaticValue(this.toDisplayPath(next));
      this.cdr.detectChanges();
    }
  }

  onValueChange(v: string): void {
    const prev = this.lastInputValue;
    const el = this.editInputRef?.nativeElement;
    const selStart = el?.selectionStart ?? null;
    const selEnd = el?.selectionEnd ?? null;

    const next = this.maybeFixSlashesWhileTyping(v);
    this.value = next;
    this.lastInputValue = next;

    if (next !== v && el && selStart !== null && selEnd !== null) {
      // Keep caret stable; replacement is 1:1 so positions remain valid.
      setTimeout(() => {
        try { el.setSelectionRange(selStart, selEnd); } catch {}
      }, 0);
    }
    if (!this.applyingTab) {
      this.tabSession = null;

      // Show filesystem suggestions immediately when the user types a path separator
      // (or when the path becomes a drive root like "C:/").
      const prevClean = this.maybeFixSlashesWhileTyping(prev || '');
      const isDriveRootNow = /^[A-Za-z]:\/$/.test(next);
      const wasBeforeDriveRoot = /^[A-Za-z]:$/.test(prevClean);
      const justTypedSeparator = next.endsWith('/') && !prevClean.endsWith('/');

      const immediate = (isDriveRootNow && wasBeforeDriveRoot) || justTypedSeparator;
      this.scheduleSuggestions(next, immediate);
      this.scheduleIdleSlash(next);
    }
  }

  onPaste(e: ClipboardEvent): void {
    const el = this.editInputRef?.nativeElement;
    const raw = e.clipboardData?.getData('text') ?? '';
    if (!el || !raw) return;

    e.preventDefault();

    const insertText = this.normalizePastedPath(raw);
    const start = el.selectionStart ?? this.value.length;
    const end = el.selectionEnd ?? start;

    const before = (this.value || '').slice(0, start);
    const after = (this.value || '').slice(end);
    const nextRaw = before + insertText + after;
    const next = this.maybeFixSlashesWhileTyping(nextRaw);

    this.value = next;
    this.lastInputValue = next;
    this.tabSession = null;
    this.scheduleSuggestions(next, true);
    this.scheduleIdleSlash(next);
    this.cdr.detectChanges();

    const caret = before.length + insertText.length;
    setTimeout(() => {
      try { el.setSelectionRange(caret, caret); } catch {}
    }, 0);
  }

  onDragOver(e: DragEvent): void {
    // Allow dropping text into the input.
    e.preventDefault();
  }

  onDrop(e: DragEvent): void {
    const el = this.editInputRef?.nativeElement;
    if (!el) return;

    e.preventDefault();

    const dt = e.dataTransfer;
    if (!dt) return;

    // Prefer explicit text. Some shells/webviews provide a file list.
    let raw = dt.getData('text/plain') || dt.getData('text') || '';

    if (!raw && dt.files && dt.files.length > 0) {
      const f: any = dt.files[0] as any;
      if (typeof f?.path === 'string') raw = f.path;
      else if (typeof f?.name === 'string') raw = f.name;
    }

    if (!raw) return;

    const insertText = this.normalizePastedPath(raw);
    const start = el.selectionStart ?? this.value.length;
    const end = el.selectionEnd ?? start;

    const before = (this.value || '').slice(0, start);
    const after = (this.value || '').slice(end);
    const nextRaw = before + insertText + after;
    const next = this.maybeFixSlashesWhileTyping(nextRaw);

    this.value = next;
    this.lastInputValue = next;
    this.tabSession = null;
    this.scheduleSuggestions(next, true);
    this.scheduleIdleSlash(next);
    this.cdr.detectChanges();

    const caret = before.length + insertText.length;
    setTimeout(() => {
      try { el.setSelectionRange(caret, caret); } catch {}
    }, 0);
  }

  onBlur(): void {
    clearTimeout(this.blurTimer);
    this.blurTimer = setTimeout(() => this.cancelEdit(), 170);
  }

  cancelEdit(): void {
    clearTimeout(this.blurTimer);
    this.cancel.emit();
  }

  async onKeyDown(e: KeyboardEvent): Promise<void> {
    switch (e.key) {
      case 'Enter': {
        e.preventDefault();
        if (this.acIdx >= 0 && this.suggestions[this.acIdx]) {
          this.acceptSuggestion(this.suggestions[this.acIdx]);
        } else {
          const val = normalizePath(this.value);
          if (val) this.commit.emit(val);
          else this.cancelEdit();
        }
        break;
      }
      case 'Escape':
        e.preventDefault();
        this.cancelEdit();
        break;
      case 'ArrowDown':
        e.preventDefault();
        this.acIdx = Math.min(this.acIdx + 1, this.suggestions.length - 1);
        break;
      case 'ArrowUp':
        e.preventDefault();
        this.acIdx = Math.max(this.acIdx - 1, -1);
        break;
      case 'Tab':
        e.preventDefault();
        // UX: first Tab on an empty input inserts the current drive root.
        if (!(this.value || '').trim()) {
          const root = this.getDefaultRootForTab();
          if (root) {
            this.writeProgrammaticValue(root);
            this.tabSession = null;
            this.scheduleSuggestions(root, true);
            this.scheduleIdleSlash(root);
            this.cdr.detectChanges();
            break;
          }
        }
        await this.cycleTab(e.shiftKey ? -1 : 1);
        break;
    }
  }

  private getDefaultRootForTab(): string | null {
    const cp = (this.currentPath || '').trim();
    if (/^[A-Za-z]:\//.test(cp)) {
      return cp.slice(0, 2).toUpperCase() + '/';
    }
    return null;
  }

  acceptSuggestion(s: PathSuggestion, e?: MouseEvent): void {
    e?.preventDefault();
    clearTimeout(this.blurTimer);
    const val = normalizePath(s.path);
    if (val) this.commit.emit(val);
  }

  private shouldNormalizeToForwardSlashes(): boolean {
    if (isWindowsPath(this.currentPath)) return true;
    // If user started typing a Windows-ish path, treat as Windows even if
    // currentPath isn't (e.g. editing from Quick Access / empty).
    const raw = (this.value || '').trim();
    if (/^[A-Za-z]:/.test(raw)) return true;
    if (raw.startsWith('\\\\')) return true;
    if (raw.startsWith('//')) return true;
    return false;
  }

  private toDisplayPath(path: string): string {
    if (!path) return '';
    if (!this.shouldNormalizeToForwardSlashes()) return path;
    return path.replace(/\\/g, '/');
  }

  private maybeFixSlashesWhileTyping(v: string): string {
    if (!this.shouldNormalizeToForwardSlashes()) return v;
    // Convert any backslashes to forward slashes as the user types.
    return (v || '').replace(/\\/g, '/');
  }

  private normalizePastedPath(text: string): string {
    let t = (text || '').replace(/\r?\n/g, '').trim();
    if ((t.startsWith('"') && t.endsWith('"')) || (t.startsWith("'") && t.endsWith("'"))) {
      t = t.slice(1, -1);
    }

    // Explorer (and some tools) may copy extended-length paths like \\?\C:\...
    if (t.startsWith('\\\\?\\')) t = t.slice('\\\\?\\'.length);
    if (t.startsWith('\\\\.\\')) t = t.slice('\\\\.\\'.length);

    // Always normalize backslashes for pasted content.
    return t.replace(/\\/g, '/');
  }

  private scheduleSuggestions(query: string, immediate: boolean): void {
    clearTimeout(this.suggestionsTimer);
    const requestId = ++this.suggestionsRequestId;
    const delay = immediate ? 0 : 240;

    this.suggestionsTimer = setTimeout(async () => {
      const q = (query || '').trim();
      if (!q) {
        this.suggestions = [];
        this.acIdx = -1;
        this.cdr.detectChanges();
        return;
      }

      try {
        const suggestions = await this.loadTypingSuggestions(query);
        if (requestId !== this.suggestionsRequestId) return;
        this.suggestions = suggestions || [];
        this.acIdx = -1;
        this.cdr.detectChanges();
      } catch {
        if (requestId !== this.suggestionsRequestId) return;
        this.suggestions = [];
        this.acIdx = -1;
        this.cdr.detectChanges();
      }
    }, delay);
  }

  private scheduleIdleSlash(value: string): void {
    clearTimeout(this.idleSlashTimer);
    this.idleSlashLastValue = value;

    this.idleSlashTimer = setTimeout(() => {
      if (this.tabSession?.enteredDir) return;

      const v = (this.idleSlashLastValue || '').trim();
      const n = normalizePath(v);
      if (!v) return;
      if (n === '/' || n.endsWith('/')) return;
      if (/^[A-Za-z]:\/$/.test(n)) return;

      void this.tryAppendSlash(v);
    }, 1000);
  }

  private onOutsideClick = (e: MouseEvent): void => {
    if (!this.hostEl.nativeElement.contains(e.target as Node)) {
      this.cancelEdit();
    }
  };

  private async cycleTab(direction: 1 | -1): Promise<void> {
    if (!this.tabSession || this.tabSession.candidates.length === 0) {
      const candidates = await this.fetchChildren(this.value);
      this.tabSession  = { candidates, index: -1, enteredDir: false };
    }

    if (this.tabSession.candidates.length === 0) return;

    const currentIndex = this.findSessionIndex(this.value);

    direction > 0
      ? this.tabForward(currentIndex)
      : await this.tabBackward(currentIndex);
  }

  private tabForward(currentIndex: number): void {
    const session   = this.tabSession!;
    const nextIndex = session.enteredDir
      ? 0
      : (currentIndex + 1) % session.candidates.length;

    this.applyCandidate(session.candidates[nextIndex], session.candidates, false);
  }

  private async tabBackward(currentIndex: number): Promise<void> {
    const session = this.tabSession!;

    if (session.enteredDir) {
      const withoutSlash = this.value.replace(/\/+$/, '');
      const lastSlash    = withoutSlash.lastIndexOf('/');
      const parentDir    = lastSlash >= 0 ? withoutSlash.slice(0, lastSlash + 1) : '/';

      const siblings   = await this.fetchChildren(parentDir);
      const landingIdx = siblings.findIndex(
        c => normalizePath(c.path) === normalizePath(withoutSlash),
      );
      const idx = landingIdx >= 0 ? landingIdx : 0;

      this.writeTabValue(withoutSlash);
      this.tabSession  = { candidates: siblings, index: idx, enteredDir: false };
      this.suggestions = siblings;
      this.acIdx       = idx;
      this.cdr.detectChanges();

      this.scheduleIdleSlash(this.value);
      return;
    }

    const prevIndex = currentIndex <= 0
      ? session.candidates.length - 1
      : currentIndex - 1;

    this.applyCandidate(session.candidates[prevIndex], session.candidates, false);
  }

  private applyCandidate(
    candidate: PathSuggestion,
    sessionCandidates: PathSuggestion[],
    enteredDir: boolean,
  ): void {
    if (!candidate) return;
    const index = sessionCandidates.indexOf(candidate);

    this.writeTabValue(this.toDisplayPath(candidate.path));
    this.tabSession  = { candidates: sessionCandidates, index, enteredDir };
    this.suggestions = sessionCandidates;
    this.acIdx       = index;
    this.cdr.detectChanges();

    this.scheduleIdleSlash(this.value);
  }

  private writeTabValue(value: string): void {
    this.applyingTab = true;
    this.value       = value;
    this.lastInputValue = value;
    this.applyingTab = false;
  }

  private writeProgrammaticValue(value: string): void {
    this.applyingTab = true;
    this.value       = value;
    this.lastInputValue = value;
    this.applyingTab = false;
  }

  private findSessionIndex(editValue: string): number {
    const clean = normalizePath(editValue);
    return (this.tabSession?.candidates ?? []).findIndex(
      c => normalizePath(c.path) === clean,
    );
  }

  private async tryAppendSlash(value: string): Promise<void> {
    const children = await this.fetchChildren(value + '/');
    if (children.length === 0) return;

    const withSlash = value.endsWith('/') ? value : value + '/';

    this.writeTabValue(this.toDisplayPath(withSlash));
    this.tabSession  = { candidates: children, index: -1, enteredDir: true };
    this.suggestions = children;
    this.acIdx       = -1;
    this.cdr.detectChanges();
  }

  private async fetchChildren(query: string): Promise<PathSuggestion[]> {
    const { dirPath, prefix } = this.splitQueryContext(query);
    if (!dirPath) return [];

    try {
      const listing     = await this.apiService.listDirectory(dirPath, false);
      const prefixLower = prefix.toLowerCase();

      return listing.entries
        .filter(e =>
          e.type === FileType.DIRECTORY &&
          e.name.toLowerCase().startsWith(prefixLower),
        )
        .map(e => ({
          path:        this.toDisplayPath(this.joinPath(dirPath, e.name)),
          displayName: e.name,
          matchType:   'directory' as const,
          score:       100,
          exists:      true,
        }))
        .sort((a, b) => a.path.localeCompare(b.path));
    } catch {
      return [];
    }
  }

  private splitQueryContext(query: string): { dirPath: string; prefix: string } {
    const q = (query || '').replace(/\\/g, '/');
    if (!q) return { dirPath: '/', prefix: '' };
    if (q.endsWith('/')) return { dirPath: q, prefix: '' };

    const lastSlash = q.lastIndexOf('/');
    if (lastSlash >= 0) {
      return { dirPath: q.slice(0, lastSlash + 1), prefix: q.slice(lastSlash + 1) };
    }
    if (/^[A-Za-z]:$/.test(q)) return { dirPath: `${q}/`, prefix: '' };
    return { dirPath: '/', prefix: q };
  }

  private joinPath(dirPath: string, name: string): string {
    const clean  = dirPath.replace(/\/+$/, '');
    const joined = `${clean}/${name}`.replace(/\\/g, '/');
    if (joined.startsWith('//')) return `//${joined.slice(2).replace(/\/{2,}/g, '/')}`;
    return joined.replace(/\/{2,}/g, '/');
  }

  private shouldFetchFsSuggestions(query: string): boolean {
    const q = (query || '').trim();
    if (!q) return false;

    const n = normalizePath(q);
    if (!n) return false;

    // Avoid listing the full filesystem root on short inputs.
    if (n === '/') return false;

    // Any slash-based path, or Windows drive prefix, should show dir suggestions.
    if (n.includes('/')) return true;
    if (/^[A-Za-z]:/.test(n)) return true;

    return false;
  }

  private async loadTypingSuggestions(query: string): Promise<PathSuggestion[]> {
    const q = query || '';

    const [fs, hist] = await Promise.all([
      this.shouldFetchFsSuggestions(q) ? this.fetchChildren(q) : Promise.resolve([]),
      this.pathHistoryService.getSuggestions(q, this.currentPath).catch(() => []),
    ]);

    // Merge + de-dupe by normalized path.
    const seen = new Set<string>();
    const merged: PathSuggestion[] = [];

    for (const s of [...fs, ...(hist as any as PathSuggestion[])]) {
      const key = normalizePath(s?.path || '').toLowerCase();
      if (!key || seen.has(key)) continue;
      seen.add(key);
      merged.push(this.shouldNormalizeToForwardSlashes() ? { ...s, path: this.toDisplayPath(s.path) } : s);
    }

    return merged;
  }
}