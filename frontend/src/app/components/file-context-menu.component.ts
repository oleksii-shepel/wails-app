import { CommonModule } from '@angular/common';
import { Component, EventEmitter, HostListener, Input, Output } from '@angular/core';

export interface FileContextMenuItemVm {
  readonly id: string;
  readonly label: string;
  readonly shortcut?: string;
  readonly icon?: string;
  readonly enabled: boolean;
  readonly separatorBefore?: boolean;
  readonly execute: () => void | Promise<void>;
}

export interface FileContextMenuState {
  readonly x: number;
  readonly y: number;
  readonly items: readonly FileContextMenuItemVm[];
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
      [style.left.px]="state.x"
      [style.top.px]="state.y"
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
    .context-menu-item:focus-visible:not(:disabled) {
      outline: none;
      background: var(--vsc-list-hover-background);
      color: var(--vsc-list-hover-foreground, var(--vsc-foreground));
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
export class FileContextMenuComponent {
  @Input() state: FileContextMenuState | null = null;
  @Output() closed = new EventEmitter<void>();

  trackItem(_index: number, item: FileContextMenuItemVm): string {
    return item.id;
  }

  run(item: FileContextMenuItemVm): void {
    if (!item.enabled) return;
    this.closed.emit();
    Promise.resolve(item.execute()).catch(error => {
      console.error(`Context menu action ${item.id} failed:`, error);
    });
  }

  @HostListener('document:mousedown')
  onDocumentMouseDown(): void {
    if (this.state) this.closed.emit();
  }

  @HostListener('document:keydown.escape')
  onEscape(): void {
    if (this.state) this.closed.emit();
  }

  @HostListener('window:blur')
  onWindowBlur(): void {
    if (this.state) this.closed.emit();
  }
}
