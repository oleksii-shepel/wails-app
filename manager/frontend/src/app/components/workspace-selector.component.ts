import { Component, Input, Output, EventEmitter, ViewChild, ElementRef, AfterViewInit, ChangeDetectorRef, OnChanges, OnDestroy, OnInit, SimpleChanges } from '@angular/core';
import { CommonModule } from '@angular/common';
import { WorkspaceConfig } from '../shared/protocol-enhanced';
import { DropdownDirective, DropdownTriggerDirective, DropdownMenuDirective } from '../directives/dropdown.directive';
import { DropdownContainerComponent } from './dropdown-container.component';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';

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
    <div #wsSelector appDropdownTrigger class="ws-selector" (click)="toggleMenu()">
      <span class="ws-icon">{{ activeWorkspace?.icon || '📁' }}</span>
      <span class="ws-name">{{ activeWorkspace?.name || 'Default' }}</span>
      <svg class="ws-caret" [class.ws-caret--open]="showMenu"
        width="10" height="10" viewBox="0 0 16 16" fill="currentColor">
        <path d="M4 6l4 4 4-4H4z"/>
      </svg>
    </div>
    <app-dropdown-container [(isOpen)]="showMenu" (isOpenChange)="onMenuStateChange($event)" [anchorEl]="anchorElement">
    
    <div appDropdownMenu class="ws-menu" role="menu" tabindex="-1" *ngIf="showMenu">
      <div class="ws-menu-header">
        <span>Workspaces</span>
        <button (click)="onCreateNew()" class="ws-btn-icon" title="New workspace">
          <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
            <path d="M8 2a.75.75 0 0 1 .75.75v4.5h4.5a.75.75 0 0 1 0 1.5h-4.5v4.5a.75.75 0 0 1-1.5 0v-4.5h-4.5a.75.75 0 0 1 0-1.5h4.5v-4.5A.75.75 0 0 1 8 2z"/>
          </svg>
        </button>
      </div>

      <div class="ws-list app-scroll-surface">
        <div
          *ngFor="let workspace of workspaces; let i = index"
          class="ws-item"
          role="menuitemradio"
          [attr.tabindex]="i === selectedIndex ? 0 : -1"
          [attr.aria-checked]="workspace.id === activeWorkspace?.id"
          [class.ws-item--active]="workspace.id === activeWorkspace?.id"
          [class.ws-item--selected]="i === selectedIndex"
          (focus)="setSelectedIndex(i)"
          (mouseenter)="setSelectedIndex(i)"
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
      min-width: 250px;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border-bright);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
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
      touch-action: pan-y;
      padding: var(--vsc-padding-xs) 0;
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

      &.ws-item--selected {
        background: color-mix(in srgb, var(--vsc-list-hover-background) 80%, transparent);
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
      color: var(--vsc-button-secondary-foreground);
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
        &:hover { background: var(--vsc-button-danger-background); }
      }
    }
  `]
})
export class WorkspaceSelectorComponent implements AfterViewInit, OnChanges, OnDestroy, OnInit {
  @Input() workspaces: WorkspaceConfig[] = [];
  @Input() activeWorkspace: WorkspaceConfig | null = null;
  @Input() showMenu = false;

  @ViewChild('wsSelector') wsSelectorRef?: ElementRef<HTMLElement>;

  @Output() workspaceChange = new EventEmitter<WorkspaceConfig>();
  @Output() createNew = new EventEmitter<void>();
  @Output() rename = new EventEmitter<void>();
  @Output() deleteWorkspace = new EventEmitter<void>();
  @Output() menuToggle = new EventEmitter<boolean>();

  anchorElement: HTMLElement | null = null;
  selectedIndex = -1;

  private readonly commandOwner = `overlay.workspaceSelector:${Math.random().toString(36).slice(2, 8)}`;
  private releaseOverlayContext: (() => void) | null = null;

  constructor(
    private cdr: ChangeDetectorRef,
    private inputContexts: InputContextService,
    private inputEngine: InputEngineService,
  ) {}

  ngOnInit(): void {
    this.registerInputCommands();
    this.syncSelection();
    this.syncContextLease();
  }

  ngAfterViewInit(): void {
    this.anchorElement = this.wsSelectorRef?.nativeElement ?? null;
    this.cdr.detectChanges();
  }

  ngOnChanges(_changes: SimpleChanges): void {
    this.syncSelection();
    this.syncContextLease();
  }

  ngOnDestroy(): void {
    this.releaseOverlayContext?.();
    this.releaseOverlayContext = null;
    this.unregisterInputCommands();
  }

  // isOpen is two-way bound to appDropdown — directive handles all close/outside-click logic
  toggleMenu(): void {
    this.showMenu = !this.showMenu;
    this.syncSelection();
    this.syncContextLease();
    this.menuToggle.emit(this.showMenu);
    if (this.showMenu) {
      this.focusSelectedItem();
    }
  }

  onSelect(workspace: WorkspaceConfig): void {
    this.workspaceChange.emit(workspace);
    this.showMenu = false;
    this.syncContextLease();
    this.menuToggle.emit(false);
  }

  onMenuStateChange(show: boolean): void {
    this.showMenu = show;
    this.syncSelection();
    this.syncContextLease();
    this.menuToggle.emit(show);
    if (show) {
      this.focusSelectedItem();
    }
  }

  setSelectedIndex(index: number): void {
    if (index < 0 || index >= this.workspaces.length) return;
    this.selectedIndex = index;
    this.cdr.markForCheck();
    this.focusSelectedItem();
  }

  private moveSelection(delta: number): void {
    if (this.workspaces.length === 0) return;
    const baseIndex = this.selectedIndex >= 0 ? this.selectedIndex : 0;
    this.selectedIndex = Math.max(0, Math.min(this.workspaces.length - 1, baseIndex + delta));
    this.cdr.markForCheck();
    this.focusSelectedItem();
  }

  private activateSelected(): void {
    const workspace = this.workspaces[this.selectedIndex];
    if (!workspace) return;
    this.onSelect(workspace);
  }

  private syncSelection(): void {
    if (this.workspaces.length === 0) {
      this.selectedIndex = -1;
      return;
    }

    const activeIndex = this.workspaces.findIndex(workspace => workspace.id === this.activeWorkspace?.id);
    this.selectedIndex = activeIndex >= 0 ? activeIndex : 0;
    this.cdr.markForCheck();
  }

  private syncContextLease(): void {
    if (this.showMenu) {
      this.releaseOverlayContext ??= this.inputContexts.acquire('overlay.workspaceSelector');
      return;
    }

    this.releaseOverlayContext?.();
    this.releaseOverlayContext = null;
  }

  private registerInputCommands(): void {
    this.inputEngine.registerCommand('overlay.workspaceSelector.close', () => {
      if (!this.showMenu) return;
      this.onMenuStateChange(false);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.workspaceSelector.prevItem', () => {
      if (!this.showMenu) return;
      this.moveSelection(-1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.workspaceSelector.nextItem', () => {
      if (!this.showMenu) return;
      this.moveSelection(1);
    }, this.commandOwner);

    this.inputEngine.registerCommand('overlay.workspaceSelector.activateSelected', () => {
      if (!this.showMenu) return;
      this.activateSelected();
    }, this.commandOwner);
  }

  private unregisterInputCommands(): void {
    this.inputEngine.unregisterCommand('overlay.workspaceSelector.close', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.workspaceSelector.prevItem', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.workspaceSelector.nextItem', this.commandOwner);
    this.inputEngine.unregisterCommand('overlay.workspaceSelector.activateSelected', this.commandOwner);
  }

  onCreateNew(): void { this.createNew.emit(); }
  onRename(): void { this.rename.emit(); }
  onDelete(): void { this.deleteWorkspace.emit(); }

  private focusSelectedItem(): void {
    requestAnimationFrame(() => {
      const item = document.querySelector<HTMLElement>('.ws-item.ws-item--selected');
      item?.focus({ preventScroll: true });
    });
  }
}