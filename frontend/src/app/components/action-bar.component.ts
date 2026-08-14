import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { DropdownContainerComponent } from './dropdown-container.component';
import { DropdownTriggerDirective } from '../directives/dropdown.directive';
import type { PaneToolbarAction } from './pane-toolbar-action';

export type ViewMode = 'list' | 'grid' | 'tree' | 'details';

@Component({
  selector: 'app-action-bar',
  standalone: true,
  imports: [CommonModule, DropdownContainerComponent, DropdownTriggerDirective],
  template: `
    <div class="action-bar">
      <!-- View switcher (merged) -->
      <div class="view-switcher" role="toolbar" aria-label="View options">
        <button
          *ngFor="let option of viewOptions"
          class="view-btn"
          [class.active]="currentView === option.mode"
          [attr.aria-pressed]="currentView === option.mode"
          [title]="option.label + (option.shortcut ? ' (' + option.shortcut + ')' : '')"
          (click)="onViewChange(option.mode)">
          <ng-container [ngSwitch]="option.mode">
            <svg *ngSwitchCase="'list'" width="20" height="20" viewBox="0 0 16 16" fill="none">
              <line x1="2" y1="4"  x2="14" y2="4"  stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
              <line x1="2" y1="8"  x2="14" y2="8"  stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
              <line x1="2" y1="12" x2="14" y2="12" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
            </svg>
            <svg *ngSwitchCase="'grid'" width="20" height="20" viewBox="0 0 16 16" fill="none">
              <rect x="2"  y="2"  width="5" height="5" rx="1" stroke="currentColor" stroke-width="1"/>
              <rect x="9"  y="2"  width="5" height="5" rx="1" stroke="currentColor" stroke-width="1"/>
              <rect x="2"  y="9"  width="5" height="5" rx="1" stroke="currentColor" stroke-width="1"/>
              <rect x="9"  y="9"  width="5" height="5" rx="1" stroke="currentColor" stroke-width="1"/>
            </svg>
            <svg *ngSwitchCase="'tree'" width="20" height="20" viewBox="0 0 16 16" fill="none">
              <circle cx="3"  cy="3"  r="1.5" fill="currentColor"/>
              <circle cx="3"  cy="8"  r="1.5" fill="currentColor"/>
              <circle cx="3"  cy="13" r="1.5" fill="currentColor"/>
              <circle cx="12" cy="5"  r="1.5" fill="currentColor"/>
              <circle cx="12" cy="11" r="1.5" fill="currentColor"/>
              <line x1="3" y1="4.5" x2="3"    y2="11.5" stroke="currentColor" stroke-width="1"/>
              <line x1="3" y1="5"   x2="10.5" y2="5"    stroke="currentColor" stroke-width="1"/>
              <line x1="3" y1="11"  x2="10.5" y2="11"   stroke="currentColor" stroke-width="1"/>
            </svg>
            <svg *ngSwitchCase="'details'" width="20" height="20" viewBox="0 0 16 16" fill="none">
              <rect x="2" y="2"  width="3" height="3" rx="0.5" fill="currentColor" opacity="0.8"/>
              <rect x="2" y="7"  width="3" height="3" rx="0.5" fill="currentColor" opacity="0.8"/>
              <rect x="2" y="12" width="3" height="2" rx="0.5" fill="currentColor" opacity="0.8"/>
              <line x1="7" y1="3.5" x2="14" y2="3.5" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
              <line x1="7" y1="8.5" x2="14" y2="8.5" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
              <line x1="7" y1="13"  x2="14" y2="13"  stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
            </svg>
          </ng-container>
        </button>

        <div class="view-divider"></div>

        <app-dropdown-container [(isOpen)]="arrangeMenuOpen">
          <button
            appDropdownTrigger
            class="view-btn arrange-btn"
            [class.active]="arrangeMenuOpen"
            (click)="toggleArrangeMenu($event)"
            title="Arrange">
            <svg width="20" height="20" viewBox="0 0 16 16" fill="none">
              <line x1="2" y1="5"  x2="9" y2="5"  stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
              <line x1="2" y1="8"  x2="7" y2="8"  stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
              <line x1="2" y1="11" x2="5" y2="11" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
              <path d="M12 2 L12 13 M10 11 L12 13 L14 11" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </button>
          <div appDropdownMenu class="arrange-menu">
            <div
              class="arrange-option"
              *ngFor="let sort of arrangeSortOptions"
              [class.active]="currentSort === sort.value"
              (click)="onSortChange(sort.value)">
              <span class="arrange-option-indicator">
                <svg *ngIf="currentSort === sort.value" width="20" height="20" viewBox="0 0 16 16" fill="none">
                  <path d="M2 8L6 12L14 4" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </span>
              <span class="arrange-option-label">{{ sort.label }}</span>
            </div>
            <div class="arrange-divider"></div>
            <div class="arrange-option" (click)="toggleSortDirection()">
              <span class="arrange-option-indicator">
                <svg *ngIf="sortDirection === 'asc'" width="20" height="20" viewBox="0 0 16 16" fill="none">
                  <path d="M8 13 L8 3 M4 7 L8 3 L12 7" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
                <svg *ngIf="sortDirection === 'desc'" width="20" height="20" viewBox="0 0 16 16" fill="none">
                  <path d="M8 3 L8 13 M4 9 L8 13 L12 9" stroke="currentColor" stroke-width="1" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </span>
              <span class="arrange-option-label">{{ sortDirection === 'asc' ? 'Ascending' : 'Descending' }}</span>
            </div>

            <div class="arrange-divider"></div>
            <div class="arrange-option" [class.active]="directoriesFirst" (click)="toggleDirectoriesFirst()">
              <span class="arrange-option-indicator">
                <svg *ngIf="directoriesFirst" width="20" height="20" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                  <circle cx="8" cy="8" r="2.5" fill="currentColor" />
                </svg>
              </span>
              <span class="arrange-option-label">Directories first</span>
            </div>

            <div class="arrange-option" [class.active]="colorizeByMime" (click)="toggleColorizeByMime()">
              <span class="arrange-option-indicator">
                <svg *ngIf="colorizeByMime" width="20" height="20" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                  <circle cx="8" cy="8" r="2.5" fill="currentColor" />
                </svg>
              </span>
              <span class="arrange-option-label">Colorize by Category</span>
            </div>
          </div>
        </app-dropdown-container>

        <div class="view-divider"></div>

        <ng-container *ngFor="let action of actions; trackBy: trackAction">
          <button
            class="action-btn"
            [disabled]="!action.enabled"
            [title]="action.label + (action.shortcut ? ' (' + action.shortcut + ')' : '')"
            [attr.aria-label]="action.label"
            (click)="invokeAction(action)">
            <ng-container [ngSwitch]="action.icon">
              <svg *ngSwitchCase="'new-folder'" width="20" height="20" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path d="M1 3.5A1.5 1.5 0 0 1 2.5 2H6l1.5 1.5H13.5A1.5 1.5 0 0 1 15 5v6.5A1.5 1.5 0 0 1 13.5 13h-11A1.5 1.5 0 0 1 1 11.5v-8z" fill="currentColor" opacity="0.25"/>
                <path d="M1 5h14v6.5A1.5 1.5 0 0 1 13.5 13h-11A1.5 1.5 0 0 1 1 11.5V5z" fill="currentColor" opacity="0.4"/>
                <path d="M8 6.75a.75.75 0 0 1 .75.75v1.25h1.25a.75.75 0 0 1 0 1.5H8.75v1.25a.75.75 0 0 1-1.5 0V10.25H6a.75.75 0 0 1 0-1.5h1.25V7.5A.75.75 0 0 1 8 6.75z" fill="currentColor"/>
              </svg>
              <svg *ngSwitchCase="'delete'" width="20" height="20" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path d="M6 2h4l1 1h2v1H3V3h2l1-1zM4 5h8l-1 8a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2L4 5z" fill="currentColor"/>
              </svg>
              <svg *ngSwitchCase="'copy'" width="20" height="20" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path d="M3 2.5A1.5 1.5 0 0 1 4.5 1h5.586a1 1 0 0 1 .707.293l2.914 2.914A1 1 0 0 1 14 4.914V10.5A1.5 1.5 0 0 1 12.5 12H4.5A1.5 1.5 0 0 1 3 10.5v-8z" fill="currentColor" opacity="0.3"/>
                <path d="M1 5.5A1.5 1.5 0 0 1 2.5 4h5.586a1 1 0 0 1 .707.293l2.914 2.914A1 1 0 0 1 12 7.914V13.5A1.5 1.5 0 0 1 10.5 15h-8A1.5 1.5 0 0 1 1 13.5v-8z" fill="currentColor"/>
              </svg>
              <svg *ngSwitchCase="'move'" width="20" height="20" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path d="M1 3.5A1.5 1.5 0 0 1 2.5 2h6.586a1 1 0 0 1 .707.293l2.914 2.914A1 1 0 0 1 13 5.914V7h-1.5V6H9.5A1.5 1.5 0 0 1 8 4.5V3H2.5a.5.5 0 0 0-.5.5v9a.5.5 0 0 0 .5.5H7v1.5H2.5A1.5 1.5 0 0 1 1 13V3.5z" fill="currentColor"/>
                <path d="M10 9.75a.75.75 0 0 1 .75-.75h2.69l-.72-.72a.75.75 0 1 1 1.06-1.06l2 2a.75.75 0 0 1 0 1.06l-2 2a.75.75 0 1 1-1.06-1.06l.72-.72H10.75A.75.75 0 0 1 10 9.75z" fill="currentColor"/>
              </svg>
              <svg *ngSwitchCase="'refresh'" width="20" height="20" viewBox="0 0 16 16" fill="none" aria-hidden="true">
                <path d="M8 3a5 5 0 1 0 4.546 2.914.5.5 0 0 1 .908-.417A6 6 0 1 1 8 2v1z" fill="currentColor"/>
                <path d="M8 4.466V.534a.25.25 0 0 1 .41-.192l2.36 1.966c.12.1.12.284 0 .384L8.41 4.658A.25.25 0 0 1 8 4.466z" fill="currentColor"/>
              </svg>
              <span *ngSwitchDefault class="plugin-action-icon" aria-hidden="true">{{ action.iconText || '◇' }}</span>
            </ng-container>
          </button>
        </ng-container>
      </div>
    </div>
  `,
  styles: [`
    .action-bar {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      /* Keep action-bar no taller than the address bar */
      height: var(--vsc-titlebar-height);
      padding: 0 var(--vsc-padding-lg);
      background: var(--vsc-sidebar-background);
      border-bottom: 1px solid var(--vsc-border);
      flex-wrap: wrap;
      flex-shrink: 0;
    }

    /* Soften white visuals slightly (icons/text) */
    .action-bar svg,
    .action-bar .action-btn,
    .action-bar .view-btn,
    .action-bar .btn-small {
      filter: brightness(0.90);
    }

    .view-switcher { display: flex; align-items: center; gap: var(--vsc-padding-sm); }

    .action-pictograms { display:flex; gap: var(--vsc-padding-xs); margin-right: var(--vsc-padding-sm); }

    .action-btn { width:28px; height:28px; display:inline-flex; align-items:center; justify-content:center; border:none; border-radius:var(--vsc-border-radius); background:transparent; color:var(--vsc-tab-foreground); cursor:pointer; transition: background 120ms ease, color 120ms ease, opacity 120ms ease; }
    .action-btn:disabled { opacity:0.35; cursor:not-allowed; }
    .action-btn:hover:not(:disabled) { background: var(--vsc-tab-hover-background); color: var(--vsc-tab-hover-foreground); }
    .plugin-action-icon { font-size:13px; line-height:1; max-width:18px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }

    .view-btn { display:flex; align-items:center; justify-content:center; width:28px; height:28px; border:none; border-radius:var(--vsc-border-radius); background:transparent; color:var(--vsc-tab-foreground); cursor:pointer; }
    .action-btn svg,
    .view-btn svg {
      width: 16px;
      height: 16px;
      flex-shrink: 0;
    }
    .view-btn svg { opacity: 0.45; transition: opacity 120ms ease, filter 120ms ease; }
    .view-btn:hover { background: var(--vsc-tab-hover-background); color: var(--vsc-tab-hover-foreground); }
    .view-btn:hover svg { opacity: 0.80; }
    .view-btn.active { background: var(--vsc-tab-active-background); color: var(--vsc-accent-blue); }
    .view-btn.active svg { opacity: 1; filter: none; }

    .view-divider { width:1px; height:20px; background:var(--vsc-border); margin:0 var(--vsc-padding-sm); }

    .arrange-menu { min-width:192px; background:var(--vsc-input-background); border:1px solid var(--vsc-border); border-radius:var(--vsc-border-radius); box-shadow:var(--vsc-widget-shadow); padding:var(--vsc-padding-xs) 0; }
    .arrange-option { display:flex; align-items:center; gap:var(--vsc-padding-sm); padding:var(--vsc-padding-sm) var(--vsc-padding-md); cursor:pointer; color:var(--vsc-foreground); font-size:var(--vsc-font-size); }
    .arrange-option:hover { background: var(--vsc-list-hover-background); }
    .arrange-option.active { color: var(--vsc-accent-blue); }
    .arrange-option-indicator { width:16px; height:16px; display:flex; align-items:center; justify-content:center; flex-shrink:0; }
    .arrange-option-label { min-width:0; }
    .arrange-divider { height:1px; background:var(--vsc-border); margin: var(--vsc-padding-xs) 0; }

    .legacy-actions { display:flex; gap: var(--vsc-padding-sm); }

    .btn-small { display:inline-flex; align-items:center; justify-content:center; gap: var(--vsc-padding-xs); padding: var(--vsc-padding-xs) var(--vsc-padding-sm); border:1px solid transparent; border-radius:var(--vsc-border-radius-sm); background:var(--vsc-button-secondary); color:var(--vsc-foreground); cursor:pointer; font-size:var(--vsc-font-size-small); font-family:var(--vsc-font-family); font-weight:500; height:22px; white-space:nowrap; line-height:1; opacity:0.94; transition: background-color var(--vsc-transition-fast), color var(--vsc-transition-fast), transform 80ms ease, opacity 120ms ease; }
    .btn-small:hover:not(:disabled) { background:var(--vsc-button-secondary-hover); transform:translateY(-1px); opacity:1; }
    .btn-small:disabled { opacity:0.5; cursor:not-allowed; }

    /* Compact mode: match address-bar compact sizing */
    :host-context(.compact-mode) {
      .action-bar { height: 28px; }
      .action-btn, .view-btn { width:22px; height:22px; }
      .action-btn svg, .view-btn svg { width: 14px; height: 14px; }
      .btn-small { height:20px; padding: 0 6px; font-size: calc(var(--vsc-font-size-small) - 1px); }
    }
  `]
})
export class ActionBarComponent {
  @Input() actions: PaneToolbarAction[] = [];

