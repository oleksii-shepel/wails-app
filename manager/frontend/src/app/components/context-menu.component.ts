import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  OnChanges,
  OnDestroy,
  Output,
  SimpleChanges,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';

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
        tabindex="-1"
        aria-label="Context menu"
        [style.left.px]="menuLeft"
        [style.top.px]="menuTop"
        [style.width.px]="menuWidth"
        [style.height.px]="menuHeight"
        [style.columnCount]="columnCount"
        (keydown)="onMenuKeyDown($event)"
        (mousedown)="$event.stopPropagation()"
        (contextmenu)="$event.preventDefault(); $event.stopPropagation()">

        <div class="title" *ngIf="state.title" [title]="state.title">
          {{ state.title }}
        </div>

        <ng-container *ngFor="let item of state.items">
          <div
            *ngIf="item.separator"
            class="separator"
            role="separator">
          </div>

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
      z-index: 20000;
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

      min-width: min(230px, calc(100vw - 16px));
      max-width: calc(100vw - 16px);
      max-height: calc(100vh - 16px);

      column-width: 230px;
      column-gap: 4px;
      column-fill: auto;

      overflow: hidden;
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
    button:focus:not(:disabled),
    button:focus-visible:not(:disabled) {
      outline: none;
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    button:focus:not(:focus-visible) {
      outline: none;
    }

    button:disabled {
      opacity: .38;
      cursor: default;
    }

    .label {
      min-width: 0;
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
      break-inside: avoid;
      height: 1px;
      margin: 4px 5px;
      background: var(--vsc-border, var(--vsc-border));
    }
  `],
})
export class ContextMenuComponent implements OnChanges, OnDestroy {
  @Input() state: ContextMenuState | null = null;
  @Output() command = new EventEmitter<string>();
  @Output() closed = new EventEmitter<void>();
  @ViewChild('menu') menu?: ElementRef<HTMLElement>;

  menuLeft = 8;
  menuTop = 8;
  menuWidth = 230;
  menuHeight = 0;
  columnCount = 1;

  private typeahead = '';
  private typeaheadTimer: ReturnType<typeof setTimeout> | null = null;
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;

  private readonly viewportMargin = 8;
  private readonly columnWidth = 230;
  private readonly columnGap = 4;
  private readonly itemHeight = 36;
  private readonly separatorHeight = 9;
  private readonly titleHeight = 28;
  private readonly menuPadding = 8;

  constructor(private readonly cdr: ChangeDetectorRef) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (!changes['state']) return;

    this.clearTypeahead();

    if (!this.state) {
      this.unregisterModal?.();
      this.unregisterModal = null;
      return;
    }

    if (this.modalStack && this.hostElement && !this.unregisterModal) {
      this.unregisterModal = this.modalStack.register({
        id: 'context-menu',
        element: this.hostElement.nativeElement,
        dismiss: () => this.closed.emit(),
        dismissible: () => !!this.state,
      });
    } else if (this.modalStack && this.unregisterModal) {
      this.modalStack.bringToFront('context-menu');
    }

    this.menuLeft = Math.max(this.viewportMargin, this.state.x);
    this.menuTop = Math.max(this.viewportMargin, this.state.y);
    this.layoutMenu();
    this.cdr.markForCheck();

    queueMicrotask(() => {
      this.fitToViewport();
      this.menu?.nativeElement.focus({ preventScroll: true });
      this.focusItem(0, 1);
    });
  }

  ngOnDestroy(): void {
    this.clearTypeahead();
    this.unregisterModal?.();
    this.unregisterModal = null;
  }

  choose(item: ContextMenuItem): void {
    if (!item.commandId || item.enabled === false) return;
    this.command.emit(item.commandId);
  }

  onMenuKeyDown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      if (this.modalStack && !this.modalStack.isTopmost('context-menu')) return;
      event.preventDefault();
      event.stopPropagation();
      this.closed.emit();
      return;
    }

    const items = this.items();
    if (!items.length) return;

    const active = document.activeElement as HTMLButtonElement | null;
    const current = Math.max(0, items.indexOf(active as HTMLButtonElement));

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

  private layoutMenu(): void {
    const state = this.state;
    if (!state || typeof window === 'undefined') return;

    const commandCount = state.items.reduce(
      (count, item) => count + (item.separator ? 0 : 1),
      0,
    );
    const separatorCount = state.items.length - commandCount;

    const availableHeight = Math.max(
      120,
      window.innerHeight - this.viewportMargin * 2,
    );
    const availableWidth = Math.max(
      this.columnWidth,
      window.innerWidth - this.viewportMargin * 2,
    );

    const fixedHeight =
      this.menuPadding +
      (state.title ? this.titleHeight : 0);

    const contentHeight =
      commandCount * this.itemHeight +
      separatorCount * this.separatorHeight;

    const usableColumnHeight = Math.max(
      this.itemHeight,
      availableHeight - fixedHeight,
    );

    const neededColumns = Math.max(
      1,
      Math.ceil(contentHeight / usableColumnHeight),
    );

    const maxColumns = Math.max(
      1,
      Math.floor(
        (availableWidth + this.columnGap) /
        (this.columnWidth + this.columnGap),
      ),
    );

    this.columnCount = Math.min(neededColumns, maxColumns);

    const estimatedColumnContentHeight = Math.ceil(
      contentHeight / this.columnCount,
    );

    this.menuHeight = Math.min(
      availableHeight,
      fixedHeight + estimatedColumnContentHeight,
    );

    this.menuWidth = Math.min(
      availableWidth,
      this.columnCount * this.columnWidth +
        (this.columnCount - 1) * this.columnGap +
        this.menuPadding,
    );
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

    const margin = this.viewportMargin;
    const rect = menu.getBoundingClientRect();

    const maxLeft = Math.max(
      margin,
      window.innerWidth - rect.width - margin,
    );
    const maxTop = Math.max(
      margin,
      window.innerHeight - rect.height - margin,
    );

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
