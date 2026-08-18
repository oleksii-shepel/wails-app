import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  OnChanges,
  Output,
  SimpleChanges,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

export interface ContextMenuItem {
  commandId?: string;
  label?: string;
  shortcut?: string;
  enabled?: boolean;
  separator?: boolean;
}

export interface ContextMenuState {
  x: number;
  y: number;
  title?: string;
  items: ContextMenuItem[];
}

@Component({
  selector: 'app-context-menu',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      *ngIf="state"
      class="backdrop"
      (mousedown)="closed.emit()"
      (contextmenu)="$event.preventDefault(); closed.emit()">

      <section
        #menu
        class="menu"
        role="menu"
        aria-label="Context menu"
        [style.left.px]="menuLeft"
        [style.top.px]="menuTop"
        (keydown)="onMenuKeyDown($event)"
        (mousedown)="$event.stopPropagation()"
        (contextmenu)="$event.preventDefault(); $event.stopPropagation()">

        <div class="title" *ngIf="state.title">{{ state.title }}</div>

        <ng-container *ngFor="let item of state.items">
          <div *ngIf="item.separator" class="separator" role="separator"></div>

          <button
            *ngIf="!item.separator"
            type="button"
            role="menuitem"
            data-context-menu-item
            [disabled]="item.enabled === false"
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
      width: max-content;
      min-width: min(220px, calc(100vw - 16px));
      max-width: calc(100vw - 16px);
      max-height: calc(100vh - 16px);
      overflow: visible;
      columns: 230px auto;
      column-fill: auto;
      column-gap: 4px;
      padding: 4px;
      border: 1px solid var(--vsc-border, var(--vsc-border));
      border-radius: 6px;
      background: var(--vsc-menu-background, var(--vsc-panel-background));
      color: var(--vsc-menu-foreground, var(--vsc-foreground));
      box-shadow: 0 8px 30px var(--vsc-scrim);
      font: 12px/1.35 var(--vsc-font-family, system-ui, sans-serif);
    }

    .title {
      column-span: all;
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
      break-inside: avoid;
    }

    button:hover:not(:disabled),
    button:focus-visible:not(:disabled) {
      outline: none;
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    button:disabled {
      opacity: .38;
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
    }

    .separator {
      break-inside: avoid;
      height: 1px;
      margin: 4px 5px;
      background: var(--vsc-border, var(--vsc-border));
    }
  `],
})
export class ContextMenuComponent implements OnChanges {
  @Input() state: ContextMenuState | null = null;
  @Output() command = new EventEmitter<string>();
  @Output() closed = new EventEmitter<void>();
  @ViewChild('menu') menu?: ElementRef<HTMLElement>;

  menuLeft = 8;
  menuTop = 8;
  private typeahead = '';
  private typeaheadTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(private readonly cdr: ChangeDetectorRef) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (!changes['state']) return;

    this.clearTypeahead();

    if (!this.state) return;

    this.menuLeft = Math.max(8, this.state.x);
    this.menuTop = Math.max(8, this.state.y);
    this.cdr.markForCheck();

    queueMicrotask(() => {
      this.fitToViewport();
      this.focusItem(0, 1);
    });
  }

  choose(item: ContextMenuItem): void {
    if (!item.commandId || item.enabled === false) return;
    this.command.emit(item.commandId);
  }

  onMenuKeyDown(event: KeyboardEvent): void {
    const items = this.items();
    if (!items.length) return;

    const active = document.activeElement as HTMLButtonElement | null;
    const current = Math.max(0, items.indexOf(active as HTMLButtonElement));

    if (event.key === 'Escape') {
      event.preventDefault();
      event.stopPropagation();
      this.closed.emit();
      return;
    }

    if (event.key === 'ArrowDown') {
      event.preventDefault();
      this.focusItem(current + 1, 1);
      return;
    }

    if (event.key === 'ArrowUp') {
      event.preventDefault();
      this.focusItem(current - 1, -1);
      return;
    }

    if (event.key === 'Home') {
      event.preventDefault();
      this.focusItem(0, 1);
      return;
    }

    if (event.key === 'End') {
      event.preventDefault();
      this.focusItem(items.length - 1, -1);
      return;
    }

    if (
      event.key.length === 1 &&
      !event.ctrlKey &&
      !event.altKey &&
      !event.metaKey
    ) {
      this.typeahead += event.key.toLocaleLowerCase();

      if (this.typeaheadTimer) clearTimeout(this.typeaheadTimer);
      this.typeaheadTimer = setTimeout(() => this.clearTypeahead(), 650);

      const match = items.find(item =>
        !item.disabled &&
        (item.textContent ?? '')
          .trim()
          .toLocaleLowerCase()
          .startsWith(this.typeahead)
      );

      if (match) {
        event.preventDefault();
        match.focus({ preventScroll: true });
      }
    }
  }

  private items(): HTMLButtonElement[] {
    return Array.from(
      this.menu?.nativeElement.querySelectorAll<HTMLButtonElement>(
        'button[data-context-menu-item]',
      ) ?? [],
    );
  }

  private focusItem(requested: number, direction: 1 | -1): void {
    const items = this.items();
    if (!items.length) return;

    let index = ((requested % items.length) + items.length) % items.length;
    for (let attempts = 0; attempts < items.length; attempts += 1) {
      const item = items[index];
      if (!item.disabled) {
        item.focus({ preventScroll: true });
        return;
      }
      index = ((index + direction) % items.length + items.length) % items.length;
    }
  }

  private fitToViewport(): void {
    const menu = this.menu?.nativeElement;
    const state = this.state;
    if (!menu || !state || typeof window === 'undefined') return;

    const margin = 8;
    const rect = menu.getBoundingClientRect();
    const maxLeft = Math.max(margin, window.innerWidth - rect.width - margin);
    const maxTop = Math.max(margin, window.innerHeight - rect.height - margin);

    this.menuLeft = Math.min(Math.max(margin, state.x), maxLeft);
    this.menuTop = Math.min(Math.max(margin, state.y), maxTop);
    this.cdr.markForCheck();
  }

  private clearTypeahead(): void {
    this.typeahead = '';
    if (this.typeaheadTimer) {
      clearTimeout(this.typeaheadTimer);
      this.typeaheadTimer = null;
    }
  }
}