  // View inputs
  @Input() currentView: ViewMode = 'list';
  @Input() currentSort: string = 'name';
  @Input() sortDirection: 'asc' | 'desc' = 'asc';
  @Input() directoriesFirst = true;
  @Input() colorizeByMime = false;

  // Outputs: view/sort
  @Output() viewChange = new EventEmitter<ViewMode>();
  @Output() sortChange = new EventEmitter<{ field: string; direction: 'asc' | 'desc' }>();
  @Output() directoriesFirstChange = new EventEmitter<boolean>();
  @Output() colorizeByMimeChange = new EventEmitter<boolean>();

  arrangeMenuOpen = false;

  viewOptions = [
    { mode: 'list' as ViewMode, label: 'List View', shortcut: 'Ctrl+1' },
    { mode: 'grid' as ViewMode, label: 'Grid View', shortcut: 'Ctrl+2' },
    { mode: 'tree' as ViewMode, label: 'Tree View', shortcut: 'Ctrl+3' },
    { mode: 'details' as ViewMode, label: 'Details View', shortcut: 'Ctrl+4' },
  ];

  arrangeSortOptions = [
    { value: 'name',     label: 'Name' },
    { value: 'size',     label: 'Size' },
    { value: 'modified', label: 'Date Modified' },
    { value: 'type',     label: 'Type' },
  ];

