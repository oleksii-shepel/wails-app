import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WorkspaceConfig } from '../shared/protocol-enhanced';
import { DropdownDirective, DropdownTriggerDirective, DropdownMenuDirective } from '../directives/dropdown.directive';
import { DropdownContainerComponent } from './dropdown-container.component';

/**
 * Workspace Selector Component
 * Dropdown menu for selecting and managing workspaces.
 * Uses DropdownService for robust outside-click handling.
 */
@Component({
  selector: 'app-workspace-selector',
  standalone: true,
  imports: [CommonModule, DropdownTriggerDirective, DropdownMenuDirective, DropdownContainerComponent],
  template: `
    <div appDropdownTrigger class="ws-selector" (click)="toggleMenu()">
      <span class="ws-icon">{{ activeWorkspace?.icon || '📁' }}</span>
      <span class="ws-name">{{ activeWorkspace?.name || 'Default' }}</span>
      <svg class="ws-caret" [class.ws-caret--open]="showMenu"
        width="10" height="10" viewBox="0 0 16 16" fill="currentColor">
        <path d="M4 6l4 4 4-4H4z"/>
      </svg>
    </div>
    <app-dropdown-container [(isOpen)]="showMenu">
    
    <div appDropdownMenu class="ws-menu" *ngIf="showMenu">
      <div class="ws-menu-header">
        <span>Workspaces</span>
        <button (click)="onCreateNew()" class="ws-btn-icon" title="New workspace">
          <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
            <path d="M8 2a.75.75 0 0 1 .75.75v4.5h4.5a.75.75 0 0 1 0 1.5h-4.5v4.5a.75.75 0 0 1-1.5 0v-4.5h-4.5a.75.75 0 0 1 0-1.5h4.5v-4.5A.75.75 0 0 1 8 2z"/>
          </svg>
        </button>
      </div>

      <div class="ws-list">
        <div
          *ngFor="let workspace of workspaces"
          class="ws-item"
          [class.ws-item--active]="workspace.id === activeWorkspace?.id"
          (click)="onSelect(workspace)">
          <span class="ws-item-icon">{{ workspace.icon }}</span>
          <span class="ws-item-name">{{ workspace.name }}</span>
          <svg *ngIf="workspace.id === activeWorkspace?.id"
            class="ws-item-check" width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
            <path d="M13.485 1.929L5.5 9.914 2.515 6.929a1 1 0 0 0-1.414 1.414l3.692 3.693a1 1 0 0 0 1.414 0l8.692-8.693a1 1 0 0 0-1.414-1.414z"/>
          </svg>
        </div>
      </div>

      <div class="ws-menu-footer">
        <button (click)="onRename()" class="ws-footer-btn">Rename</button>
        <button (click)="onDelete()" class="ws-footer-btn ws-footer-btn--danger">Delete</button>
      </div>
    </div>
    </app-dropdown-container>
  `,
  styles: [`
    :host {
      position: relative;
      /* inline-block so it sizes to content inside flex containers */
      display: flex;
      vertical-align: middle;
    }

    .ws-selector {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-xs) var(--vsc-padding-md);
      background: var(--vsc-input-background);
      /* ensure selector doesn't collapse to zero width inside flex/grid parents */
      min-width: 120px;
      flex: 0 0 auto;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm);
      cursor: pointer;
      transition: all var(--vsc-transition-fast);
      min-height: 26px;
      user-select: none;

      &:hover {
        background: var(--vsc-list-hover-background);
        border-color: var(--vsc-border-bright);
      }
    }

    .ws-icon { font-size: 14px; line-height: 1; }

    .ws-name {
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground);
      font-weight: 500;
    }

    .ws-caret {
      opacity: 0.6;
      transition: transform var(--vsc-transition-fast);
      margin-left: auto;
    }
    .ws-caret--open { transform: rotate(180deg); opacity: 1; }

    .ws-menu {
      position: absolute;
      top: calc(100% + 4px);
      left: 0;
      min-width: 250px;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border-bright);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      z-index: var(--vsc-z-dropdown);
      overflow: hidden;
      animation: ws-in 0.12s ease-out;
    }

    @keyframes ws-in {
      from { opacity: 0; transform: translateY(-4px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    .ws-menu-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      border-bottom: 1px solid var(--vsc-border);
      background: var(--vsc-sidebar-background);
      font-size: var(--vsc-font-size-small);
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      color: var(--vsc-foreground-dim);
    }

    .ws-btn-icon {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 22px;
      height: 22px;
      padding: 0;
      background: transparent;
      border: none;
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-dim);
      cursor: pointer;
      transition: all var(--vsc-transition-fast);

      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground);
      }
    }

    .ws-list {
      max-height: 280px;
      overflow-y: auto;
      padding: var(--vsc-padding-xs) 0;

      &::-webkit-scrollbar { width: 8px; }
      &::-webkit-scrollbar-track { background: var(--vsc-scrollbar-track); }
      &::-webkit-scrollbar-thumb {
        background: var(--vsc-scrollbar-thumb);
        border-radius: 4px;
        &:hover { background: var(--vsc-scrollbar-thumb-hover); }
      }
    }

    .ws-item {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-md);
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      cursor: pointer;
      transition: background var(--vsc-transition-fast);
      border-left: 2px solid transparent;
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground);

      &:hover {
        background: var(--vsc-list-hover-background);
        border-left-color: var(--vsc-border-bright);
      }

      &.ws-item--active {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
        border-left-color: var(--vsc-accent-blue);
        font-weight: 500;
      }
    }

    .ws-item-icon { font-size: 15px; line-height: 1; flex-shrink: 0; }
    .ws-item-name { flex: 1; }
    .ws-item-check { color: var(--vsc-accent-blue); flex-shrink: 0; }

    .ws-menu-footer {
      display: flex;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-md);
      border-top: 1px solid var(--vsc-border);
      background: var(--vsc-sidebar-background);
    }

    .ws-footer-btn {
      flex: 1;
      padding: var(--vsc-padding-xs) var(--vsc-padding-sm);
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius-sm);
      background: var(--vsc-button-secondary);
      color: var(--vsc-foreground);
      cursor: pointer;
      font-size: var(--vsc-font-size-small);
      font-family: var(--vsc-font-family);
      font-weight: 500;
      height: 22px;
      transition: all var(--vsc-transition-fast);

      &:hover { background: var(--vsc-button-secondary-hover); }

      &.ws-footer-btn--danger {
        background: var(--vsc-error);
        color: var(--vsc-foreground-bright);
        &:hover { background: #d32f2f; }
      }
    }
  `]
})
export class WorkspaceSelectorComponent {
  @Input() workspaces: WorkspaceConfig[] = [];
  @Input() activeWorkspace: WorkspaceConfig | null = null;
  @Input() showMenu = false;

  @Output() workspaceChange = new EventEmitter<WorkspaceConfig>();
  @Output() createNew = new EventEmitter<void>();
  @Output() rename = new EventEmitter<void>();
  @Output() deleteWorkspace = new EventEmitter<void>();
  @Output() menuToggle = new EventEmitter<boolean>();

  // isOpen is two-way bound to appDropdown — directive handles all close/outside-click logic
  toggleMenu(): void {
    this.showMenu = !this.showMenu;
    this.menuToggle.emit(this.showMenu);
  }

  onSelect(workspace: WorkspaceConfig): void {
    this.workspaceChange.emit(workspace);
    this.showMenu = false;
    this.menuToggle.emit(false);
  }

  onCreateNew(): void { this.createNew.emit(); }
  onRename(): void { this.rename.emit(); }
  onDelete(): void { this.deleteWorkspace.emit(); }
}
