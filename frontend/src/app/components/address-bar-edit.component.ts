import {
  Component, Input, Output, EventEmitter, ViewChild, ElementRef,
  OnChanges, SimpleChanges, ChangeDetectionStrategy,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { PathSuggestion } from '../services/path-history.service';
import { highlightMatch } from './address-bar.utils';

@Component({
  selector: 'address-bar-edit',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <!-- Edit input row -->
    <div class="ab-edit">
      <span class="ab-edit-icon">📂</span>
      <input
        #editInput
        class="ab-edit-input"
        type="text"
        [(ngModel)]="value"
        (ngModelChange)="valueChange.emit($event)"
        (keydown)="editKeydown.emit($event)"
        (blur)="blur.emit()"
        [placeholder]="placeholder"
        autocomplete="off"
        spellcheck="false"
      />
      <span class="ab-edit-hint" *ngIf="!value">{{ currentPath }}</span>
      <button class="ab-btn ab-btn--cancel" (click)="cancel.emit()" title="Cancel (Esc)" aria-label="Cancel">✕</button>
    </div>

    <!-- Autocomplete dropdown -->
    <div class="ab-dropdown ab-autocomplete" *ngIf="suggestions.length > 0">
      <div class="ab-drop-head">
        <span>Suggestions</span>
        <span class="ab-badge">{{ suggestions.length }}</span>
      </div>
      <div class="ab-drop-scroll">
        <div
          *ngFor="let s of suggestions; let i = index"
          class="ab-drop-item ab-ac-item"
          [class.ab-drop-item--focused]="acIdx === i"
          (mouseenter)="acIdxChange.emit(i)"
          (mousedown)="suggestionSelect.emit({ suggestion: s, event: $event })"
        >
          <span class="ab-ac-badge ab-ac-badge--{{ s.matchType }}">{{ s.matchType }}</span>
          <span class="ab-ac-path" [innerHTML]="highlight(s.path, value)"></span>
        </div>
      </div>
      <div class="ab-drop-footer">
        <kbd>Tab</kbd>/<kbd>Shift+Tab</kbd> cycle &nbsp; <kbd>/</kbd> auto-add on idle &nbsp; <kbd>↑↓</kbd> navigate &nbsp; <kbd>⏎</kbd> select &nbsp; <kbd>Esc</kbd> cancel
      </div>
    </div>
  `,
  styles: [`
    :host { display: block; width: 100%; }

    .ab-edit {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      height: var(--vsc-titlebar-height); padding: 0 var(--vsc-padding-md);
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-accent-blue);
      border-radius: var(--vsc-border-radius);
      animation: ab-edit-in var(--vsc-transition-fast) ease;
      position: relative;
      box-sizing: border-box;
      width: 100%;
    }

    @keyframes ab-edit-in {
      from { background: color-mix(in srgb, var(--vsc-accent-blue) 10%, var(--vsc-input-background)); border-color: color-mix(in srgb, var(--vsc-accent-blue) 50%, transparent); }
      to   { background: var(--vsc-input-background); border-color: var(--vsc-accent-blue); }
    }

    .ab-edit-icon { font-size: 14px; flex-shrink: 0; opacity: 0.8; color: var(--vsc-foreground-dim); }

    .ab-edit-input {
      flex: 1; background: transparent; border: none; outline: none;
      color: var(--vsc-input-foreground); font-family: var(--vsc-font-family-mono);
      font-size: var(--vsc-font-size); caret-color: var(--vsc-accent-blue);
      min-width: 0; padding: 0;
      &::placeholder { color: var(--vsc-input-placeholder); opacity: 1; }
      &::selection { background: color-mix(in srgb, var(--vsc-accent-blue) 40%, transparent); }
    }

    .ab-edit-hint {
      position: absolute; left: calc(32px + var(--vsc-padding-sm));
      font-family: var(--vsc-font-family-mono); font-size: var(--vsc-font-size);
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
      background-color: var(--vsc-panel-background, #252526) !important;
      background-image: none !important;
      border: 1px solid var(--vsc-border); border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow); z-index: 9999; overflow: hidden;
      animation: ab-appear var(--vsc-transition-fast) cubic-bezier(0.4, 0, 0.2, 1);
    }

    @keyframes ab-appear {
      from { opacity: 0; transform: translateY(-4px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    .ab-autocomplete { max-height: 350px; overflow-y: auto; }

    .ab-drop-head {
      display: flex; align-items: center; justify-content: space-between;
      height: 32px; padding: 0 var(--vsc-padding-lg);
      background: var(--vsc-panel-background, #252526) !important;
      border-bottom: 1px solid var(--vsc-border-subtle);
      font-size: var(--vsc-font-size-small); color: var(--vsc-foreground-dim);
      text-transform: uppercase; letter-spacing: 0.5px; font-weight: 600;
    }

    .ab-badge {
      display: inline-flex; align-items: center; justify-content: center;
      min-width: 20px; height: 20px; padding: 0 var(--vsc-padding-xs);
      background: var(--vsc-badge-background); color: var(--vsc-badge-foreground);
      border-radius: 10px; font-size: 11px; font-weight: 600;
    }

    .ab-drop-scroll {
      max-height: 300px; overflow-y: auto; padding: var(--vsc-padding-xs) 0;
      background-color: var(--vsc-panel-background, #252526) !important;
    }

    .ab-drop-item {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      height: 28px; padding: 0 var(--vsc-padding-lg); cursor: pointer;
      transition: background-color var(--vsc-transition-fast);
      border-left: 2px solid transparent; background: transparent;
      color: var(--vsc-foreground); font-size: var(--vsc-font-size);
      &:hover { background: var(--vsc-list-hover-background); }
      &.ab-drop-item--focused { background: var(--vsc-list-focus-background); }
    }

    .ab-drop-footer {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      height: 28px; padding: 0 var(--vsc-padding-lg);
      background: var(--vsc-panel-background, #252526) !important;
      border-top: 1px solid var(--vsc-border-subtle);
      font-size: 11px; color: var(--vsc-foreground-dim);
      kbd {
        display: inline-flex; align-items: center; justify-content: center;
        min-width: 18px; height: 18px; padding: 0 3px;
        background: var(--vsc-input-background); border: 1px solid var(--vsc-border);
        border-bottom-width: 2px; border-radius: var(--vsc-border-radius-sm);
        font-family: var(--vsc-font-family-mono); font-size: 10px;
        color: var(--vsc-foreground); margin: 0 var(--vsc-padding-xs);
      }
    }

    .ab-ac-badge {
      flex-shrink: 0; font-size: 10px; font-weight: 700; text-transform: uppercase;
      letter-spacing: 0.4px; padding: 2px var(--vsc-padding-xs);
      border-radius: var(--vsc-border-radius-sm); min-width: 50px; text-align: center;
      &.ab-ac-badge--history   { background: var(--vsc-success); color: var(--vsc-foreground-bright); }
      &.ab-ac-badge--directory { background: var(--vsc-accent-blue); color: var(--vsc-foreground-bright); }
      &.ab-ac-badge--fuzzy     { background: var(--vsc-accent-orange); color: var(--vsc-foreground-bright); }
    }

    .ab-ac-path {
      flex: 1; font-size: var(--vsc-font-size); font-family: var(--vsc-font-family-mono);
      color: var(--vsc-foreground); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
      ::ng-deep mark {
        background: var(--vsc-accent-blue); color: var(--vsc-foreground-bright);
        padding: 0 2px; border-radius: 2px; font-style: normal; font-weight: 500;
      }
    }
  `],
})
export class AddressBarEditComponent implements OnChanges {
  @ViewChild('editInput') editInputRef?: ElementRef<HTMLInputElement>;

  @Input() value = '';
  @Input() currentPath = '';
  @Input() placeholder = 'Enter path…';
  @Input() suggestions: PathSuggestion[] = [];
  @Input() acIdx = -1;
  @Input() focusOnInit = false;

  @Output() valueChange      = new EventEmitter<string>();
  @Output() editKeydown      = new EventEmitter<KeyboardEvent>();
  @Output() blur             = new EventEmitter<void>();
  @Output() cancel           = new EventEmitter<void>();
  @Output() acIdxChange      = new EventEmitter<number>();
  @Output() suggestionSelect = new EventEmitter<{ suggestion: PathSuggestion; event: MouseEvent }>();

  highlight = highlightMatch;

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['focusOnInit']?.currentValue) {
      setTimeout(() => {
        const el = this.editInputRef?.nativeElement;
        if (el) { el.focus(); el.select(); }
      }, 30);
    }
  }
}
