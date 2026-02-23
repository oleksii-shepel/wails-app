import {
  Component, Output, EventEmitter, HostListener,
  ElementRef, ViewChild, ChangeDetectionStrategy, ChangeDetectorRef,
  OnDestroy,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

export interface FindWidgetState {
  text: string;
  matchCase: boolean;
  wholeWord: boolean;
  useRegex: boolean;
}

@Component({
  selector: 'app-find-widget',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="fw-panel"
      [class.fw-panel--visible]="visible"
      role="search"
      aria-label="Find in files"
    >
      <!-- Input row -->
      <div class="fw-input-row">
        <div class="fw-input-wrap" [class.fw-input-wrap--error]="hasRegexError">
          <input
            #inputEl
            type="text"
            class="fw-input"
            [placeholder]="'Search'"
            [(ngModel)]="searchText"
            (ngModelChange)="onTextChange()"
            (keydown)="onInputKeyDown($event)"
            [attr.aria-label]="'Find'"
            autocomplete="off"
            spellcheck="false"
          />

          <!-- Match count -->
          <span class="fw-match-count" *ngIf="searchText && matchCount !== null">
            {{ matchCount === 0 ? 'No results' : (currentMatch + ' of ' + matchCount) }}
          </span>

          <!-- Toggle buttons -->
          <div class="fw-toggles">
            <button
              class="fw-toggle"
              [class.fw-toggle--on]="matchCase"
              (click)="matchCase = !matchCase; onTextChange()"
              title="Match Case (Alt+C)"
              aria-label="Match Case"
            >Aa</button>

            <button
              class="fw-toggle"
              [class.fw-toggle--on]="wholeWord"
              (click)="wholeWord = !wholeWord; onTextChange()"
              title="Match Whole Word (Alt+W)"
              aria-label="Match Whole Word"
            >
              <svg width="11" height="11" viewBox="0 0 16 16" fill="currentColor">
                <path d="M1 11V3h1v1.5l.5-.5h1l.5.5V3h1v8H4V7.5l-.5.5h-1l-.5-.5V11zm5-8v8h1V8.5l.5.5h1l.5-.5V11h1V3h-1v2.5L9 5H8l-.5-.5V3zm5 0h3v1h-2v2h1.5v1H14v2h2v1h-3V3z"/>
              </svg>
            </button>

            <button
              class="fw-toggle"
              [class.fw-toggle--on]="useRegex"
              (click)="useRegex = !useRegex; onTextChange()"
              title="Use Regular Expression (Alt+R)"
              aria-label="Use Regular Expression"
            >.*</button>
          </div>
        </div>

        <!-- Prev / Next / Close -->
        <div class="fw-actions">
          <button
            class="fw-action"
            (click)="onPrev()"
            [disabled]="!matchCount"
            title="Previous Match (Shift+Enter)"
            aria-label="Previous match"
          >
            <svg width="11" height="11" viewBox="0 0 16 16" fill="currentColor">
              <path d="M7.977 10.079l4.119-4.834-.753-.642-3.367 3.951-3.367-3.951-.753.642z"/>
            </svg>
          </button>
          <button
            class="fw-action"
            (click)="onNext()"
            [disabled]="!matchCount"
            title="Next Match (Enter)"
            aria-label="Next match"
          >
            <svg width="11" height="11" viewBox="0 0 16 16" fill="currentColor">
              <path d="M7.976 10.072l-4.204-4.786.753-.661 3.451 3.93 3.451-3.93.753.661z" transform="rotate(180,8,8)"/>
            </svg>
          </button>
          <button
            class="fw-action fw-action--close"
            (click)="close()"
            title="Close (Escape)"
            aria-label="Close find widget"
          >
            <svg width="11" height="11" viewBox="0 0 16 16" fill="currentColor">
              <path d="M8 8.707l3.646 3.647.708-.707L8.707 8l3.647-3.646-.707-.708L8 7.293 4.354 3.646l-.707.707L7.293 8l-3.646 3.647.707.707z"/>
            </svg>
          </button>
        </div>
      </div>

      <!-- Regex error -->
      <div class="fw-error" *ngIf="hasRegexError">
        <svg width="11" height="11" viewBox="0 0 16 16" fill="currentColor">
          <path d="M8 1a7 7 0 1 0 0 14A7 7 0 0 0 8 1zm-.75 3h1.5v5.5h-1.5V4zm0 7h1.5v1.5h-1.5V11z"/>
        </svg>
        Invalid regular expression
      </div>
    </div>
  `,
  styles: [`
    :host {
      position: absolute;
      top: 0;
      right: 0;
      z-index: 200;
      pointer-events: none;
    }

    /* ── Panel ─────────────────────────────────────────────────── */
    .fw-panel {
      display: flex;
      flex-direction: column;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-top: none;
      border-radius: 0 0 0 var(--vsc-border-radius);
      box-shadow: 0 4px 12px rgba(0,0,0,0.32);
      font-family: var(--vsc-font-family);
      font-size: var(--vsc-font-size);
      pointer-events: all;
      min-width: 320px;
      max-width: 480px;

      /* Slide in from top-right */
      transform: translateY(-100%) scaleY(0.8);
      opacity: 0;
      transform-origin: top right;
      transition:
        transform 0.14s cubic-bezier(0.4, 0, 0.2, 1),
        opacity 0.10s ease;
    }
    .fw-panel--visible {
      transform: translateY(0) scaleY(1);
      opacity: 1;
    }

    /* ── Input row ─────────────────────────────────────────────── */
    .fw-input-row {
      display: flex;
      align-items: center;
      gap: 3px;
      padding: 4px 4px 4px 6px;
    }

    .fw-input-wrap {
      display: flex;
      align-items: center;
      flex: 1;
      min-width: 0;
      height: 26px;
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-input-border);
      border-radius: var(--vsc-border-radius-sm);
      transition: border-color var(--vsc-transition-fast);

      &:focus-within { border-color: var(--vsc-input-focus-border); }
    }
    .fw-input-wrap--error {
      border-color: var(--vsc-error) !important;
    }

    .fw-input {
      flex: 1;
      min-width: 0;
      height: 100%;
      padding: 0 6px;
      background: transparent;
      border: none;
      outline: none;
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);
      font-family: var(--vsc-font-family);

      &::placeholder { color: var(--vsc-input-placeholder); }
    }

    /* Match count pill */
    .fw-match-count {
      flex-shrink: 0;
      padding: 0 6px;
      font-size: 11px;
      font-family: var(--vsc-font-family-mono);
      color: var(--vsc-foreground-dim);
      white-space: nowrap;
      border-left: 1px solid var(--vsc-border-subtle);
    }

    /* ── Toggle buttons (Aa / W / .*) ────────────────────────── */
    .fw-toggles {
      display: flex;
      align-items: center;
      flex-shrink: 0;
      border-left: 1px solid var(--vsc-border-subtle);
    }

    .fw-toggle {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 24px;
      height: 24px;
      padding: 0;
      background: transparent;
      border: none;
      color: var(--vsc-foreground-dim);
      font-size: 11px;
      font-family: var(--vsc-font-family-mono);
      font-weight: 600;
      cursor: pointer;
      border-radius: var(--vsc-border-radius-sm);
      transition: background var(--vsc-transition-fast), color var(--vsc-transition-fast);

      &:hover { background: var(--vsc-list-hover-background); color: var(--vsc-foreground-bright); }
      &:focus-visible { outline: 1px solid var(--vsc-input-focus-border); outline-offset: -1px; }
    }
    .fw-toggle--on {
      background: var(--vsc-list-active-background);
      color: var(--vsc-foreground-bright);
    }

    /* ── Action buttons (prev/next/close) ─────────────────────── */
    .fw-actions {
      display: flex;
      align-items: center;
      gap: 1px;
      flex-shrink: 0;
    }

    .fw-action {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 24px;
      height: 24px;
      padding: 0;
      background: transparent;
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-dim);
      cursor: pointer;
      transition: background var(--vsc-transition-fast), color var(--vsc-transition-fast), border-color var(--vsc-transition-fast);

      svg { fill: currentColor; pointer-events: none; }

      &:hover:not(:disabled) {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground-bright);
        border-color: var(--vsc-border);
      }
      &:disabled { opacity: 0.35; cursor: default; }
      &:focus-visible { outline: 1px solid var(--vsc-input-focus-border); outline-offset: -1px; }
    }
    .fw-action--close { margin-left: 2px; }

    /* ── Regex error bar ──────────────────────────────────────── */
    .fw-error {
      display: flex;
      align-items: center;
      gap: 5px;
      padding: 3px 8px 5px;
      font-size: 11px;
      color: var(--vsc-error);

      svg { fill: currentColor; flex-shrink: 0; }
    }
  `],
})
export class FindWidgetComponent implements OnDestroy {

  @ViewChild('inputEl') inputEl?: ElementRef<HTMLInputElement>;

  @Output() stateChange = new EventEmitter<FindWidgetState | null>();
  @Output() nextMatch   = new EventEmitter<void>();
  @Output() prevMatch   = new EventEmitter<void>();

  visible     = false;
  searchText  = '';
  matchCase   = false;
  wholeWord   = false;
  useRegex    = false;
  matchCount: number | null = null;
  currentMatch = 0;
  hasRegexError = false;

  private _focusTimer: any;

  constructor(private cdr: ChangeDetectorRef) {}

  ngOnDestroy(): void { clearTimeout(this._focusTimer); }

  // ── Open / close ──────────────────────────────────────────────

  open(): void {
    this.visible = true;
    this.hasRegexError = false;
    this.cdr.markForCheck();
    // Defer focus so the CSS transition has started
    clearTimeout(this._focusTimer);
    this._focusTimer = setTimeout(() => {
      this.inputEl?.nativeElement.focus();
      this.inputEl?.nativeElement.select();
    }, 60);
    this.emit();
  }

  close(): void {
    this.visible = false;
    this.searchText = '';
    this.matchCount = null;
    this.currentMatch = 0;
    this.hasRegexError = false;
    this.cdr.markForCheck();
    this.stateChange.emit(null);  // null = no filter
  }

  toggle(): void { this.visible ? this.close() : this.open(); }

  // ── Input events ──────────────────────────────────────────────

  onTextChange(): void {
    this.hasRegexError = false;
    if (this.useRegex && this.searchText) {
      try { new RegExp(this.searchText); } catch { this.hasRegexError = true; }
    }
    this.currentMatch = 1;
    this.emit();
  }

  onInputKeyDown(e: KeyboardEvent): void {
    switch (e.key) {
      case 'Escape':    e.preventDefault(); this.close(); break;
      case 'Enter':     e.preventDefault(); e.shiftKey ? this.onPrev() : this.onNext(); break;
      case 'F3':        e.preventDefault(); e.shiftKey ? this.onPrev() : this.onNext(); break;
    }
  }

  onNext(): void { if (this.matchCount) { this.currentMatch = (this.currentMatch % this.matchCount) + 1; this.nextMatch.emit(); this.cdr.markForCheck(); } }
  onPrev(): void { if (this.matchCount) { this.currentMatch = ((this.currentMatch - 2 + this.matchCount) % this.matchCount) + 1; this.prevMatch.emit(); this.cdr.markForCheck(); } }

  /** Called by the parent to update the match count badge. */
  setMatchCount(count: number): void {
    this.matchCount = count;
    if (this.currentMatch > count) this.currentMatch = count || 0;
    this.cdr.markForCheck();
  }

  // ── Emit state ────────────────────────────────────────────────

  private emit(): void {
    if (!this.visible || !this.searchText || this.hasRegexError) {
      this.stateChange.emit(null);
      return;
    }
    this.stateChange.emit({
      text:      this.searchText,
      matchCase: this.matchCase,
      wholeWord: this.wholeWord,
      useRegex:  this.useRegex,
    });
  }
}