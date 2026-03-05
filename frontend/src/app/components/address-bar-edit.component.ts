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
    <div class="ab-edit-wrap">
      <!-- Edit input row -->
      <div class="ab-edit" [class.ab-edit--open]="suggestions.length > 0">
        <span class="ab-edit-icon">📂</span>
        <input
          #editInput
          class="ab-edit-input"
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
        <button class="ab-btn ab-btn--cancel" (click)="cancelEdit()" title="Cancel (Esc)" aria-label="Cancel">✕</button>
      </div>

      <!-- Autocomplete dropdown (anchored to the full-width wrapper) -->
      <div
        class="ab-dropdown ab-autocomplete"
        *ngIf="suggestions.length > 0"
        (mousedown)="$event.preventDefault()"
      >
        <div class="ab-drop-scroll">
          <div
            *ngFor="let s of suggestions; let i = index"
            class="ab-drop-item ab-ac-item"
            [class.ab-drop-item--focused]="acIdx === i"
            (mouseenter)="acIdx = i"
            (mousedown)="acceptSuggestion(s, $event)"
          >
            <span class="ab-ac-path" [innerHTML]="highlight(s.path, value)"></span>
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host { display: block; width: 100%; }

    .ab-edit-wrap { position: relative; width: 100%; }

    .ab-edit {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      height: var(--vsc-titlebar-height); padding: 0 var(--vsc-padding-md);
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-input-border);
      border-radius: var(--vsc-border-radius-sm);
      position: relative;
      box-sizing: border-box;
      width: 100%;

      &:focus-within {
        border-color: var(--vsc-input-focus-border);
      }
    }

    .ab-edit.ab-edit--open {
      border-bottom-left-radius: 0;
      border-bottom-right-radius: 0;
      border-bottom: none;
    }

    .ab-edit-icon { font-size: 14px; flex-shrink: 0; opacity: 0.8; color: var(--vsc-foreground-dim); }

    .ab-edit-input {
      flex: 1; background: transparent; border: none; outline: none;
      color: var(--vsc-input-foreground); font-family: var(--vsc-font-family);
      font-size: var(--vsc-font-size); caret-color: var(--vsc-input-focus-border);
      min-width: 0; padding: 0;
      &::placeholder { color: var(--vsc-input-placeholder); opacity: 1; }
    }

    .ab-edit-hint {
      position: absolute; left: calc(32px + var(--vsc-padding-sm));
      font-family: var(--vsc-font-family); font-size: var(--vsc-font-size);
      color: var(--vsc-input-placeholder); pointer-events: none;
      white-space: nowrap; overflow: hidden; opacity: 0.7;
    }

    .ab-btn {
      display: flex; align-items: center; justify-content: center;
      width: 26px; height: 26px; padding: 0; background: transparent; border: none;
      border-radius: var(--vsc-border-radius-sm); cursor: pointer;
      transition: background-color var(--vsc-transition-fast), color var(--vsc-transition-fast);
    }

    .ab-btn--cancel {
      font-size: 14px; color: var(--vsc-foreground-dim);
      &:hover { background: var(--vsc-error); color: var(--vsc-foreground-bright); }
    }

    /* autocomplete dropdown */
    .ab-dropdown {
      position: absolute; left: 0; right: 0;
      top: calc(100% - 1px);
      background-color: var(--vsc-panel-background, #252526) !important;
      background-image: none !important;
      border: 1px solid var(--vsc-input-border);
      border-top: none;
      border-bottom-left-radius: var(--vsc-border-radius-sm);
      border-bottom-right-radius: var(--vsc-border-radius-sm);
      box-shadow: var(--vsc-widget-shadow); z-index: 9999; overflow: hidden;
      animation: ab-appear var(--vsc-transition-fast) cubic-bezier(0.4, 0, 0.2, 1);
    }

    .ab-edit-wrap:focus-within .ab-dropdown {
      border-color: var(--vsc-input-focus-border);
    }

    @keyframes ab-appear {
      from { opacity: 0; transform: translateY(-4px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    .ab-drop-scroll {
      padding: var(--vsc-padding-xs) 0;
      background-color: var(--vsc-panel-background, #252526) !important;
      max-height: 280px;
      overflow-y: auto;
    }

    .ab-drop-item {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      height: 26px; padding: 0 var(--vsc-padding-md); cursor: pointer;
      transition: background-color var(--vsc-transition-fast);
      border-left: 2px solid transparent; background: transparent;
      color: var(--vsc-foreground); font-size: var(--vsc-font-size);
      &:hover { background: var(--vsc-list-hover-background); }
      &.ab-drop-item--focused { background: var(--vsc-list-focus-background); }
    }

    .ab-ac-path {
      flex: 1; font-size: var(--vsc-font-size); font-family: var(--vsc-font-family);
      color: var(--vsc-foreground); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
      ::ng-deep mark {
        background: var(--vsc-accent-blue); color: var(--vsc-foreground-bright);
        padding: 0 2px; border-radius: 2px; font-style: normal; font-weight: 500;
      }
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
