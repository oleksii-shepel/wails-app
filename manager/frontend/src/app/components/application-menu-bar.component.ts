import {
  ChangeDetectorRef,
  Component,
  ElementRef,
  HostListener,
  OnDestroy,
  OnInit,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { FILE_MANAGER_COMMANDS } from './command-registry';
import { WindowStateService } from '../services/window-state.service';
import {
  Quit,
  WindowIsMaximised,
  WindowMinimise,
  WindowToggleMaximise,
} from 'wailsjs/runtime';

type MenuEntry =
  | {
      readonly kind: 'command';
      readonly commandId: string;
      readonly label: string;
    }
  | {
      readonly kind: 'separator';
    };

interface ApplicationMenu {
  readonly id: string;
  readonly label: string;
  readonly entries: readonly MenuEntry[];
}

const command = (commandId: string, label: string): MenuEntry => ({
  kind: 'command',
  commandId,
  label,
});

const separator = (): MenuEntry => ({ kind: 'separator' });

const MENUS: readonly ApplicationMenu[] = [
  {
    id: 'file',
    label: 'File',
    entries: [
      command(FILE_MANAGER_COMMANDS.newTab, 'New Tab'),
      command('reassembly.manager.reopenClosedTab', 'Reopen Closed Tab'),
      command(FILE_MANAGER_COMMANDS.showClosedTabs, 'Recently Closed Tabs…'),
      separator(),
      command(FILE_MANAGER_COMMANDS.newFolder, 'New Folder'),
      command('reassembly.manager.newFile', 'New File'),
      command('reassembly.manager.newTextDocument', 'New Text Document'),
      command(FILE_MANAGER_COMMANDS.createArchive, 'Archive Files…'),
      separator(),
      command('reassembly.manager.properties', 'Properties'),
      separator(),
      command(FILE_MANAGER_COMMANDS.openSettings, 'Settings'),
    ],
  },
  {
    id: 'edit',
    label: 'Edit',
    entries: [
      command('reassembly.edit.undo', 'Undo'),
      command('reassembly.edit.redo', 'Redo'),
      command(FILE_MANAGER_COMMANDS.operationHistory, 'Operation History…'),
      separator(),
      command(FILE_MANAGER_COMMANDS.cut, 'Cut'),
      command(FILE_MANAGER_COMMANDS.copy, 'Copy'),
      command(FILE_MANAGER_COMMANDS.paste, 'Paste'),
      separator(),
      command(FILE_MANAGER_COMMANDS.rename, 'Rename'),
      command(FILE_MANAGER_COMMANDS.deleteSelected, 'Delete'),
    ],
  },
  {
    id: 'selection',
    label: 'Selection',
    entries: [
      command(FILE_MANAGER_COMMANDS.selectAll, 'Select All'),
      command(FILE_MANAGER_COMMANDS.deselectAll, 'Clear Selection'),
      command(FILE_MANAGER_COMMANDS.invertSelection, 'Invert Selection'),
      separator(),
      command(FILE_MANAGER_COMMANDS.selectSameExt, 'Select Same Extension'),
      command('reassembly.selection.sameType', 'Select Same Type'),
      separator(),
      command('reassembly.selection.files', 'Select Files'),
      command('reassembly.selection.folders', 'Select Folders'),
      command('reassembly.selection.unselectFiles', 'Unselect Files'),
      command('reassembly.selection.unselectFolders', 'Unselect Folders'),
    ],
  },
  {
    id: 'view',
    label: 'View',
    entries: [
      command(FILE_MANAGER_COMMANDS.openCommandPalette, 'Command Palette…'),
      command(FILE_MANAGER_COMMANDS.openGlobalSearch, 'Global Search…'),
      separator(),
      command(FILE_MANAGER_COMMANDS.refresh, 'Refresh'),
      command(FILE_MANAGER_COMMANDS.toggleHidden, 'Toggle Hidden Files'),
      separator(),
      command(FILE_MANAGER_COMMANDS.openOperations, 'Operations Center…'),
      command(FILE_MANAGER_COMMANDS.openSelectionBasket, 'Selection Basket…'),
      command('reassembly.workspace.details', 'Details Workspace'),
    ],
  },
  {
    id: 'go',
    label: 'Go',
    entries: [
      command('reassembly.manager.back', 'Back'),
      command('reassembly.manager.forward', 'Forward'),
      command(FILE_MANAGER_COMMANDS.navigateUp, 'Parent'),
      command('reassembly.manager.location', 'Go to Location…'),
      command(FILE_MANAGER_COMMANDS.showPathHistory, 'Location History…'),
      command(FILE_MANAGER_COMMANDS.openQuickAccess, 'Quick Access…'),
      command(FILE_MANAGER_COMMANDS.showPinnedLocations, 'Pinned Locations…'),
      separator(),
      command(FILE_MANAGER_COMMANDS.openRemoteConnections, 'Remote Connections…'),
      command(FILE_MANAGER_COMMANDS.remoteConnect, 'Connect Remote…'),
      separator(),
      command(FILE_MANAGER_COMMANDS.prevTab, 'Previous Tab'),
      command(FILE_MANAGER_COMMANDS.nextTab, 'Next Tab'),
      command(FILE_MANAGER_COMMANDS.switchPane, 'Switch Pane'),
      separator(),
      command('reassembly.workspace.previous', 'Previous Workspace'),
      command('reassembly.workspace.next', 'Next Workspace'),
      command(FILE_MANAGER_COMMANDS.showWorkspaceSwitcher, 'Workspace Switcher…'),
    ],
  },
  {
    id: 'tools',
    label: 'Tools',
    entries: [
      command(FILE_MANAGER_COMMANDS.openOperations, 'Operations Center…'),
      command(FILE_MANAGER_COMMANDS.openSelectionBasket, 'Selection Basket…'),
      command(FILE_MANAGER_COMMANDS.operationHistory, 'Operation History…'),
      separator(),
      command(FILE_MANAGER_COMMANDS.diskUsage, 'Disk Usage…'),
      command(FILE_MANAGER_COMMANDS.findDuplicates, 'Find Duplicates…'),
      command(FILE_MANAGER_COMMANDS.batchRename, 'Batch Rename…'),
      separator(),
      command(FILE_MANAGER_COMMANDS.comparePanes, 'Compare Panes'),
      command(FILE_MANAGER_COMMANDS.recursiveComparePanes, 'Recursive Compare'),
      command(FILE_MANAGER_COMMANDS.reviewSyncPlan, 'Review Sync Plan'),
      separator(),
      command('reassembly.navigation.openTerminal', 'Open Terminal Here'),
      separator(),
      command(FILE_MANAGER_COMMANDS.openRemoteConnections, 'Remote Connections…'),
      command(FILE_MANAGER_COMMANDS.remoteConnect, 'Connect Remote…'),
      command(FILE_MANAGER_COMMANDS.openTrash, 'Trash / Recycle Bin…'),
    ],
  },
  {
    id: 'help',
    label: 'Help',
    entries: [
      command(FILE_MANAGER_COMMANDS.showKeyboardHelp, 'Keyboard Shortcuts…'),
      command(FILE_MANAGER_COMMANDS.openInputCenter, 'Input & Shortcuts…'),
      separator(),
      command(FILE_MANAGER_COMMANDS.openPluginSettings, 'Plugin Settings…'),
      command(FILE_MANAGER_COMMANDS.openSettings, 'Settings…'),
      separator(),
      command(FILE_MANAGER_COMMANDS.openCommandPalette, 'Command Palette…'),
    ],
  },
] as const;

@Component({
  selector: 'app-application-menu-bar',
  standalone: true,
  imports: [CommonModule],
  template: `
    <header
      class="application-menu-bar"
      role="menubar"
      aria-label="Application menu">
      <nav class="menus" aria-label="Application commands">
        <div
          *ngFor="let menu of menus; let index = index"
          class="menu-root"
          [class.open]="openMenuIndex === index"
          (mouseenter)="onMenuHover(index)">
          <button
            type="button"
            class="menu-trigger"
            role="menuitem"
            [attr.aria-expanded]="openMenuIndex === index"
            [attr.aria-controls]="'application-menu-' + menu.id"
            (click)="toggleMenu(index, $event)"
            (keydown.arrowdown)="openMenu(index, $event)"
            (keydown.enter)="openMenu(index, $event)"
            (keydown.space)="openMenu(index, $event)">
            {{ menu.label }}
          </button>

          <div
            *ngIf="openMenuIndex === index"
            class="menu-popup"
            role="menu"
            [id]="'application-menu-' + menu.id">
            <ng-container *ngFor="let entry of menu.entries">
              <div
                *ngIf="entry.kind === 'separator'"
                class="menu-separator"
                role="separator">
              </div>

              <button
                *ngIf="entry.kind === 'command'"
                type="button"
                class="menu-item"
                role="menuitem"
                [disabled]="!isEnabled(entry.commandId)"
                (click)="execute(entry.commandId, $event)">
                <span class="menu-item-label">{{ entry.label }}</span>
                <span class="menu-item-shortcut">{{ shortcut(entry.commandId) }}</span>
              </button>
            </ng-container>
          </div>
        </div>
      </nav>

      <div class="window-title" aria-hidden="true">Aether File Manager</div>
      <div class="drag-space" aria-hidden="true"></div>

      <div class="window-controls" aria-label="Window controls">
        <button
          type="button"
          class="window-control"
          aria-label="Minimize"
          title="Minimize"
          (click)="minimizeWindow($event)">
          <svg viewBox="0 0 12 12" aria-hidden="true">
            <path d="M2 8.5h8" />
          </svg>
        </button>

        <button
          type="button"
          class="window-control"
          [attr.aria-label]="maximized ? 'Restore' : 'Maximize'"
          [title]="maximized ? 'Restore' : 'Maximize'"
          (click)="toggleMaximize($event)">
          <svg *ngIf="!maximized" viewBox="0 0 12 12" aria-hidden="true">
            <rect x="2.25" y="2.25" width="7.5" height="7.5" />
          </svg>
          <svg *ngIf="maximized" viewBox="0 0 12 12" aria-hidden="true">
            <path d="M4 2.25h5.75V8H8.5V3.5H4z" />
            <rect x="2.25" y="4" width="5.75" height="5.75" />
          </svg>
        </button>

        <button
          type="button"
          class="window-control window-control-close"
          aria-label="Close"
          title="Close"
          (click)="closeWindow($event)">
          <svg viewBox="0 0 12 12" aria-hidden="true">
            <path d="M2.5 2.5l7 7M9.5 2.5l-7 7" />
          </svg>
        </button>
      </div>
    </header>
  `,
  styles: [`
    :host {
      display: block;
      flex: 0 0 auto;
      width: 100%;
      min-width: 0;
      z-index: 9000;
      --wails-draggable: drag;
    }

    .application-menu-bar {
      position: relative;
      display: flex;
      align-items: center;
      width: 100%;
      height: 35px;
      min-width: 0;
      padding: 0 7px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-titlebar-background, var(--vsc-editor-background));
      color: var(--vsc-titlebar-foreground, var(--vsc-foreground));
      font: 12px/1 var(--vsc-font-family, system-ui, sans-serif);
      user-select: none;
      box-sizing: border-box;
      --wails-draggable: drag;
    }

    .menus {
      position: relative;
      z-index: 2;
      display: flex;
      align-items: center;
      height: 100%;
      gap: 1px;
      flex: 0 0 auto;
      --wails-draggable: no-drag;
    }

    .menus,
    .menus *,
    .menu-popup,
    .menu-popup * {
      --wails-draggable: no-drag;
    }

    .menu-root {
      position: relative;
      height: 100%;
      display: flex;
      align-items: center;
    }

    .menu-trigger {
      height: 26px;
      padding: 0 8px;
      border: 0;
      border-radius: 4px;
      background: transparent;
      color: inherit;
      font: inherit;
      cursor: default;
    }

    .menu-trigger:hover,
    .menu-root.open > .menu-trigger {
      background: var(--vsc-list-hover-background, rgba(127, 127, 127, .14));
    }

    .menu-root.open > .menu-trigger {
      color: var(--vsc-list-active-foreground, var(--vsc-foreground));
    }

    .menu-popup {
      position: absolute;
      top: 31px;
      left: 0;
      min-width: 245px;
      max-width: min(360px, calc(100vw - 16px));
      padding: 4px;
      border: 1px solid var(--vsc-border, #555);
      border-radius: 5px;
      background: var(--vsc-menu-background, var(--vsc-popover-background, var(--vsc-editor-background)));
      color: var(--vsc-menu-foreground, var(--vsc-foreground));
      box-shadow: 0 8px 24px var(--vsc-scrim-soft, rgba(0, 0, 0, .28));
      z-index: 10000;
      transform-origin: top left;
      animation: menu-surface-enter 0.18s var(--dialog-enter-ease, cubic-bezier(0.16, 1, 0.3, 1));
    }

    .menu-item {
      display: grid;
      grid-template-columns: minmax(0, 1fr) auto;
      align-items: center;
      column-gap: 26px;
      width: 100%;
      min-height: 28px;
      padding: 4px 9px;
      border: 0;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      text-align: left;
      font: inherit;
      cursor: default;
    }

    .menu-item:hover:not(:disabled),
    .menu-item:focus-visible:not(:disabled) {
      outline: 0;
      background: var(--vsc-list-active-background, var(--vsc-list-hover-background));
      color: var(--vsc-list-active-foreground, var(--vsc-foreground));
    }

    .menu-item:disabled {
      opacity: .42;
    }

    .menu-item-label {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .menu-item-shortcut {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      white-space: nowrap;
      font-size: 11px;
    }

    .menu-item:hover:not(:disabled) .menu-item-shortcut {
      color: inherit;
    }

    .menu-separator {
      height: 1px;
      margin: 4px 6px;
      background: var(--vsc-border-subtle, var(--vsc-border));
    }

    .drag-space {
      flex: 1 1 auto;
      align-self: stretch;
      min-width: 30px;
      --wails-draggable: drag;
    }


    .window-controls {
      position: relative;
      z-index: 3;
      display: flex;
      align-self: stretch;
      align-items: stretch;
      margin: 0 -7px 0 8px;
      --wails-draggable: no-drag;
    }

    .window-controls,
    .window-controls * {
      --wails-draggable: no-drag;
    }

    .window-control {
      display: inline-grid;
      place-items: center;
      width: 46px;
      min-width: 46px;
      height: 35px;
      padding: 0;
      border: 0;
      border-radius: 0;
      background: transparent;
      color: inherit;
      cursor: default;
    }

    .window-control:hover {
      background: var(--vsc-list-hover-background, rgba(127, 127, 127, .16));
    }

    .window-control-close:hover {
      background: #c42b1c;
      color: #fff;
    }

    .window-control svg {
      width: 12px;
      height: 12px;
      fill: none;
      stroke: currentColor;
      stroke-width: 1;
      vector-effect: non-scaling-stroke;
      pointer-events: none;
    }

    .window-title {
      position: absolute;
      left: 50%;
      top: 50%;
      max-width: 36vw;
      transform: translate(-50%, -50%);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      pointer-events: none;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
      --wails-draggable: drag;
    }
  `],
})
export class ApplicationMenuBarComponent implements OnInit, OnDestroy {
  readonly menus = MENUS;
  openMenuIndex: number | null = null;
  maximized = false;

  private readonly resizeHandler = () => {
    void this.refreshMaximizedState();
  };

  constructor(
    private readonly input: InputEngineService,
    private readonly element: ElementRef<HTMLElement>,
    private readonly windowState: WindowStateService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    window.addEventListener('resize', this.resizeHandler, { passive: true });
    void this.refreshMaximizedState();
  }

  ngOnDestroy(): void {
    window.removeEventListener('resize', this.resizeHandler);
  }

  toggleMenu(index: number, event: Event): void {
    event.preventDefault();
    event.stopPropagation();
    this.openMenuIndex = this.openMenuIndex === index ? null : index;
  }

  openMenu(index: number, event?: Event): void {
    event?.preventDefault();
    event?.stopPropagation();
    this.openMenuIndex = index;
    queueMicrotask(() => this.focusFirstEnabledItem());
  }

  onMenuHover(index: number): void {
    if (this.openMenuIndex !== null) {
      this.openMenuIndex = index;
    }
  }

  execute(commandId: string, event: Event): void {
    event.preventDefault();
    event.stopPropagation();
    if (!this.isEnabled(commandId)) return;

    this.openMenuIndex = null;
    this.cdr.detectChanges();
    queueMicrotask(() => {
      this.input.dispatchCommandById(commandId, { source: 'mouse' });
    });
  }

  isEnabled(commandId: string): boolean {
    return this.input.getCommandState(commandId)?.enabled === true;
  }

  shortcut(commandId: string): string {
    const binding = this.input.getPreferredKeyboardBinding(commandId);
    return binding ? this.formatBinding(binding) : '';
  }

  minimizeWindow(event: MouseEvent): void {
    event.preventDefault();
    event.stopPropagation();
    this.openMenuIndex = null;
    WindowMinimise();
  }

  async toggleMaximize(event: MouseEvent): Promise<void> {
    event.preventDefault();
    event.stopPropagation();
    this.openMenuIndex = null;
    WindowToggleMaximise();
    await this.refreshMaximizedState();
  }

  async closeWindow(event: MouseEvent): Promise<void> {
    event.preventDefault();
    event.stopPropagation();
    this.openMenuIndex = null;

    // Preserve the same geometry persistence path used by normal app shutdown.
    await this.windowState.captureNow().catch(() => undefined);
    Quit();
  }

  private async refreshMaximizedState(): Promise<void> {
    try {
      this.maximized = await WindowIsMaximised();
    } catch {
      this.maximized = false;
    }
  }

  @HostListener('document:mousedown', ['$event'])
  onDocumentMouseDown(event: MouseEvent): void {
    if (this.openMenuIndex === null) return;
    if (this.element.nativeElement.contains(event.target as Node)) return;
    this.openMenuIndex = null;
  }

  @HostListener('document:keydown', ['$event'])
  onDocumentKeyDown(event: KeyboardEvent): void {
    if (event.key === 'Escape' && this.openMenuIndex !== null) {
      event.preventDefault();
      event.stopPropagation();
      this.openMenuIndex = null;
      return;
    }

    if (this.openMenuIndex === null) return;

    if (event.key === 'ArrowRight') {
      event.preventDefault();
      this.openMenuIndex = (this.openMenuIndex + 1) % this.menus.length;
      queueMicrotask(() => this.focusFirstEnabledItem());
      return;
    }

    if (event.key === 'ArrowLeft') {
      event.preventDefault();
      this.openMenuIndex =
        (this.openMenuIndex - 1 + this.menus.length) % this.menus.length;
      queueMicrotask(() => this.focusFirstEnabledItem());
    }
  }

  private focusFirstEnabledItem(): void {
    const popup = this.element.nativeElement.querySelector<HTMLElement>('.menu-popup');
    popup?.querySelector<HTMLButtonElement>('.menu-item:not(:disabled)')?.focus();
  }

  private formatBinding(binding: InputBindingDefinition): string {
    if (binding.trigger.device !== 'keyboard') return '';
    return binding.trigger.chord.steps
      .map(step => {
        const parts = [
          ...(step.modifiers ?? []).map(modifier => this.modifierLabel(modifier)),
          this.keyLabel(step.key),
        ];
        return parts.join('+');
      })
      .join(' ');
  }

  private modifierLabel(value: string): string {
    switch (value.toLowerCase()) {
      case 'ctrl': return 'Ctrl';
      case 'shift': return 'Shift';
      case 'alt': return 'Alt';
      case 'meta': return 'Meta';
      default: return value;
    }
  }

  private keyLabel(value: string): string {
    const key = value.toLowerCase();
    if (key === 'arrowleft') return 'Left';
    if (key === 'arrowright') return 'Right';
    if (key === 'arrowup') return 'Up';
    if (key === 'arrowdown') return 'Down';
    if (key === 'escape') return 'Esc';
    if (key === 'delete') return 'Del';
    if (key === 'backspace') return 'Backspace';
    if (key.length === 1) return key.toUpperCase();
    return key.replace(/^f(\d+)$/, 'F$1');
  }
}
