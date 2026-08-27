import {
  ChangeDetectionStrategy,
  Component,
  EventEmitter,
  Input,
  Output,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { DropdownContainerComponent } from './dropdown-container.component';
import { DropdownTriggerDirective } from '../directives/dropdown.directive';

export interface PaneSortChange {
  field: string;
  direction: 'asc' | 'desc';
}

@Component({
  selector: 'app-pane-arrange-controls',
  standalone: true,
  imports: [
    CommonModule,
    DropdownContainerComponent,
    DropdownTriggerDirective,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-dropdown-container [(isOpen)]="menuOpen" [backdrop]="false">
      <button
        appDropdownTrigger
        type="button"
        class="arrange-button"
        [class.active]="menuOpen"
        aria-haspopup="menu"
        [attr.aria-expanded]="menuOpen"
        title="Arrange"
        (click)="toggleMenu($event)">
        <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <line x1="2" y1="5" x2="9" y2="5" stroke="currentColor"/>
          <line x1="2" y1="8" x2="7" y2="8" stroke="currentColor"/>
          <line x1="2" y1="11" x2="5" y2="11" stroke="currentColor"/>
          <path d="M12 2v11m-2-2 2 2 2-2" stroke="currentColor"/>
        </svg>
      </button>

      <div class="arrange-menu" role="menu" aria-label="Arrange pane">
        <button
          *ngFor="let option of sortOptions"
          type="button"
          role="menuitemradio"
          class="arrange-option"
          [class.active]="currentSort === option.value"
          [attr.aria-checked]="currentSort === option.value"
          (click)="selectSort(option.value)">
          <span class="indicator">{{ currentSort === option.value ? '✓' : '' }}</span>
          <span>{{ option.label }}</span>
        </button>

        <div class="divider" role="separator"></div>

        <button type="button" role="menuitem" class="arrange-option" (click)="toggleDirection()">
          <span class="indicator">{{ sortDirection === 'asc' ? '↑' : '↓' }}</span>
          <span>{{ sortDirection === 'asc' ? 'Ascending' : 'Descending' }}</span>
        </button>

        <div class="divider" role="separator"></div>

        <button
          type="button"
          role="menuitemcheckbox"
          class="arrange-option"
          [class.active]="directoriesFirst"
          [attr.aria-checked]="directoriesFirst"
          (click)="toggleDirectoriesFirst()">
          <span class="indicator">{{ directoriesFirst ? '✓' : '' }}</span>
          <span>Directories first</span>
        </button>

        <button
          *ngIf="showColorizeByMime"
          type="button"
          role="menuitemcheckbox"
          class="arrange-option"
          [class.active]="colorizeByMime"
          [attr.aria-checked]="colorizeByMime"
          (click)="toggleColorizeByMime()">
          <span class="indicator">{{ colorizeByMime ? '✓' : '' }}</span>
          <span>Colorize by Category</span>
        </button>
      </div>
    </app-dropdown-container>
  `,
  styles: [`
    :host {
      display: block;
      flex: 0 0 auto;
    }

    .arrange-button {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      padding: 0;
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius);
      background: transparent;
      color: var(--vsc-tab-foreground);
      cursor: pointer;
    }

    .arrange-button:hover,
    .arrange-button.active {
      background: var(--vsc-tab-hover-background);
      color: var(--vsc-tab-hover-foreground);
    }

    .arrange-button svg {
      width: 16px;
      height: 16px;
    }

    .arrange-menu {
      display: flex;
      flex-direction: column;
      min-width: 192px;
      padding: 6px;
      background: var(--vsc-menu-background, var(--vsc-panel-background));
      color: var(--vsc-menu-foreground, var(--vsc-foreground));
      border: 1px solid var(--vsc-menu-border, var(--vsc-border));
      border-radius: 6px;
      box-shadow: var(--vsc-widget-shadow);
    }

    .arrange-option {
      display: grid;
      grid-template-columns: 16px minmax(0, 1fr);
      gap: var(--vsc-padding-sm);
      align-items: center;
      min-height: 28px;
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      border: 0;
      background: transparent;
      color: var(--vsc-foreground);
      cursor: pointer;
      font: inherit;
      text-align: left;
    }

    .arrange-option:hover {
      background: var(--vsc-list-hover-background);
    }

    .arrange-option.active {
      color: var(--vsc-accent-blue);
    }

    .indicator {
      width: 16px;
      text-align: center;
    }

    .divider {
      height: 1px;
      margin: var(--vsc-padding-xs) 0;
      background: var(--vsc-border);
    }

    :host-context(.compact-mode) .arrange-button {
      width: 22px;
      height: 22px;
    }

    :host-context(.compact-mode) .arrange-button svg {
      width: 14px;
      height: 14px;
    }
  `],
})
export class PaneArrangeControlsComponent {
  @Input() currentSort = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';
  @Input() directoriesFirst = true;
  @Input() colorizeByMime = false;
  @Input() showColorizeByMime = false;

  @Output() sortChange = new EventEmitter<PaneSortChange>();
  @Output() directoriesFirstChange = new EventEmitter<boolean>();
  @Output() colorizeByMimeChange = new EventEmitter<boolean>();

  menuOpen = false;

  readonly sortOptions = [
    { value: 'name', label: 'Name' },
    { value: 'size', label: 'Size' },
    { value: 'modified', label: 'Date Modified' },
    { value: 'type', label: 'Type' },
  ];

  toggleMenu(event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.menuOpen = !this.menuOpen;
  }

  selectSort(field: string): void {
    this.sortChange.emit({ field, direction: this.sortDirection });
    this.menuOpen = false;
  }

  toggleDirection(): void {
    const direction = this.sortDirection === 'asc' ? 'desc' : 'asc';
    this.sortChange.emit({ field: this.currentSort, direction });
    this.menuOpen = false;
  }

  toggleDirectoriesFirst(): void {
    this.directoriesFirstChange.emit(!this.directoriesFirst);
  }

  toggleColorizeByMime(): void {
    this.colorizeByMimeChange.emit(!this.colorizeByMime);
  }
}