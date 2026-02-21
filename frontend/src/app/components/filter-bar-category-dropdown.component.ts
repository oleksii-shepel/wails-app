// filter-bar-category-dropdown.component.ts
import { Component, Input, Output, EventEmitter, ChangeDetectionStrategy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FilterCategory } from './filter-bar.types';

@Component({
  selector: 'app-filter-bar-category-dropdown',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div
      class="fb-dropdown"
      [ngStyle]="{ left: left + 'px', top: top + 'px', position: 'fixed' }"
    >
      <div class="fb-drop-head">
        <span>{{ headLabel }}</span>
        <span class="fb-badge">{{ categories.length }}</span>
      </div>

      <!-- Built-in categories section -->
      <div class="fb-drop-section" *ngIf="builtInCategories.length">
        <div class="fb-drop-section-title">Built-in</div>
        <div
          *ngFor="let cat of builtInCategories"
          class="fb-drop-item"
          [class.fb-drop-item--active]="isSelected(cat.id)"
          [class.fb-drop-item--custom]="cat.isCustom"
          (click)="onCategoryClick(cat)"
        >
          <span class="fb-item-icon">{{ cat.icon || '📌' }}</span>
          <span class="fb-item-label">{{ cat.label }}</span>
          <span class="fb-item-check" *ngIf="isSelected(cat.id)">✓</span>
        </div>
      </div>

      <!-- Custom categories section -->
      <div class="fb-drop-section" *ngIf="customCategories.length">
        <div class="fb-drop-section-title">
          Custom
          <button
            class="fb-add-btn"
            (click)="onAddCustom($event)"
            title="Add new custom category"
          >+</button>
        </div>
        <div
          *ngFor="let cat of customCategories"
          class="fb-drop-item fb-drop-item--custom"
          [class.fb-drop-item--active]="isSelected(cat.id)"
          (click)="onCategoryClick(cat)"
        >
          <span class="fb-item-icon">{{ cat.icon || '🏷️' }}</span>
          <span class="fb-item-label">{{ cat.label }}</span>
          <button
            class="fb-item-edit"
            (click)="onEditCustom(cat, $event)"
            title="Edit category"
          >✎</button>
          <button
            class="fb-item-delete"
            (click)="onDeleteCustom(cat, $event)"
            title="Delete category"
          >×</button>
          <span class="fb-item-check" *ngIf="isSelected(cat.id)">✓</span>
        </div>
      </div>

      <div class="fb-drop-footer" *ngIf="!builtInCategories.length && !customCategories.length">
        <span class="fb-empty">No categories available</span>
      </div>
    </div>
  `,
  styles: [`
    :host { display: contents; }

    .fb-dropdown {
      min-width: 240px;
      max-width: 320px;
      background-color: var(--vsc-panel-background, #252526);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      z-index: 10000;
      overflow: hidden;
      animation: fb-appear var(--vsc-transition-fast) cubic-bezier(0.4, 0, 0.2, 1);
    }

    @keyframes fb-appear {
      from { opacity: 0; transform: translateY(-4px); }
      to { opacity: 1; transform: translateY(0); }
    }

    .fb-drop-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 32px;
      padding: 0 var(--vsc-padding-lg);
      background: var(--vsc-panel-background);
      border-bottom: 1px solid var(--vsc-border-subtle);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-dim);
      text-transform: uppercase;
      letter-spacing: 0.5px;
      font-weight: 600;
    }

    .fb-badge {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 20px;
      height: 20px;
      padding: 0 var(--vsc-padding-xs);
      background: var(--vsc-badge-background);
      color: var(--vsc-badge-foreground);
      border-radius: 10px;
      font-size: 11px;
      font-weight: 600;
    }

    .fb-drop-section {
      padding: var(--vsc-padding-xs) 0;
      border-bottom: 1px solid var(--vsc-border-subtle);
      
      &:last-child {
        border-bottom: none;
      }
    }

    .fb-drop-section-title {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: var(--vsc-padding-xs) var(--vsc-padding-lg);
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      text-transform: uppercase;
    }

    .fb-add-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 18px;
      height: 18px;
      padding: 0;
      background: transparent;
      border: 1px solid var(--vsc-border);
      border-radius: 4px;
      color: var(--vsc-foreground-dim);
      font-size: 14px;
      cursor: pointer;
      
      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground-bright);
      }
    }

    .fb-drop-item {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      height: 28px;
      padding: 0 var(--vsc-padding-lg);
      cursor: pointer;
      transition: background-color var(--vsc-transition-fast);
      border-left: 2px solid transparent;
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);
      
      &:hover {
        background: var(--vsc-list-hover-background);
      }
      
      &.fb-drop-item--active {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
        border-left-color: var(--vsc-accent-blue);
      }
      
      &.fb-drop-item--custom {
        .fb-item-label {
          font-style: italic;
        }
      }
    }

    .fb-item-icon {
      font-size: 14px;
      flex-shrink: 0;
      opacity: 0.8;
    }

    .fb-item-label {
      flex: 1;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .fb-item-edit,
    .fb-item-delete {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 20px;
      height: 20px;
      padding: 0;
      background: transparent;
      border: none;
      border-radius: 4px;
      color: var(--vsc-foreground-dim);
      font-size: 14px;
      cursor: pointer;
      opacity: 0;
      transition: opacity var(--vsc-transition-fast);
      
      .fb-drop-item:hover & {
        opacity: 1;
      }
      
      &:hover {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
      }
    }

    .fb-item-check {
      margin-left: auto;
      color: var(--vsc-accent-green);
      font-weight: 600;
    }

    .fb-drop-footer {
      padding: var(--vsc-padding-lg);
      text-align: center;
      color: var(--vsc-foreground-dim);
      font-size: var(--vsc-font-size-small);
    }

    .fb-empty {
      display: block;
      padding: var(--vsc-padding-md);
      text-align: center;
      color: var(--vsc-foreground-dim);
      font-style: italic;
    }
  `]
})
export class FilterBarCategoryDropdownComponent {
  @Input() categories: FilterCategory[] = [];
  @Input() selectedCategoryIds: string[] = [];
  @Input() left = 0;
  @Input() top = 0;
  @Input() headLabel = 'Filter by category';
  
  @Output() categorySelect = new EventEmitter<FilterCategory>();
  @Output() addCustom = new EventEmitter<MouseEvent>();
  @Output() editCustom = new EventEmitter<{category: FilterCategory, event: MouseEvent}>();
  @Output() deleteCustom = new EventEmitter<{category: FilterCategory, event: MouseEvent}>();

  get builtInCategories(): FilterCategory[] {
    return this.categories.filter(c => !c.isCustom);
  }

  get customCategories(): FilterCategory[] {
    return this.categories.filter(c => c.isCustom);
  }

  isSelected(categoryId: string): boolean {
    return this.selectedCategoryIds.includes(categoryId);
  }

  onCategoryClick(category: FilterCategory): void {
    this.categorySelect.emit(category);
  }

  onAddCustom(event: MouseEvent): void {
    event.stopPropagation();
    this.addCustom.emit(event);
  }

  onEditCustom(category: FilterCategory, event: MouseEvent): void {
    event.stopPropagation();
    this.editCustom.emit({ category, event });
  }

  onDeleteCustom(category: FilterCategory, event: MouseEvent): void {
    event.stopPropagation();
    this.deleteCustom.emit({ category, event });
  }
}