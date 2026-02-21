import { Component, Input, Output, EventEmitter, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FilterPreset } from './filter-bar.types';

@Component({
  selector: 'app-filter-menu-item',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="item" [class.item--checked]="checked" [class.item--focused]="focused"
      role="menuitemcheckbox" [attr.aria-checked]="checked"
      (mouseenter)="hovered.emit()"
      (click)="toggle.emit(preset)">
      <span class="item-check" [class.item-check--on]="checked">
        <svg *ngIf="checked" width="10" height="10" viewBox="0 0 16 16" fill="currentColor">
          <path d="M13.485 1.929L5.5 9.914 2.515 6.929a1 1 0 0 0-1.414 1.414l3.692 3.693a1 1 0 0 0 1.414 0l8.692-8.693a1 1 0 0 0-1.414-1.414z"/>
        </svg>
      </span>
      <span class="item-icon">{{ preset.icon || defaultIcon }}</span>
      <span class="item-name">{{ preset.name }}</span>
      <span class="item-count" *ngIf="preset.count != null">{{ preset.count }}</span>
      <span class="item-order" *ngIf="checked && showOrder && selectionOrder > 0">{{ selectionOrder }}</span>
    </div>
  `,
  styles: [`
    .item {
      display: flex; align-items: center; gap: 6px;
      padding: 0 var(--vsc-padding-md) 0 var(--vsc-padding-sm);
      height: 22px; cursor: pointer; user-select: none;
      transition: background var(--vsc-transition-fast);
    }
    .item--focused { background: var(--vsc-list-hover-background); }
    .item--checked { background: color-mix(in srgb, var(--vsc-list-active-background) 30%, transparent); }
    .item--checked.item--focused { background: var(--vsc-list-hover-background); }
    .item-check {
      flex-shrink: 0; display: flex; align-items: center; justify-content: center;
      width: 14px; height: 14px; border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm); background: var(--vsc-input-background);
      transition: background var(--vsc-transition-fast), border-color var(--vsc-transition-fast);
    }
    .item-check--on { background: var(--vsc-accent-blue); border-color: var(--vsc-accent-blue); }
    .item-icon { font-size: 12px; flex-shrink: 0; width: 16px; text-align: center; }
    .item-name {
      flex: 1; font-size: var(--vsc-font-size-small); font-family: var(--vsc-font-family);
      color: var(--vsc-foreground); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
    }
    .item--checked .item-name { color: var(--vsc-foreground-bright); }
    .item-count {
      background: var(--vsc-badge-background); color: var(--vsc-badge-foreground);
      padding: 0 5px; border-radius: 8px; font-size: 10px; min-width: 18px; text-align: center;
    }
    .item-order {
      display: inline-flex; align-items: center; justify-content: center;
      width: 14px; height: 14px; border-radius: 50%;
      background: var(--vsc-accent-blue); color: #fff; font-size: 9px; font-weight: 700;
    }
  `],
})
export class FilterMenuItemComponent {
  @Input() preset!: FilterPreset;
  @Input() checked = false;
  @Input() focused = false;
  @Input() selectionOrder = 0;
  @Input() showOrder = false;
  @Input() defaultIcon = '📄';
  @Output() toggle  = new EventEmitter<FilterPreset>();
  @Output() hovered = new EventEmitter<void>();
}
