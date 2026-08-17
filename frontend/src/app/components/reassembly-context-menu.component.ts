import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  OnDestroy,
  Output,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

export interface ReassemblyContextMenuItem {
  commandId?: string;
  label?: string;
  shortcut?: string;
  enabled?: boolean;
  separator?: boolean;
  danger?: boolean;
}

export interface ReassemblyContextMenuState {
  x: number;
  y: number;
  title?: string;
  items: ReassemblyContextMenuItem[];
}

@Component({
  selector: 'app-reassembly-context-menu',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      *ngIf="state"
      class="backdrop"
      (mousedown)="closed.emit()"
      (contextmenu)="$event.preventDefault(); closed.emit()"
      (keydown)="onKeyDown($event)">

      <section
        #menu
        class="menu"
        role="menu"
        tabindex="-1"
        [attr.aria-label]="state.title || 'Context menu'"
        [style.left.px]="menuX"
        [style.top.px]="menuY"
        (mousedown)="$event.stopPropagation()"
        (contextmenu)="$event.preventDefault(); $event.stopPropagation()">

        <div class="title" *ngIf="state.title">{{ state.title }}</div>

        <ng-container *ngFor="let item of state.items; let i = index">
          <div *ngIf="item.separator" class="separator" role="separator"></div>

          <button
            *ngIf="!item.separator"
            type="button"
            role="menuitem"
            [attr.data-menu-index]="i"
            [class.danger]="item.danger"
            [disabled]="item.enabled === false"
            (mouseenter)="focusItem(i)"
            (click)="choose(item)">
            <span class="label">{{ item.label }}</span>
            <span class="shortcut" *ngIf="item.shortcut">{{ item.shortcut }}</span>
          </button>
        </ng-container>
      </section>
    </div>
  `,
  styles: [`
    :host {
      position: fixed;
      inset: 0;
      z-index: 9000;
      pointer-events: none;
    }

    .backdrop {
      position: fixed;
      inset: 0;
      pointer-events: auto;
    }

    .menu {
      position: fixed;
      box-sizing: border-box;
      width: min(286px, calc(100vw - 16px));
      max-height: calc(100vh - 16px);
      overflow: auto;
      padding: 4px;
      border: 1px solid var(--vsc-border, var(--vsc-border));
      border-radius: 6px;
      background: var(--vsc-menu-background, var(--vsc-panel-background));
      color: var(--vsc-menu-foreground, var(--vsc-foreground));
      box-shadow: 0 8px 30px var(--vsc-scrim);
      font: 12px/1.35 var(--vsc-font-family, system-ui, sans-serif);
      outline: none;
    }

    .title {
      padding: 5px 8px 6px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 10px;
    }

    button {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      gap: 16px;
      align-items: center;
      width: 100%;
      min-height: 28px;
      padding: 4px 8px;
      border: 0;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      text-align: left;
      cursor: pointer;
      font: inherit;
    }

    button:hover:not(:disabled),
    button:focus-visible:not(:disabled) {
      outline: none;
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    button.danger:not(:disabled) {
      color: var(--vsc-error, var(--vsc-danger));
    }

    button:disabled {
      color: var(--vsc-disabled-foreground, var(--vsc-foreground-dim));
      opacity: .62;
      cursor: default;
    }

    .label {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .shortcut {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 10px;
      white-space: nowrap;
    }

    .separator {
      height: 1px;
      margin: 4px 5px;
      background: var(--vsc-border, var(--vsc-border));
    }
  `],
})
export class ReassemblyContextMenuComponent implements OnDestroy {
  @ViewChild('menu') menu?: ElementRef<HTMLElement>;

  @Output() command = new EventEmitter<string>();
  @Output() closed = new EventEmitter<void>();

  private _state: ReassemblyContextMenuState | null = null;
  private focusTimer: ReturnType<typeof setTimeout> | null = null;
  private typeaheadTimer: ReturnType<typeof setTimeout> | null = null;
  private typeahead = '';

  menuX = 8;
  menuY = 8;

  constructor(private readonly cdr: ChangeDetectorRef) {}

  @Input()
  set state(value: ReassemblyContextMenuState | null) {
    this._state = value;
    if (!value) {
      this.clearFocusTimer();
      this.clearTypeahead();
      return;
    }

    this.menuX = value.x;
    this.menuY = value.y;
    this.scheduleInitialFocus();
  }

  get state(): ReassemblyContextMenuState | null {
    return this._state;
  }

  ngOnDestroy(): void {
    this.clearFocusTimer();
    this.clearTypeahead();
  }

  choose(item: ReassemblyContextMenuItem): void {
    if (!item.commandId || item.enabled === false) return;
    this.command.emit(item.commandId);
  }

  focusItem(index: number): void {
    const button = this.buttonForItemIndex(index);
    if (button && !button.disabled) button.focus({ preventScroll: true });
  }

  onKeyDown(event: KeyboardEvent): void {
    if (!this.state) return;

    switch (event.key) {
      case 'Escape':
        event.preventDefault();
        event.stopPropagation();
        this.closed.emit();
        return;
      case 'ArrowDown':
        event.preventDefault();
        event.stopPropagation();
        this.focusRelative(1);
        return;
      case 'ArrowUp':
        event.preventDefault();
        event.stopPropagation();
        this.focusRelative(-1);
        return;
      case 'Home':
        event.preventDefault();
        event.stopPropagation();
        this.focusBoundary(false);
        return;
      case 'End':
        event.preventDefault();
        event.stopPropagation();
        this.focusBoundary(true);
        return;
      case 'Enter':
      case ' ':
        if (this.menu?.nativeElement.contains(document.activeElement)) {
          event.preventDefault();
          event.stopPropagation();
          (document.activeElement as HTMLButtonElement | null)?.click();
        }
        return;
      default:
        break;
    }

    if (
      event.key.length === 1 &&
      !event.ctrlKey &&
      !event.altKey &&
      !event.metaKey
    ) {
      this.handleTypeahead(event.key);
    }
  }

  private scheduleInitialFocus(): void {
    this.clearFocusTimer();
    this.focusTimer = setTimeout(() => {
      this.focusTimer = null;
      this.repositionIntoViewport();
      this.focusBoundary(false);
    }, 0);
  }

  private enabledButtons(): HTMLButtonElement[] {
    const root = this.menu?.nativeElement;
    if (!root) return [];
    return Array.from(root.querySelectorAll<HTMLButtonElement>('button[role="menuitem"]'))
      .filter(button => !button.disabled);
  }

  private buttonForItemIndex(index: number): HTMLButtonElement | null {
    return this.menu?.nativeElement.querySelector<HTMLButtonElement>(
      `button[data-menu-index="${index}"]`,
    ) ?? null;
  }

  private focusRelative(delta: -1 | 1): void {
    const buttons = this.enabledButtons();
    if (!buttons.length) return;

    const active = document.activeElement as HTMLButtonElement | null;
    const current = buttons.indexOf(active as HTMLButtonElement);
    const next = current < 0
      ? (delta > 0 ? 0 : buttons.length - 1)
      : (current + delta + buttons.length) % buttons.length;

    buttons[next].focus({ preventScroll: true });
    buttons[next].scrollIntoView({ block: 'nearest' });
  }

  private focusBoundary(end: boolean): void {
    const buttons = this.enabledButtons();
    if (!buttons.length) {
      this.menu?.nativeElement.focus({ preventScroll: true });
      return;
    }

    const button = end ? buttons[buttons.length - 1] : buttons[0];
    button.focus({ preventScroll: true });
    button.scrollIntoView({ block: 'nearest' });
  }

  private handleTypeahead(key: string): void {
    const buttons = this.enabledButtons();
    if (!buttons.length) return;

    this.typeahead += key.toLocaleLowerCase();
    if (this.typeaheadTimer) clearTimeout(this.typeaheadTimer);
    this.typeaheadTimer = setTimeout(() => this.clearTypeahead(), 650);

    const active = document.activeElement as HTMLButtonElement | null;
    const activeIndex = buttons.indexOf(active as HTMLButtonElement);
    const ordered = [
      ...buttons.slice(activeIndex + 1),
      ...buttons.slice(0, activeIndex + 1),
    ];

    const match = ordered.find(button =>
      (button.querySelector('.label')?.textContent ?? '')
        .trim()
        .toLocaleLowerCase()
        .startsWith(this.typeahead),
    );

    if (match) {
      match.focus({ preventScroll: true });
      match.scrollIntoView({ block: 'nearest' });
    }
  }

  private repositionIntoViewport(): void {
    const element = this.menu?.nativeElement;
    if (!element || typeof window === 'undefined') return;

    const rect = element.getBoundingClientRect();
    const margin = 8;
    let x = this.menuX;
    let y = this.menuY;

    if (rect.right > window.innerWidth - margin) {
      x = Math.max(margin, window.innerWidth - rect.width - margin);
    }
    if (rect.bottom > window.innerHeight - margin) {
      y = Math.max(margin, window.innerHeight - rect.height - margin);
    }

    if (x !== this.menuX || y !== this.menuY) {
      this.menuX = x;
      this.menuY = y;
      this.cdr.markForCheck();
    }
  }

  private clearFocusTimer(): void {
    if (!this.focusTimer) return;
    clearTimeout(this.focusTimer);
    this.focusTimer = null;
  }

  private clearTypeahead(): void {
    this.typeahead = '';
    if (this.typeaheadTimer) {
      clearTimeout(this.typeaheadTimer);
      this.typeaheadTimer = null;
    }
  }
}