  // View/sort handlers
  onViewChange(mode: ViewMode): void {
    if (this.currentView !== mode) {
      this.currentView = mode;
      this.viewChange.emit(mode);
    }
  }

  toggleArrangeMenu(event: Event): void {
    event.stopPropagation();
    this.arrangeMenuOpen = !this.arrangeMenuOpen;
    if (this.arrangeMenuOpen) {
      setTimeout(() => document.addEventListener('click', this.closeArrangeMenu, { once: true }));
    }
  }

  private closeArrangeMenu = (): void => { this.arrangeMenuOpen = false; };

  onSortChange(field: string): void {
    this.sortChange.emit({ field, direction: this.sortDirection });
    this.arrangeMenuOpen = false;
  }

  toggleSortDirection(): void {
    const newDir = this.sortDirection === 'asc' ? 'desc' : 'asc';
    this.sortChange.emit({ field: this.currentSort, direction: newDir });
    this.arrangeMenuOpen = false;
  }

  toggleDirectoriesFirst(): void {
    this.directoriesFirst = !this.directoriesFirst;
    this.directoriesFirstChange.emit(this.directoriesFirst);
    // Keep menu open (checkbox-style)
  }

  toggleColorizeByMime(): void {
    this.colorizeByMime = !this.colorizeByMime;
    this.colorizeByMimeChange.emit(this.colorizeByMime);
  }


  trackAction(_index: number, action: PaneToolbarAction): string { return action.id; }

  invokeAction(action: PaneToolbarAction): void {
    if (!action.enabled) return;
    void Promise.resolve(action.execute()).catch(error => {
      console.error(`Toolbar action failed: ${action.id}`, error);
    });
  }

}
