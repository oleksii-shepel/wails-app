import {
  Component, Input, Output, EventEmitter, ViewChild, ElementRef,
  OnInit, OnDestroy, OnChanges, SimpleChanges, ChangeDetectionStrategy, ChangeDetectorRef,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { createSubject } from '@epikodelabs/streamix';
import { PathSuggestion } from '../services/path-history.service';
import { PathHistoryService } from '../services/path-history.service';
import { ApiService } from '../services/api.service';
import { DirectorySourceService } from '../services/directory-source.service';
import { SchemeAutocompleteService } from '../services/scheme-autocomplete.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { FileType, FileInfo } from '../shared/protocol';
import { formatSearchAliasForEdit, highlightMatch, isWindowsPath, normalizeAddressBarPathInput, normalizePath } from './address-bar.utils';
import { EventsOn } from 'wailsjs/runtime';

interface TabSession {
  candidates: PathSuggestion[];
  index: number;
  enteredDir: boolean;
}

function toEditorDriveDisplay(path: string): string {
  const normalized = normalizePath(path);
  const match = /^filesystem:\/\/([a-z])(\/.*)?$/i.exec(normalized);
  if (!match) return normalized;
  const drive = match[1].toUpperCase();
  const rest = (match[2] || '').replace(/^\/+/, '');
  return rest ? `${drive}:/${rest}` : `${drive}:/`;
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

        <!-- Folder icon -->
        <span class="ab-icon" aria-hidden="true">📁</span>

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
✕
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
        <div
          #dropScroll
          class="ab-scroll"
          (scroll)="onDropScroll()"
        >
          <!-- virtual spacer gives the scrollbar the right total height -->
          <div class="ab-virtual-spacer" [style.height.px]="totalDropHeight"></div>

          <!-- only the visible slice is rendered, translated into place -->
          <div
            class="ab-virtual-window"
            [style.transform]="'translateY(' + virtualTranslate + 'px)'"
          >
            <div
              *ngFor="let s of visibleSuggestions; trackBy: trackBySuggestion"
              class="ab-item"
              role="option"
              [attr.aria-selected]="acIdx === s.__idx"
              [class.ab-item--focused]="acIdx === s.__idx"
              (mouseenter)="acIdx = s.__idx"
              (mousedown)="acceptSuggestion(s, $event)"
            >
              <span class="ab-item-path" [innerHTML]="highlight(s.path, value)"></span>
            </div>
          </div>

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
      box-shadow: var(--vsc-widget-shadow, none);
      z-index: var(--vsc-z-dropdown, 1000);
      overflow: hidden;
    }

    /* Inherit focus ring colour on the dropdown border */
    .ab-wrap:focus-within .ab-dropdown {
      border-color: var(--vsc-input-focus-border, #007acc);
    }

    /* Scroll area */
    .ab-scroll {
      max-height: 272px;
      overflow-y: auto;
      padding: 3px 0 4px;
      position: relative;
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

    /* Virtual scroll helpers */
    .ab-virtual-spacer {
      position: absolute;
      top: 0; left: 0; width: 1px;
      pointer-events: none;
    }

    .ab-virtual-window {
      position: relative;
      will-change: transform;
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
        border-left-color: var(--vsc-accent-blue, #007acc);
        font-weight: 600;
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
    :host ::ng-deep .ab-item-path .ab-ac-match {
      background: color-mix(in srgb, var(--vsc-accent-blue, #007acc) 18%, transparent);
      color: color-mix(in srgb, var(--vsc-accent-blue, #007acc) 70%, var(--vsc-foreground, #cccccc));
      font-weight: 700;
      border-radius: 2px;
      padding: 0 1px;
      @supports not (background: color-mix(in srgb, red 1%, blue)) {
        background: var(--vsc-list-focus-background, #062f4a);
        color: var(--vsc-accent-blue, #007acc);
      }
    }
  `],
})
export class AddressBarEditComponent implements OnInit, OnDestroy, OnChanges {
  @ViewChild('editInput') editInputRef?: ElementRef<HTMLInputElement>;
  @ViewChild('dropScroll') dropScrollRef?: ElementRef<HTMLElement>;

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

  // ── Virtual scroll state ────────────────────────────────────────────────
  private readonly DROP_ROW_H = 26;   // must match .ab-item height in CSS
  private readonly DROP_OVERSCAN = 4;
  private readonly DROP_MAX_H = 272;  // must match max-height in CSS
  private vsStart = 0;
  private vsEnd = 20;
  private vsScrollTop = 0;
  private vsRAF: number | null = null;

  get totalDropHeight(): number { return this.suggestions.length * this.DROP_ROW_H; }
  get virtualTranslate(): number { return this.vsStart * this.DROP_ROW_H; }
  get visibleSuggestions(): (PathSuggestion & { __idx: number })[] {
    return this.suggestions
      .slice(this.vsStart, this.vsEnd)
      .map((s, i) => ({ ...s, __idx: this.vsStart + i }));
  }
  trackBySuggestion(_i: number, s: PathSuggestion & { __idx: number }): string { return s.__idx + s.path; }

  private readonly fetchPaneId = 'ab-edit-' + Math.random().toString(36).slice(2, 8);
  private activeFetchStreamId: string | null = null;
  private fetchStreamCancel: (() => void) | null = null;

  private scrollListener?: () => void;
  private readonly commandOwner = `widget.addressBar.edit:${Math.random().toString(36).slice(2, 8)}`;
  private releaseWidgetContext: (() => void) | null = null;
  private syntheticRootSuggestions: ReadonlyArray<{ path: string; displayName: string }> = [];

  highlight = highlightMatch;

  constructor(
    private pathHistoryService: PathHistoryService,
    private apiService: ApiService,
    private directorySource: DirectorySourceService,
    private schemeAutocomplete: SchemeAutocompleteService,
    private cdr: ChangeDetectorRef,
    private hostEl: ElementRef,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
  ) {}

  ngOnInit(): void {
    this.registerInputCommands();
    this.releaseWidgetContext ??= this.inputContexts.acquire('widget.addressBar.edit');
    this.value = this.toDisplayPath(this.currentPath || '');
    this.lastInputValue = this.value;
    void this.loadSyntheticRootSuggestions();

    // Keep existing UX: show suggestions shortly after entering edit mode.
    if (this.value?.trim()) this.scheduleSuggestions(this.value, true);

    document.addEventListener('mousedown', this.onOutsideClick, { capture: true });
  }

  ngOnDestroy(): void {
    clearTimeout(this.blurTimer);
    clearTimeout(this.suggestionsTimer);
    this.cancelFetchStream();
    if (this.vsRAF) cancelAnimationFrame(this.vsRAF);
    this.detachDropScroll();
    this.destroy$.next();
    this.destroy$.complete();
    this.releaseWidgetContext?.();
    this.releaseWidgetContext = null;
    this.unregisterInputCommands();
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
    this.cdr.detectChanges();

    const caret = before.length + insertText.length;
    setTimeout(() => {
      try { el.setSelectionRange(caret, caret); } catch {}
    }, 0);
  }

  private resetVirtualScroll(): void {
    this.vsScrollTop = 0;
    this.vsStart = 0;
    this.vsEnd = Math.min(this.suggestions.length, Math.ceil(this.DROP_MAX_H / this.DROP_ROW_H) + this.DROP_OVERSCAN * 2);
    // reset the DOM scroll position if element exists
    const el = this.dropScrollRef?.nativeElement;
    if (el) el.scrollTop = 0;
  }

  onDropScroll(): void {
    if (this.vsRAF) return;
    this.vsRAF = requestAnimationFrame(() => {
      this.vsRAF = null;
      const el = this.dropScrollRef?.nativeElement;
      if (el) {
        this.vsScrollTop = el.scrollTop;
        this.updateVirtualWindow();
        this.cdr.detectChanges();
      }
    });
  }

  private attachDropScroll(): void {
    const el = this.dropScrollRef?.nativeElement;
    if (!el || this.scrollListener) return;
    this.scrollListener = () => this.onDropScroll();
    el.addEventListener('scroll', this.scrollListener, { passive: true });
  }

  private detachDropScroll(): void {
    const el = this.dropScrollRef?.nativeElement;
    if (el && this.scrollListener) el.removeEventListener('scroll', this.scrollListener);
    this.scrollListener = undefined;
  }

  private updateVirtualWindow(): void {
    const total = this.suggestions.length;
    const visCount = Math.ceil(this.DROP_MAX_H / this.DROP_ROW_H);
    const start = Math.max(0, Math.floor(this.vsScrollTop / this.DROP_ROW_H) - this.DROP_OVERSCAN);
    const end = Math.min(total, start + visCount + this.DROP_OVERSCAN * 2);
    this.vsStart = start;
    this.vsEnd = end;
  }

  onBlur(): void {
    clearTimeout(this.blurTimer);
    this.blurTimer = setTimeout(() => this.cancelEdit(), 170);
  }

  cancelEdit(): void {
    clearTimeout(this.blurTimer);
    this.cancel.emit();
  }

  private commitCurrentValue(): void {
    if (this.acIdx >= 0 && this.suggestions[this.acIdx]) {
      this.acceptSuggestion(this.suggestions[this.acIdx]);
      return;
    }

    const val = normalizeAddressBarPathInput(this.value, this.currentPath);
    if (val) this.commit.emit(val);
    else this.cancelEdit();
  }

  private moveSuggestionSelection(delta: number): void {
    if (this.suggestions.length === 0) return;
    const nextIndex = Math.max(-1, Math.min(this.suggestions.length - 1, this.acIdx + delta));
    this.acIdx = nextIndex;
    this.cdr.markForCheck();
  }

  async onKeyDown(e: KeyboardEvent): Promise<void> {
    switch (e.key) {
      case 'Enter': {
        e.preventDefault();
        this.commitCurrentValue();
        break;
      }
      case 'Escape':
        e.preventDefault();
        this.cancelEdit();
        break;
      case 'ArrowDown':
        e.preventDefault();
        this.moveSuggestionSelection(1);
        break;
      case 'ArrowUp':
        e.preventDefault();
        this.moveSuggestionSelection(-1);
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
            this.cdr.detectChanges();
            break;
          }
        }
        await this.cycleTab(e.shiftKey ? -1 : 1);
        break;
    }
  }

  private getDefaultRootForTab(): string | null {
    const cp = normalizePath((this.currentPath || '').trim());
    const driveMatch = /^filesystem:\/\/([a-z])(?:\/.*)?$/i.exec(cp);
    if (driveMatch) {
      return `${driveMatch[1].toUpperCase()}:/`;
    }
    return null;
  }

  acceptSuggestion(s: PathSuggestion, e?: MouseEvent): void {
    if (e) {
      e.preventDefault();
      e.stopPropagation();
      // Restore focus so the blur timer never fires and dropdown stays open.
      this.editInputRef?.nativeElement?.focus();
    }
    clearTimeout(this.blurTimer);

    const val = normalizePath(s.path);
    if (!val) return;

    // For directories, navigate into them (append slash + refresh suggestions)
    // rather than committing — keeps the dropdown alive for further browsing.
    const isDir = s.matchType === 'directory';
    const finalVal = isDir && !val.endsWith('/') ? val + '/' : val;

    this.writeProgrammaticValue(this.formatEditorPath(finalVal, s.path));
    this.tabSession = null;
    this.cancelFetchStream();
    this.scheduleSuggestions(finalVal, true);
    this.cdr.detectChanges();
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

  private prefersCanonicalDriveDisplay(reference?: string): boolean {
    const raw = ((reference ?? this.value) || '').trim().replace(/\\/g, '/').toLowerCase();
    return raw === 'filesystem://' || raw.startsWith('filesystem://');
  }

  private isFriendlyDirectPathInput(reference?: string): boolean {
    const raw = ((reference ?? this.value) || '').trim().replace(/\\/g, '/');
    if (!raw) return false;
    if (this.prefersCanonicalDriveDisplay(raw)) return false;
    if (/^[A-Za-z]:/.test(raw)) return true;
    if (raw.startsWith('/')) return true;
    if (raw.startsWith('//')) return true;
    return false;
  }

  private isFilesystemRootSuggestion(path: string): boolean {
    const normalized = ((path || '').trim().replace(/\\/g, '/')).toLowerCase();
    return /^filesystem:\/\/[a-z]$/.test(normalized) || normalized === 'filesystem://' || normalized === 'filesystem:///';
  }

  private normalizeSuggestionIdentityPath(path: string): string {
    const normalized = ((path || '').trim().replace(/\\/g, '/')).toLowerCase();
    if (!normalized) return '';
    if (this.isFilesystemRootSuggestion(normalized)) return normalized;
    return normalizePath(path).toLowerCase();
  }

  private toDisplayPath(path: string): string {
    if (!path) return '';
    const searchDisplay = formatSearchAliasForEdit(path);
    if (searchDisplay) return searchDisplay;
    const display = toEditorDriveDisplay(path);
    if (!this.shouldNormalizeToForwardSlashes() && !isWindowsPath(display)) return display;
    return display.replace(/\\/g, '/');
  }

  private formatEditorPath(path: string, reference?: string): string {
    const normalized = normalizePath(path);
    if (!normalized) return '';
    if (this.prefersCanonicalDriveDisplay(reference) && /^filesystem:\/\/[a-z](?:\/.*)?$/i.test(normalized)) {
      return normalized;
    }
    return this.toDisplayPath(normalized);
  }

  private formatSuggestionPath(path: string, query: string, isAliasRoot: boolean): string {
    const normalized = normalizePath(path);
    if (!normalized) return '';
    if (isAliasRoot) return normalized;
    if (this.prefersCanonicalDriveDisplay(query) && /^filesystem:\/\/[a-z](?:\/.*)?$/i.test(normalized)) {
      return normalized;
    }
    return this.toDisplayPath(normalized);
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
    this.cancelFetchStream();
    const requestId = ++this.suggestionsRequestId;
    const delay = immediate ? 0 : 240;

    this.suggestionsTimer = setTimeout(async () => {
      const q = (query || '').trim();
      const aliasRoots = this.buildAliasRootSuggestions(q);
      if (!q) {
        this.suggestions = this.buildMergedSuggestions([], [], aliasRoots, q);
        this.acIdx = -1;
        this.resetVirtualScroll();
        this.cdr.detectChanges();
        return;
      }

      // Load history immediately (local/fast)
      // We are skipping history suggestions as per user request
      const hist: PathSuggestion[] = [];
      if (requestId !== this.suggestionsRequestId) return;

      // Show history right away
      this.suggestions = this.buildMergedSuggestions([], hist, aliasRoots, q);
      this.acIdx = -1;
      this.resetVirtualScroll();
      this.cdr.detectChanges();
      // Attach scroll listener after Angular renders the .ab-scroll div
      setTimeout(() => this.attachDropScroll(), 0);

      // Stream children progressively for any scheme
      if (this.shouldFetchSuggestions(q)) {
        const { dirPath, prefix } = this.splitQueryContext(q);
        if (dirPath) {
          await this.streamDirChildren(dirPath, prefix, (fsBatch) => {
            if (requestId !== this.suggestionsRequestId) return;
            this.suggestions = this.buildMergedSuggestions(fsBatch, hist, aliasRoots, q);
            this.updateVirtualWindow();
            this.cdr.detectChanges();
            setTimeout(() => this.attachDropScroll(), 0);
          });
        }
      }
    }, delay);
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

    this.writeTabValue(this.formatEditorPath(candidate.path, candidate.path));
    this.tabSession  = { candidates: sessionCandidates, index, enteredDir };
    this.suggestions = sessionCandidates;
    this.acIdx       = index;
    this.cdr.detectChanges();

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

  private async fetchChildren(query: string): Promise<PathSuggestion[]> {
    const aliasRoots = this.buildAliasRootSuggestions(query);
    const { dirPath, prefix } = this.splitQueryContext(query);
    if (!dirPath) return aliasRoots;
    const fsSuggestions = await this.streamDirChildren(dirPath, prefix);
    return this.buildMergedSuggestions(fsSuggestions, [], aliasRoots, query);
  }

  private cancelFetchStream(): void {
    if (this.fetchStreamCancel) {
      this.fetchStreamCancel();
      this.fetchStreamCancel = null;
    }

    // Safety net: cancel backend stream even if callback has not been wired yet.
    if (this.activeFetchStreamId) {
      void this.apiService.cancelListDirectoryStream(this.activeFetchStreamId);
      this.activeFetchStreamId = null;
    }
  }

  private streamDirChildren(
    dirPath: string,
    prefix: string,
    onBatch?: (accumulated: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    return new Promise<PathSuggestion[]>(async (resolve) => {
      // Cancel any prior fetch stream
      this.cancelFetchStream();

      // Check if this scheme supports streaming via the DirectoryView's API
      const supportsDirectoryStreaming = this.directorySource.supportsStreaming(dirPath);

      if (!supportsDirectoryStreaming) {
        // Fall back to batch fetch for schemes that don't support streaming
        try {
          const suggestions = await this.schemeAutocomplete.fetchSuggestions(dirPath, prefix);
          onBatch?.(suggestions);
          resolve(suggestions);
        } catch {
          resolve([]);
        }
        return;
      }

      // For schemes that support streaming (filesystem, search, etc.)
      // Use the backend streaming API
      const prefixLower = prefix.toLowerCase();
      const accumulated: PathSuggestion[] = [];
      const seenNames = new Set<string>();

      let streamId: string;
      try {
        streamId = await this.apiService.startListDirectoryStream(this.fetchPaneId, dirPath, false);
      } catch {
        return resolve([]);
      }
      this.activeFetchStreamId = streamId;

      const finish = (unsubAll: () => void) => {
        unsubAll();
        if (this.activeFetchStreamId === streamId) this.activeFetchStreamId = null;
        if (this.fetchStreamCancel === cancel) this.fetchStreamCancel = null;
        resolve(accumulated.slice());
      };

      const offBatch = EventsOn('fs:listStream:batch', (ev: any) => {
        if (ev?.streamId !== streamId) return;
        let changed = false;
        for (const entry of (ev.entries as FileInfo[] || [])) {
          if (entry.type !== FileType.DIRECTORY) continue;
          if (prefixLower && !entry.name.toLowerCase().startsWith(prefixLower)) continue;
          if (seenNames.has(entry.name)) continue;
          seenNames.add(entry.name);
          accumulated.push({
            path:        normalizePath(entry.path),
            displayName: entry.name,
            matchType:   'directory' as const,
            score:       100,
            exists:      true,
          });
          changed = true;
        }
        if (changed) {
          accumulated.sort((a, b) => a.path.localeCompare(b.path));
          onBatch?.(accumulated.slice());
        }
      });

      const offDone = EventsOn('fs:listStream:done', (ev: any) => {
        if (ev?.streamId !== streamId) return;
        finish(() => { offBatch(); offDone(); offError(); });
      });

      const offError = EventsOn('fs:listStream:error', (ev: any) => {
        if (ev?.streamId !== streamId) return;
        finish(() => { offBatch(); offDone(); offError(); });
      });

      const cancel = () => {
        offBatch(); offDone(); offError();
        if (this.activeFetchStreamId === streamId) {
          void this.apiService.cancelListDirectoryStream(streamId);
          this.activeFetchStreamId = null;
        }
        resolve(accumulated.slice());
      };
      this.fetchStreamCancel = cancel;
    });
  }

  private splitQueryContext(query: string): { dirPath: string; prefix: string } {
    return this.schemeAutocomplete.splitQueryContext(query);
  }

  private joinPath(dirPath: string, name: string): string {
    const clean  = dirPath.replace(/\/+$/, '');
    const joined = `${clean}/${name}`.replace(/\\/g, '/');
    if (joined.startsWith('//')) return `//${joined.slice(2).replace(/\/{2,}/g, '/')}`;
    return joined.replace(/\/{2,}/g, '/');
  }

  private shouldFetchSuggestions(query: string): boolean {
    return this.schemeAutocomplete.shouldFetchSuggestions(query);
  }

  private buildAliasRootSuggestions(query: string): PathSuggestion[] {
    const normalizedQuery = normalizePath((query || '').trim().replace(/\\/g, '/')).toLowerCase();

    const suggestions: PathSuggestion[] = [];

    for (const entry of this.syntheticRootSuggestions) {
      const rootPath = this.normalizeSuggestionIdentityPath(entry.path);
      const schemeMatch = /^([a-z][a-z0-9+.-]*):\/\//i.exec(rootPath);
      const schemeName = (schemeMatch?.[1] || rootPath).toLowerCase();
      const schemePrefix = schemeMatch ? `${schemeName}://` : rootPath;

      let score = 0;
      if (!normalizedQuery) {
        score = 120;
      } else if (normalizedQuery === rootPath || normalizedQuery === schemePrefix || normalizedQuery === schemeName) {
        score = 180;
      } else if (rootPath.startsWith(normalizedQuery)) {
        score = 160;
      } else if (schemePrefix.startsWith(normalizedQuery) || schemeName.startsWith(normalizedQuery)) {
        score = 150;
      }

      // When the user is typing a friendly direct path (e.g. "C:/" or "/"),
      // keep only the bare filesystem root aliases at the end so friendlier
      // path forms remain ahead of the raw scheme roots.
      if (this.isFilesystemRootSuggestion(entry.path) && this.isFriendlyDirectPathInput(query)) {
        // Keep the suggestion but give it a low score so it sorts last.
        if (score) score = Math.min(score, 10);
        else score = 10;
      }

      if (!score) continue;

      suggestions.push({
        path: entry.path,
        displayName: entry.displayName,
        matchType: 'directory',
        score,
        exists: true,
      });
    }

    suggestions.sort((left, right) => right.score - left.score || left.path.localeCompare(right.path));
    return suggestions;
  }

  private async loadSyntheticRootSuggestions(): Promise<void> {
    try {
      const drives = await this.apiService.listDrives();
      this.syntheticRootSuggestions = drives
        .filter((drive) => {
          const normalized = (drive.path || '').trim().replace(/\\/g, '/');
          return /^[a-z][a-z0-9+.-]*:\/\/(?:[a-z0-9._~-]+)?$/i.test(normalized) || /^filesystem:\/\/\/?$/i.test(normalized);
        })
        .map((drive) => ({
          path: this.isFilesystemRootSuggestion(drive.path) ? (drive.path || '').trim().replace(/\\/g, '/') : normalizePath(drive.path),
          displayName: drive.name || drive.path,
        }));

      if (this.value?.trim()) {
        this.scheduleSuggestions(this.value, true);
      }
    } catch {
      this.syntheticRootSuggestions = [];
    }
  }

  private buildMergedSuggestions(fs: PathSuggestion[], hist: PathSuggestion[], aliasRoots: PathSuggestion[] = [], query: string = ''): PathSuggestion[] {
    const seen = new Set<string>();
    const aliases: PathSuggestion[] = [];
    const deferredAliases: PathSuggestion[] = [];
    const dirs: PathSuggestion[] = [];
    const rest: PathSuggestion[] = [];

    const aliasRootPaths = new Set(aliasRoots.map((entry) => this.normalizeSuggestionIdentityPath(entry.path)));

    for (const s of [...aliasRoots, ...fs, ...hist]) {
      const key = this.normalizeSuggestionIdentityPath(s?.path || '');
      if (!key || seen.has(key)) continue;
      seen.add(key);
      const isAliasRoot = aliasRootPaths.has(key);
      const entry = { ...s, path: this.formatSuggestionPath(s.path, query, isAliasRoot) };
      if (isAliasRoot) {
        if (this.isFilesystemRootSuggestion(s.path) && this.isFriendlyDirectPathInput(query)) {
          deferredAliases.push(entry);
        } else {
          aliases.push(entry);
        }
        continue;
      }
      (entry.matchType === 'directory' ? dirs : rest).push(entry);
    }
    return [...aliases, ...dirs, ...rest, ...deferredAliases];
  }

  private registerInputCommands(): void {
    this.inputEngine.registerCommand('widget.addressBar.edit.commit', () => {
      this.commitCurrentValue();
    }, this.commandOwner);

    this.inputEngine.registerCommand('widget.addressBar.edit.cancel', () => {
      this.cancelEdit();
    }, this.commandOwner);

    this.inputEngine.registerCommand('widget.addressBar.edit.nextSuggestion', async () => {
      if (!(this.value || '').trim()) {
        const root = this.getDefaultRootForTab();
        if (root) {
          this.writeProgrammaticValue(root);
          this.tabSession = null;
          this.scheduleSuggestions(root, true);
          this.cdr.detectChanges();
          return;
        }
      }

      await this.cycleTab(1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('widget.addressBar.edit.prevSuggestion', async () => {
      if (!(this.value || '').trim()) {
        const root = this.getDefaultRootForTab();
        if (root) {
          this.writeProgrammaticValue(root);
          this.tabSession = null;
          this.scheduleSuggestions(root, true);
          this.cdr.detectChanges();
          return;
        }
      }

      await this.cycleTab(-1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('widget.addressBar.edit.prevItem', () => {
      this.moveSuggestionSelection(-1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('widget.addressBar.edit.nextItem', () => {
      this.moveSuggestionSelection(1);
    }, this.commandOwner);
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('widget.addressBar.edit.commit', this.commandOwner);
    this.inputEngine.unregisterCommand('widget.addressBar.edit.cancel', this.commandOwner);
    this.inputEngine.unregisterCommand('widget.addressBar.edit.nextSuggestion', this.commandOwner);
    this.inputEngine.unregisterCommand('widget.addressBar.edit.prevSuggestion', this.commandOwner);
    this.inputEngine.unregisterCommand('widget.addressBar.edit.prevItem', this.commandOwner);
    this.inputEngine.unregisterCommand('widget.addressBar.edit.nextItem', this.commandOwner);
  }
}