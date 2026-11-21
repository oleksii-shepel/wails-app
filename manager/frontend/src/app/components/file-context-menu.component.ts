import { CommonModule } from '@angular/common';
import { Component, ElementRef, EventEmitter, HostListener, Input, OnChanges, OnDestroy, Output, SimpleChanges, ViewChild } from '@angular/core';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';
import type { PresentedCommand } from './presented-command';

export interface FileContextMenuState {
  readonly x: number;
  readonly y: number;
  readonly items: readonly PresentedCommand[];
}

@Component({
  selector: 'app-file-context-menu',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div
      *ngIf="state"
      class="context-menu"
      role="menu"
      tabindex="-1"
      #menu
      [style.left.px]="state.x"
      [style.top.px]="state.y"
      (keydown)="onMenuKeyDown($event)"
      (mousedown)="$event.stopPropagation()"
      (contextmenu)="$event.preventDefault(); $event.stopPropagation()">
      <ng-container *ngFor="let item of state.items; trackBy: trackItem">
        <div *ngIf="item.separatorBefore" class="context-menu-separator" role="separator"></div>
        <button
          type="button"
          class="context-menu-item"
          role="menuitem"
          [disabled]="!item.enabled"
          [attr.aria-disabled]="!item.enabled"
          (click)="run(item)">
          <span class="context-menu-icon">{{ item.icon || '' }}</span>
          <span class="context-menu-label">{{ item.label }}</span>
          <span *ngIf="item.shortcut" class="context-menu-shortcut">{{ item.shortcut }}</span>
        </button>
      </ng-container>
    </div>
  `,
  styles: [`
    .context-menu {
      position: fixed;
      z-index: 20000;
      min-width: 220px;
      max-width: min(320px, calc(100vw - 16px));
      padding: 4px 0;
      background: var(--vsc-menu-background, var(--vsc-editor-background));
      color: var(--vsc-menu-foreground, var(--vsc-foreground));
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm);
      box-shadow: var(--vsc-widget-shadow);
      user-select: none;
    }

    .context-menu-item {
      width: 100%;
      min-height: 28px;
      display: grid;
      grid-template-columns: 18px minmax(0, 1fr) auto;
      align-items: center;
      gap: 8px;
      padding: 4px 10px;
      border: 0;
      background: transparent;
      color: inherit;
      text-align: left;
      font: inherit;
      cursor: pointer;
    }

    .context-menu-item:hover:not(:disabled),
    .context-menu-item:focus:not(:disabled),
    .context-menu-item:focus-visible:not(:disabled) {
      outline: none;
      background: var(--vsc-list-hover-background);
      color: var(--vsc-list-hover-foreground, var(--vsc-foreground));
    }

    .context-menu-item:focus:not(:focus-visible) {
      outline: none;
    }

    .context-menu-item:disabled {
      opacity: 0.42;
      cursor: default;
    }

    .context-menu-icon {
      width: 18px;
      text-align: center;
      opacity: 0.85;
    }

    .context-menu-label {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .context-menu-shortcut {
      margin-left: 18px;
      color: var(--vsc-foreground-dim);
      font-size: 11px;
      white-space: nowrap;
    }

    .context-menu-separator {
      height: 1px;
      margin: 4px 0;
      background: var(--vsc-border-subtle, var(--vsc-border));
    }
  `],
})
export class FileContextMenuComponent implements OnChanges, OnDestroy {
  @Input() state: FileContextMenuState | null = null;
  @Output() closed = new EventEmitter<void>();
  @ViewChild('menu') menu?: ElementRef<HTMLElement>;

  private typeahead = '';
  private typeaheadTimer: ReturnType<typeof setTimeout> | null = null;
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;

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
        id: 'file-context-menu',
        element: this.hostElement.nativeElement,
        dismiss: () => this.closed.emit(),
        dismissible: () => !!this.state,
      });
    } else if (this.modalStack && this.unregisterModal) {
      this.modalStack.bringToFront('file-context-menu');
    }

    queueMicrotask(() => {
      this.menu?.nativeElement.focus({ preventScroll: true });
      this.focusItem(0, 1);
    });
  }

  ngOnDestroy(): void {
    this.clearTypeahead();
    this.unregisterModal?.();
    this.unregisterModal = null;
  }

  trackItem(_index: number, item: PresentedCommand): string {
    return item.id;
  }

  run(item: PresentedCommand): void {
    if (!item.enabled) return;
    this.closed.emit();
    Promise.resolve(item.execute()).catch(error => {
      console.error(`Context menu action ${item.id} failed:`, error);
    });
  }

  onMenuKeyDown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
      if (this.modalStack && !this.modalStack.isTopmost('file-context-menu')) return;
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

  @HostListener('document:mousedown')
  onDocumentMouseDown(): void {
    if (this.state) this.closed.emit();
  }

  @HostListener('document:keydown.escape', ['$event'])
  onEscape(event?: Event): void {
    if (!this.state) return;
    if (this.modalStack && !this.modalStack.isTopmost('file-context-menu')) return;
    event?.preventDefault();
    event?.stopPropagation();
    this.closed.emit();
  }

  @HostListener('window:blur')
  onWindowBlur(): void {
    if (this.state) this.closed.emit();
  }

  private items(): HTMLButtonElement[] {
    return Array.from(
      this.menu?.nativeElement.querySelectorAll<HTMLButtonElement>(
        'button[role="menuitem"]',
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

  private clearTypeahead(): void {
    this.typeahead = '';
    if (this.typeaheadTimer) {
      clearTimeout(this.typeaheadTimer);
      this.typeaheadTimer = null;
    }
  }
}
