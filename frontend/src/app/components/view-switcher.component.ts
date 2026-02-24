// view-switcher.component.ts
import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';

export type ViewMode = 'list' | 'grid' | 'tree' | 'details';

export interface ViewOption {
  mode: ViewMode;
  icon: string;
  label: string;
  shortcut?: string;
}

@Component({
  selector: 'app-view-switcher',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="view-switcher" role="toolbar" [attr.aria-label]="'View options'">
      <button
        *ngFor="let option of viewOptions"
        class="view-btn"
        [class.active]="currentView === option.mode"
        [attr.aria-pressed]="currentView === option.mode"
        [title]="option.label + (option.shortcut ? ' (' + option.shortcut + ')' : '')"
        (click)="onViewChange(option.mode)">
        <span class="view-icon" *ngIf="option.icon !== 'tree'" [attr.data-icon]="option.icon"></span>
        <span class="view-icon" *ngIf="option.icon === 'tree'" [attr.data-icon]="option.icon">
          <!-- VS Code style arrow SVG -->
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" style="display:inline;vertical-align:middle;">
            <path d="M6 4L10 8L6 12" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </span>
      </button>
      
      <div class="view-divider"></div>
      
      <button 
        class="view-btn sort-btn"
        [class.active]="sortMenuOpen"
        (click)="toggleSortMenu($event)"
        title="Sort by">
        <span class="view-icon" data-icon="sort"></span>
      </button>
      
      <div class="sort-menu" *ngIf="sortMenuOpen" (click)="$event.stopPropagation()">
        <div class="sort-item" 
             *ngFor="let sort of sortOptions"
             [class.active]="currentSort === sort.value"
             (click)="onSortChange(sort.value)">
          <span class="sort-check" *ngIf="currentSort === sort.value">✓</span>
          <span class="sort-label">{{ sort.label }}</span>
        </div>
        <div class="sort-divider"></div>
        <div class="sort-item" (click)="toggleSortDirection()">
          <span class="sort-icon">{{ sortDirection === 'asc' ? '↑' : '↓' }}</span>
          <span class="sort-label">{{ sortDirection === 'asc' ? 'Ascending' : 'Descending' }}</span>
        </div>
      </div>
    </div>
  `,
  styles: [`
    .view-switcher {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-xs);
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      background: var(--vsc-tabbar-background);
      border-bottom: 1px solid var(--vsc-border);
      position: relative;
    }

    .view-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      border: none;
      border-radius: var(--vsc-border-radius);
      background: transparent;
      color: var(--vsc-tab-foreground);
      cursor: pointer;
      transition: all var(--vsc-transition-fast);
      
      &:hover {
        background: var(--vsc-tab-hover-background);
        color: var(--vsc-tab-hover-foreground);
      }
      
      &.active {
        background: var(--vsc-tab-active-background);
        color: var(--vsc-tab-active-foreground);
      }
    }

    .view-icon {
      font-size: 16px;
      line-height: 1;
      
      &[data-icon="list"]::before { content: "☰"; }
      &[data-icon="grid"]::before { content: "⊞"; }
      /* Tree icon now uses SVG, so no ::before needed */
      &[data-icon="details"]::before { content: "☰"; font-weight: bold; }
      &[data-icon="sort"]::before { content: "⇅"; }
    }

    .view-divider {
      width: 1px;
      height: 20px;
      background: var(--vsc-border);
      margin: 0 var(--vsc-padding-sm);
    }

    .sort-menu {
      position: absolute;
      top: calc(100% + 4px);
      right: var(--vsc-padding-md);
      min-width: 160px;
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      z-index: var(--vsc-z-dropdown);
      padding: var(--vsc-padding-xs) 0;
    }

    .sort-item {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      cursor: pointer;
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);
      
      &:hover {
        background: var(--vsc-list-hover-background);
      }
      
      &.active {
        color: var(--vsc-accent-blue);
      }
    }

    .sort-check {
      width: 16px;
      text-align: center;
    }

    .sort-divider {
      height: 1px;
      background: var(--vsc-border);
      margin: var(--vsc-padding-xs) 0;
    }

    .sort-icon {
      width: 16px;
      text-align: center;
      font-size: 12px;
    }
  `]
})
export class ViewSwitcherComponent {
  @Input() currentView: ViewMode = 'list';
  @Input() currentSort: string = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';
  
  @Output() viewChange = new EventEmitter<ViewMode>();
  @Output() sortChange = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();

  sortMenuOpen = false;

  viewOptions: ViewOption[] = [
    { mode: 'list', icon: 'list', label: 'List View', shortcut: 'Ctrl+1' },
    { mode: 'grid', icon: 'grid', label: 'Grid View', shortcut: 'Ctrl+2' },
    { mode: 'tree', icon: 'tree', label: 'Tree View', shortcut: 'Ctrl+3' },
    { mode: 'details', icon: 'details', label: 'Details View', shortcut: 'Ctrl+4' }
  ];

  sortOptions = [
    { value: 'name', label: 'Name' },
    { value: 'size', label: 'Size' },
    { value: 'modified', label: 'Date Modified' },
    { value: 'type', label: 'Type' }
  ];

  onViewChange(mode: ViewMode): void {
    if (this.currentView !== mode) {
      this.viewChange.emit(mode);
    }
  }

  toggleSortMenu(event: Event): void {
    event.stopPropagation();
    this.sortMenuOpen = !this.sortMenuOpen;
    if (this.sortMenuOpen) {
      setTimeout(() => {
        document.addEventListener('click', this.closeSortMenu, { once: true });
      });
    }
  }

  private closeSortMenu = (): void => {
    this.sortMenuOpen = false;
  };

  onSortChange(field: string): void {
    this.sortChange.emit({ field, direction: this.sortDirection });
    this.sortMenuOpen = false;
  }

  toggleSortDirection(): void {
    const newDir = this.sortDirection === 'asc' ? 'desc' : 'asc';
    this.sortChange.emit({ field: this.currentSort, direction: newDir });
    this.sortMenuOpen = false;
  }
}