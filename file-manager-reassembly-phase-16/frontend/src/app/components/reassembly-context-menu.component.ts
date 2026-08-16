import {
  ChangeDetectionStrategy,
  Component,
  EventEmitter,
  Input,
  Output,
} from '@angular/core';
import { CommonModule } from '@angular/common';

export interface ReassemblyContextMenuItem {
  commandId?: string;
  label?: string;
  shortcut?: string;
  enabled?: boolean;
  separator?: boolean;
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
      (contextmenu)="$event.preventDefault(); closed.emit()">

      <section
        class="menu"
        role="menu"
        [style.left.px]="state.x"
        [style.top.px]="state.y"
        (mousedown)="$event.stopPropagation()"
        (contextmenu)="$event.preventDefault(); $event.stopPropagation()">

        <div class="title" *ngIf="state.title">{{ state.title }}</div>

        <ng-container *ngFor="let item of state.items">
          <div *ngIf="item.separator" class="separator" role="separator"></div>

          <button
            *ngIf="!item.separator"
            type="button"
            role="menuitem"
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
      width: min(250px, calc(100vw - 16px));
      max-height: calc(100vh - 16px);
      overflow: auto;
      padding: 4px;
      border: 1px solid var(--vsc-border, #454545);
      border-radius: 6px;
      background: var(--vsc-menu-background, #252526);
      color: var(--vsc-menu-foreground, #cccccc);
      box-shadow: 0 8px 30px rgba(0, 0, 0, .4);
      font: 12px/1.35 var(--vsc-font-family, system-ui, sans-serif);
    }

    .title {
      padding: 5px 8px 6px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-description-foreground, #969696);
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
      background: var(--vsc-list-hover-background, #2a2d2e);
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
      color: var(--vsc-description-foreground, #969696);
      font-size: 10px;
    }

    .separator {
      height: 1px;
      margin: 4px 5px;
      background: var(--vsc-border, #454545);
    }
  `],
})
export class ReassemblyContextMenuComponent {
  @Input() state: ReassemblyContextMenuState | null = null;
  @Output() command = new EventEmitter<string>();
  @Output() closed = new EventEmitter<void>();

  choose(item: ReassemblyContextMenuItem): void {
    if (!item.commandId || item.enabled === false) return;
    this.command.emit(item.commandId);
  }
}
