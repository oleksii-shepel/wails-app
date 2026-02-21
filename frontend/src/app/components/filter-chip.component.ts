import { Component, Input, Output, EventEmitter, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FilterPreset } from './filter-bar.types';

@Component({
  selector: 'app-filter-chip',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <button class="chip" [class.chip--dynamic]="preset.isDynamic"
      [title]="preset.description || preset.name" (click)="remove.emit(preset)">
      <span class="chip-icon">{{ preset.icon || '🔍' }}</span>
      <span class="chip-label">{{ preset.name }}</span>
      <span class="chip-order" *ngIf="showOrder && order > 0">{{ order }}</span>
      <svg class="chip-x" width="8" height="8" viewBox="0 0 16 16" fill="currentColor">
        <path d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"/>
      </svg>
    </button>
  `,
  styles: [`
    .chip {
      display: inline-flex; align-items: center; gap: 3px;
      padding: 1px 5px 1px 4px; height: 20px;
      border: 1px solid var(--vsc-border-subtle);
      border-radius: var(--vsc-border-radius-sm);
      background: var(--vsc-list-inactive-selection);
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size-small); font-family: var(--vsc-font-family);
      cursor: pointer; white-space: nowrap;
      transition: background var(--vsc-transition-fast);
    }
    .chip:hover { background: var(--vsc-list-hover-background); }
    .chip-icon { font-size: 11px; }
    .chip-order {
      display: inline-flex; align-items: center; justify-content: center;
      width: 13px; height: 13px; border-radius: 50%;
      background: var(--vsc-accent-blue); color: #fff; font-size: 9px; font-weight: 700;
    }
    .chip-x { opacity: 0.45; flex-shrink: 0; transition: opacity var(--vsc-transition-fast); }
    .chip:hover .chip-x { opacity: 1; }
  `],
})
export class FilterChipComponent {
  @Input() preset!: FilterPreset;
  @Input() order = 0;
  @Input() showOrder = false;
  @Output() remove = new EventEmitter<FilterPreset>();
}
