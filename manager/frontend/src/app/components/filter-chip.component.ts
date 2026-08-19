import { Component, Input, Output, EventEmitter, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FilterPreset } from './filter-bar.types';

@Component({
  selector: 'app-filter-chip',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="fb-chip"
      [class.fb-chip--dynamic]="preset.isDynamic"
      [title]="preset.description || preset.name"
    >
      <span class="fb-chip-icon">{{ preset.icon || '🔍' }}</span>
      <span class="fb-chip-label">{{ preset.name }}</span>
      <button
        class="fb-chip-close"
        (click)="$event.stopPropagation(); remove.emit(preset)"
        title="Remove"
      >×</button>
    </div>
  `,
  styles: [`
    :host { display: contents; }

    .fb-chip {
      display: inline-flex;
      align-items: center;
      gap: 3px;
      height: 18px;
      padding: 0 6px;
      background: transparent;
      border: 1px solid var(--vsc-border);
      border-radius: 2px;
      flex-shrink: 0;
      color: var(--vsc-foreground-dim);
      font-size: 11.5px;
      font-family: var(--vsc-font-family);
      transition:
        background var(--vsc-transition-fast),
        border-color var(--vsc-transition-fast),
        color var(--vsc-transition-fast);

      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground);
      }

      &.fb-chip--dynamic {
        border-color: color-mix(in srgb, var(--vsc-accent-green) 40%, var(--vsc-border));
      }
    }

    .fb-chip-icon  { font-size: 11px; line-height: 1; flex-shrink: 0; }
    .fb-chip-label { white-space: nowrap; line-height: 1; max-width: 120px; overflow: hidden; text-overflow: ellipsis; }

    .fb-chip-close {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 12px;
      height: 12px;
      margin-left: 1px;
      background: transparent;
      border: none;
      border-radius: 2px;
      color: currentColor;
      font-size: 14px;
      line-height: 1;
      padding: 0;
      cursor: pointer;
      opacity: 0.55;
      transition: opacity var(--vsc-transition-fast), background var(--vsc-transition-fast);

      &:hover {
        opacity: 1;
        background: color-mix(in srgb, var(--vsc-foreground) 15%, transparent);
      }
    }
  `],
})
export class FilterChipComponent {
  @Input() preset!: FilterPreset;
  @Output() remove = new EventEmitter<FilterPreset>();
}