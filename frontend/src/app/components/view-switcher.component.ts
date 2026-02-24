// view-switcher.component.ts
import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';

export type ViewMode = 'list' | 'grid' | 'tree' | 'details';

export interface ViewOption {
  mode: ViewMode;
  label: string;
  shortcut?: string;
}

@Component({
  selector: 'app-view-switcher',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="view-switcher" role="toolbar" aria-label="View options">
      <button
        *ngFor="let option of viewOptions"
        class="view-btn"
        [class.active]="currentView === option.mode"
        [attr.aria-pressed]="currentView === option.mode"
        [title]="option.label + (option.shortcut ? ' (' + option.shortcut + ')' : '')"
        (click)="onViewChange(option.mode)">
        <ng-container [ngSwitch]="option.mode">

          <!-- List: three full-width horizontal bars -->
          <svg *ngSwitchCase="'list'" width="16" height="16" viewBox="0 0 16 16" fill="none">
            <line x1="2" y1="4"  x2="14" y2="4"  stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
            <line x1="2" y1="8"  x2="14" y2="8"  stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
            <line x1="2" y1="12" x2="14" y2="12" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
          </svg>

          <!-- Grid: 2x2 squares -->
          <svg *ngSwitchCase="'grid'" width="16" height="16" viewBox="0 0 16 16" fill="none">
            <rect x="2"  y="2"  width="5" height="5" rx="1" stroke="currentColor" stroke-width="1"/>
            <rect x="9"  y="2"  width="5" height="5" rx="1" stroke="currentColor" stroke-width="1"/>
            <rect x="2"  y="9"  width="5" height="5" rx="1" stroke="currentColor" stroke-width="1"/>
            <rect x="9"  y="9"  width="5" height="5" rx="1" stroke="currentColor" stroke-width="1"/>
          </svg>

          <!-- Tree: branch structure -->
          <svg *ngSwitchCase="'tree'" width="16" height="16" viewBox="0 0 16 16" fill="none">
            <circle cx="3"  cy="3"  r="1.5" fill="currentColor"/>
            <circle cx="3"  cy="8"  r="1.5" fill="currentColor"/>
            <circle cx="3"  cy="13" r="1.5" fill="currentColor"/>
            <circle cx="12" cy="5"  r="1.5" fill="currentColor"/>
            <circle cx="12" cy="11" r="1.5" fill="currentColor"/>
            <!-- vertical spine -->
            <line x1="3" y1="4.5" x2="3" y2="11.5" stroke="currentColor" stroke-width="1"/>
            <!-- branches -->
            <line x1="3" y1="5"  x2="10.5" y2="5"  stroke="currentColor" stroke-width="1"/>
            <line x1="3" y1="11" x2="10.5" y2="11" stroke="currentColor" stroke-width="1"/>
          </svg>

          <!-- Details: icon + three rows of text lines -->
          <svg *ngSwitchCase="'details'" width="16" height="16" viewBox="0 0 16 16" fill="none">
            <rect x="2" y="2" width="3" height="3" rx="0.5" fill="currentColor" opacity="0.8"/>
            <rect x="2" y="7" width="3" height="3" rx="0.5" fill="currentColor" opacity="0.8"/>
            <rect x="2" y="12" width="3" height="2" rx="0.5" fill="currentColor" opacity="0.8"/>
            <line x1="7" y1="3.5" x2="14" y2="3.5" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
            <line x1="7" y1="8.5" x2="14" y2="8.5" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
            <line x1="7" y1="13" x2="14" y2="13" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
          </svg>

        </ng-container>
      </button>

      <div class="view-divider"></div>

      <button
        class="view-btn sort-btn"
        [class.active]="sortMenuOpen"
        (click)="toggleSortMenu($event)"
        title="Sort by">
        <!-- Sort: two arrows up/down with lines of different widths -->
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
          <line x1="2"  y1="5" x2="9"  y2="5" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
          <line x1="2"  y1="8" x2="7"  y2="8" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
          <line x1="2"  y1="11" x2="5" y2="11" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
          <path d="M12 2 L12 13 M10 11 L12 13 L14 11" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </button>

      <div class="sort-menu" *ngIf="sortMenuOpen" (click)="$event.stopPropagation()">
        <div
          class="sort-item"
          *ngFor="let sort of sortOptions"
          [class.active]="currentSort === sort.value"
          (click)="onSortChange(sort.value)">
          <span class="sort-check">
            <svg *ngIf="currentSort === sort.value" width="12" height="12" viewBox="0 0 12 12" fill="none">
              <path d="M2 6L5 9L10 3" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </span>
          <span class="sort-label">{{ sort.label }}</span>
        </div>
        <div class="sort-divider"></div>
        <div class="sort-item" (click)="toggleSortDirection()">
          <span class="sort-check">
            <!-- Up arrow for asc, down arrow for desc -->
            <svg *ngIf="sortDirection === 'asc'" width="12" height="12" viewBox="0 0 12 12" fill="none">
              <path d="M6 10 L6 2 M3 5 L6 2 L9 5" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <svg *ngIf="sortDirection === 'desc'" width="12" height="12" viewBox="0 0 12 12" fill="none">
              <path d="M6 2 L6 10 M3 7 L6 10 L9 7" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </span>
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

      &:hover {
        background: var(--vsc-tab-hover-background);
        color: var(--vsc-tab-hover-foreground);
      }

      &.active {
        background: var(--vsc-tab-active-background);
        color: var(--vsc-tab-active-foreground);
      }

      svg {
        display: block;
        flex-shrink: 0;
      }
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
      height: 16px;
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;
    }

    .sort-divider {
      height: 1px;
      background: var(--vsc-border);
      margin: var(--vsc-padding-xs) 0;
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
    { mode: 'list',    label: 'List View',    shortcut: 'Ctrl+1' },
    { mode: 'grid',    label: 'Grid View',    shortcut: 'Ctrl+2' },
    { mode: 'tree',    label: 'Tree View',    shortcut: 'Ctrl+3' },
    { mode: 'details', label: 'Details View', shortcut: 'Ctrl+4' },
  ];

  sortOptions = [
    { value: 'name',     label: 'Name' },
    { value: 'size',     label: 'Size' },
    { value: 'modified', label: 'Date Modified' },
    { value: 'type',     label: 'Type' },
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