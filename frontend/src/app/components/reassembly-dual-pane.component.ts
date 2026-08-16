import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  OnDestroy,
  OnInit,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { ApiService } from '../services/api.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { WorkspaceService } from '../services/workspace.service';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { canTransferBetween, transferBlockedMessage } from '../services/transfer-capabilities';
import { ReassemblyDirectoryPaneComponent } from './reassembly-directory-pane.component';
import { ReassemblyWorkspaceSwitcherComponent } from './reassembly-workspace-switcher.component';
import { ReassemblyPathHistoryComponent } from './reassembly-path-history.component';
import { ReassemblyQuickAccessComponent } from './reassembly-quick-access.component';
import {
  ReassemblyContextMenuComponent,
  type ReassemblyContextMenuItem,
  type ReassemblyContextMenuState,
} from './reassembly-context-menu.component';
import type { FileInfo } from '../shared/protocol';

type PaneId = 'left' | 'right';

const CONTEXT_ID = 'reassembly.manager';
const OWNER_ID = 'reassembly.manager';

const COMMAND = {
  switchPane: 'reassembly.manager.switchPane',
  cursorUp: 'reassembly.manager.cursorUp',
  cursorDown: 'reassembly.manager.cursorDown',
  open: 'reassembly.manager.open',
  up: 'reassembly.manager.up',
  back: 'reassembly.manager.back',
  forward: 'reassembly.manager.forward',
  location: 'reassembly.manager.location',
  selectAll: 'reassembly.manager.selectAll',
  clearSelection: 'reassembly.manager.clearSelection',
  refresh: 'reassembly.manager.refresh',
  rename: 'reassembly.manager.rename',
  newFolder: 'reassembly.manager.newFolder',
  deleteSelected: 'reassembly.manager.delete',
  copy: 'reassembly.manager.copy',
  cut: 'reassembly.manager.cut',
  paste: 'reassembly.manager.paste',
  copyToOther: 'reassembly.manager.copyToOther',
  moveToOther: 'reassembly.manager.moveToOther',
  newTab: 'reassembly.manager.newTab',
  closeTab: 'reassembly.manager.closeTab',
  nextTab: 'reassembly.manager.nextTab',
  viewerPrevious: 'reassembly.viewer.previous',
  viewerNext: 'reassembly.viewer.next',
  viewerZoomIn: 'reassembly.viewer.zoomIn',
  viewerZoomOut: 'reassembly.viewer.zoomOut',
  viewerFit: 'reassembly.viewer.fit',
  viewerActualSize: 'reassembly.viewer.actualSize',
  closeContextMenu: 'reassembly.manager.closeContextMenu',
  focusFilter: 'reassembly.manager.focusFilter',
  toggleHidden: 'reassembly.workspace.toggleHidden',
  showWorkspaceSwitcher: 'reassembly.workspace.showSwitcher',
  newWorkspace: 'reassembly.workspace.new',
  showPathHistory: 'reassembly.navigation.pathHistory',
  showQuickAccess: 'reassembly.navigation.quickAccess',
} as const;

const BINDINGS: readonly InputBindingDefinition[] = [
  { id:'reassembly.manager.switchPane.tab', commandId:COMMAND.switchPane, contextId:CONTEXT_ID, source:'default', description:'Switch active pane', category:'Pane Control', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'tab'}]}}},
  { id:'reassembly.manager.cursorUp.arrowUp', commandId:COMMAND.cursorUp, contextId:CONTEXT_ID, source:'default', description:'Move cursor up', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowup'}]}}},
  { id:'reassembly.manager.cursorDown.arrowDown', commandId:COMMAND.cursorDown, contextId:CONTEXT_ID, source:'default', description:'Move cursor down', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowdown'}]}}},
  { id:'reassembly.manager.open.enter', commandId:COMMAND.open, contextId:CONTEXT_ID, source:'default', description:'Open selected item', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'enter'}]}}},
  { id:'reassembly.manager.up.backspace', commandId:COMMAND.up, contextId:CONTEXT_ID, source:'default', description:'Parent directory', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'backspace'}]}}},
  { id:'reassembly.manager.back.altLeft', commandId:COMMAND.back, contextId:CONTEXT_ID, source:'default', description:'Back', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft', modifiers:['alt']}]}}},
  { id:'reassembly.manager.forward.altRight', commandId:COMMAND.forward, contextId:CONTEXT_ID, source:'default', description:'Forward', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'arrowright', modifiers:['alt']}]}}},
  { id:'reassembly.manager.location.ctrlL', commandId:COMMAND.location, contextId:CONTEXT_ID, source:'default', description:'Edit location', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'l', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.selectAll.ctrlA', commandId:COMMAND.selectAll, contextId:CONTEXT_ID, source:'default', description:'Select all', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'a', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.clearSelection.ctrlD', commandId:COMMAND.clearSelection, contextId:CONTEXT_ID, source:'default', description:'Clear selection', category:'Selection', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'d', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.refresh.ctrlR', commandId:COMMAND.refresh, contextId:CONTEXT_ID, source:'default', description:'Refresh active pane', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'r', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.rename.f2', commandId:COMMAND.rename, contextId:CONTEXT_ID, source:'default', description:'Rename selected item', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f2'}]}}},
  { id:'reassembly.manager.newFolder.f7', commandId:COMMAND.newFolder, contextId:CONTEXT_ID, source:'default', description:'Create folder', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f7'}]}}},
  { id:'reassembly.manager.delete.delete', commandId:COMMAND.deleteSelected, contextId:CONTEXT_ID, source:'default', description:'Delete selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'delete'}]}}},
  { id:'reassembly.manager.delete.f8', commandId:COMMAND.deleteSelected, contextId:CONTEXT_ID, source:'default', description:'Delete selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f8'}]}}},
  { id:'reassembly.manager.copy.ctrlC', commandId:COMMAND.copy, contextId:CONTEXT_ID, source:'default', description:'Copy selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'c', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.cut.ctrlX', commandId:COMMAND.cut, contextId:CONTEXT_ID, source:'default', description:'Cut selected items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'x', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.paste.ctrlV', commandId:COMMAND.paste, contextId:CONTEXT_ID, source:'default', description:'Paste items', category:'File Operations', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'v', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.copyToOther.f5', commandId:COMMAND.copyToOther, contextId:CONTEXT_ID, source:'default', description:'Copy to other pane', category:'Pane Control', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f5'}]}}},
  { id:'reassembly.manager.moveToOther.f6', commandId:COMMAND.moveToOther, contextId:CONTEXT_ID, source:'default', description:'Move to other pane', category:'Pane Control', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f6'}]}}},
  { id:'reassembly.manager.newTab.ctrlT', commandId:COMMAND.newTab, contextId:CONTEXT_ID, source:'default', description:'New tab', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'t', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.closeTab.ctrlW', commandId:COMMAND.closeTab, contextId:CONTEXT_ID, source:'default', description:'Close tab', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'w', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.nextTab.ctrlTab', commandId:COMMAND.nextTab, contextId:CONTEXT_ID, source:'default', description:'Next tab', category:'Tab Management', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'tab', modifiers:['ctrl']}]}}},
  { id:'reassembly.viewer.previous.left', commandId:COMMAND.viewerPrevious, contextId:CONTEXT_ID, source:'default', description:'Previous viewer item', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft'}]}}},
  { id:'reassembly.viewer.next.right', commandId:COMMAND.viewerNext, contextId:CONTEXT_ID, source:'default', description:'Next viewer item', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'arrowright'}]}}},
  { id:'reassembly.viewer.zoomIn.ctrlPlus', commandId:COMMAND.viewerZoomIn, contextId:CONTEXT_ID, source:'default', description:'Zoom viewer in', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'+', modifiers:['ctrl']}]}}},
  { id:'reassembly.viewer.zoomIn.ctrlEquals', commandId:COMMAND.viewerZoomIn, contextId:CONTEXT_ID, source:'default', description:'Zoom viewer in', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'=', modifiers:['ctrl']}]}}},
  { id:'reassembly.viewer.zoomOut.ctrlMinus', commandId:COMMAND.viewerZoomOut, contextId:CONTEXT_ID, source:'default', description:'Zoom viewer out', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'-', modifiers:['ctrl']}]}}},
  { id:'reassembly.viewer.fit.ctrl0', commandId:COMMAND.viewerFit, contextId:CONTEXT_ID, source:'default', description:'Fit viewer to window', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'0', modifiers:['ctrl']}]}}},
  { id:'reassembly.viewer.actualSize.ctrl1', commandId:COMMAND.viewerActualSize, contextId:CONTEXT_ID, source:'default', description:'Viewer actual size', category:'Viewer', when:'viewerSurface', trigger:{device:'keyboard', chord:{steps:[{key:'1', modifiers:['ctrl']}]}}},
  { id:'reassembly.manager.contextMenu.escape', commandId:COMMAND.closeContextMenu, contextId:CONTEXT_ID, source:'default', description:'Close context menu', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'escape'}]}}, preventDefault:true, stopPropagation:true },
  { id:'reassembly.manager.focusFilter.ctrlF', commandId:COMMAND.focusFilter, contextId:CONTEXT_ID, source:'default', description:'Focus folder filter', category:'Search & Filter', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'f', modifiers:['ctrl']}]}}},
  { id:'reassembly.workspace.toggleHidden.ctrlShiftH', commandId:COMMAND.toggleHidden, contextId:CONTEXT_ID, source:'default', description:'Show/hide hidden files', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'h', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.workspace.showSwitcher.ctrlShiftW', commandId:COMMAND.showWorkspaceSwitcher, contextId:CONTEXT_ID, source:'default', description:'Open workspace switcher', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'w', modifiers:['ctrl','shift']}]}}},
  { id:'reassembly.workspace.new.ctrlShiftAltN', commandId:COMMAND.newWorkspace, contextId:CONTEXT_ID, source:'default', description:'Create workspace', category:'Workspace', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'n', modifiers:['ctrl','shift','alt']}]}}},
  { id:'reassembly.navigation.pathHistory.ctrlH', commandId:COMMAND.showPathHistory, contextId:CONTEXT_ID, source:'default', description:'Show path history', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'h', modifiers:['ctrl']}]}}},
  { id:'reassembly.navigation.quickAccess.ctrlQ', commandId:COMMAND.showQuickAccess, contextId:CONTEXT_ID, source:'default', description:'Open Quick Access', category:'Navigation', when:'!typing', trigger:{device:'keyboard', chord:{steps:[{key:'q', modifiers:['ctrl']}]}}},
];

@Component({
  selector: 'app-reassembly-dual-pane',
  standalone: true,
  imports: [
    CommonModule,
    ReassemblyDirectoryPaneComponent,
    ReassemblyContextMenuComponent,
    ReassemblyWorkspaceSwitcherComponent,
    ReassemblyPathHistoryComponent,
    ReassemblyQuickAccessComponent,
  ],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section
      #managerRoot
      class="manager"
      [style.grid-template-columns]="layoutColumns">
      <div class="pane-slot">
        <app-reassembly-directory-pane
          #leftPane
          paneId="left"
          [active]="activePaneId === 'left'"
          [showHidden]="showHidden"
          (activateRequested)="activate('left')"
          (entryContextMenu)="openEntryMenu('left', $event.event, $event.entry)"
          (backgroundContextMenu)="openBackgroundMenu('left', $event)"
          (internalDrop)="handleInternalDrop('left', $event.event, $event.targetPath)">
        </app-reassembly-directory-pane>
      </div>

      <div
        class="divider"
        role="separator"
        aria-orientation="vertical"
        [attr.aria-valuenow]="leftPanePercent | number:'1.0-0'"
        [attr.aria-valuemin]="MIN_PANE_PERCENT"
        [attr.aria-valuemax]="MAX_PANE_PERCENT"
        (pointerdown)="beginResize($event)"
        (pointermove)="resizeFromPointer($event)"
        (pointerup)="endResize($event)"
        (pointercancel)="endResize($event)"
        (dblclick)="resetSplit()">
        <span class="divider-grip" aria-hidden="true"></span>
      </div>

      <div class="pane-slot">
        <app-reassembly-directory-pane
          #rightPane
          paneId="right"
          [active]="activePaneId === 'right'"
          [showHidden]="showHidden"
          (activateRequested)="activate('right')"
          (entryContextMenu)="openEntryMenu('right', $event.event, $event.entry)"
          (backgroundContextMenu)="openBackgroundMenu('right', $event)"
          (internalDrop)="handleInternalDrop('right', $event.event, $event.targetPath)">
        </app-reassembly-directory-pane>
      </div>

      <app-reassembly-workspace-switcher
        #workspaceSwitcher
        (workspaceActivated)="applyActiveWorkspace()">
      </app-reassembly-workspace-switcher>

      <app-reassembly-path-history
        #pathHistory
        (navigate)="navigateFromHistory($event)">
      </app-reassembly-path-history>

      <app-reassembly-quick-access
        #quickAccess
        (navigate)="navigateFromQuickAccess($event)">
      </app-reassembly-quick-access>

      <app-reassembly-context-menu
        [state]="contextMenu"
        (command)="dispatchContextCommand($event)"
        (closed)="closeContextMenu()">
      </app-reassembly-context-menu>
    </section>
  `,
  styles: [`
    :host {
      display: block;
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
    }

    .manager {
      display: grid;
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }

    .pane-slot {
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }

    .divider {
      position: relative;
      touch-action: none;
      cursor: col-resize;
      background:
        linear-gradient(
          90deg,
          transparent,
          var(--vsc-border, #3c3c3c) 40%,
          var(--vsc-border, #3c3c3c) 60%,
          transparent
        );
    }

    .divider:hover,
    .divider.resizing {
      background:
        linear-gradient(
          90deg,
          transparent,
          var(--vsc-accent-blue, #007acc) 35%,
          var(--vsc-accent-blue, #007acc) 65%,
          transparent
        );
    }

    .divider-grip {
      position: absolute;
      inset: 0 -3px;
    }

    @media (max-width: 820px) {
      .manager {
        grid-template-columns: 1fr !important;
        grid-template-rows: minmax(0, 1fr) 5px minmax(0, 1fr);
      }

      .divider {
        cursor: default;
        background:
          linear-gradient(
            180deg,
            transparent,
            var(--vsc-border, #3c3c3c) 40%,
            var(--vsc-border, #3c3c3c) 60%,
            transparent
          );
      }

      .divider-grip {
        display: none;
      }
    }
  `],
})
export class ReassemblyDualPaneComponent implements OnInit, OnDestroy {
  @ViewChild('managerRoot') managerRoot?: ElementRef<HTMLElement>;
  @ViewChild('workspaceSwitcher')
  workspaceSwitcher?: ReassemblyWorkspaceSwitcherComponent;
  @ViewChild('pathHistory')
  pathHistory?: ReassemblyPathHistoryComponent;
  @ViewChild('quickAccess') quickAccess?: ReassemblyQuickAccessComponent;
  @ViewChild('leftPane') leftPane?: ReassemblyDirectoryPaneComponent;
  @ViewChild('rightPane') rightPane?: ReassemblyDirectoryPaneComponent;

  activePaneId: PaneId = 'left';
  readonly MIN_PANE_PERCENT = 20;
  readonly MAX_PANE_PERCENT = 80;

  leftPanePercent = 50;
  resizing = false;
  showHidden = false;

  lastTransfer = 'none';
  lastDrop = 'none';
  contextMenu: ReassemblyContextMenuState | null = null;

  constructor(
    private readonly input: InputEngineService,
    private readonly api: ApiService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly workspaceService: WorkspaceService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get layoutColumns(): string {
    const left = this.clampPanePercent(this.leftPanePercent);
    return `minmax(0, ${left}fr) 5px minmax(0, ${100 - left}fr)`;
  }

  get activePane(): ReassemblyDirectoryPaneComponent | undefined {
    return this.activePaneId === 'left' ? this.leftPane : this.rightPane;
  }

  get otherPane(): ReassemblyDirectoryPaneComponent | undefined {
    return this.activePaneId === 'left' ? this.rightPane : this.leftPane;
  }

  ngOnInit(): void {
    this.restoreWorkspaceLayout();

    this.input.registerContext({
      id: CONTEXT_ID,
      kind: 'pane',
      priority: 100,
      parentId: 'global',
      description: 'Reassembly dual-pane file manager',
    });

    this.registerCommands();
    for (const binding of BINDINGS) this.input.registerBinding(binding);
    this.input.activateContext(CONTEXT_ID);
  }

  ngOnDestroy(): void {
    this.input.deactivateContext(CONTEXT_ID);
    for (const binding of BINDINGS) this.input.unregisterBinding(binding.id);
    for (const commandId of Object.values(COMMAND)) {
      this.input.unregisterCommandDescriptor(commandId, OWNER_ID);
    }
  }

  activate(paneId: PaneId): void {
    this.closeContextMenu();
    if (this.activePaneId === paneId) return;
    this.activePaneId = paneId;
    this.workspaceService.setActivePaneId(paneId);
    this.cdr.markForCheck();
  }

  switchPane(): void {
    this.activate(this.activePaneId === 'left' ? 'right' : 'left');
  }

  async copyToOther(): Promise<void> {
    await this.transferToOther('copy');
  }

  async moveToOther(): Promise<void> {
    await this.transferToOther('move');
  }

  async handleInternalDrop(
    targetPaneId: PaneId,
    event: DragEvent,
    targetPath: string,
  ): Promise<void> {
    const payload = this.readInternalDragPayload(event);
    if (!payload || payload.entries.length === 0 || !targetPath) return;

    const targetPane = targetPaneId === 'left' ? this.leftPane : this.rightPane;
    if (!targetPane) return;

    const sourcePane = payload.sourcePaneId === 'left' ? this.leftPane : this.rightPane;
    const operation: 'copy' | 'move' = event.shiftKey ? 'move' : 'copy';

    const sourceCapabilities = sourcePane?.sourceCapabilities ?? {
      read: true,
      create: false,
      delete: operation === 'move',
      rename: false,
      trash: false,
    };

    const decision = canTransferBetween(
      operation,
      sourceCapabilities,
      targetPane.sourceCapabilities,
    );

    if (!decision.allowed) {
      await this.confirmDialog.alert(
        transferBlockedMessage(operation, decision),
      );
      return;
    }

    const sourcePaths = payload.entries
      .map(entry => entry.path)
      .filter(path => !!path);

    if (sourcePaths.length === 0) return;

    // Dropping the same directory onto its own background is a no-op.
    if (
      payload.sourcePaneId === targetPaneId &&
      sourcePane?.path === targetPath
    ) {
      this.lastDrop = 'ignored: same directory';
      this.cdr.markForCheck();
      return;
    }

    try {
      const items = await Promise.all(
        sourcePaths.map(async sourcePath => ({
          source: sourcePath,
          destination: await this.api.joinPaths([
            targetPath,
            this.leafName(sourcePath),
          ]),
        })),
      );

      const result = operation === 'copy'
        ? await this.api.batchCopy(items, true)
        : await this.api.batchMove(items, true);

      this.lastDrop = `${operation} drop: ${result.succeeded}/${result.total}`;
      this.lastTransfer = this.lastDrop;
      targetPane.lastAction = this.lastDrop;

      await targetPane.refresh();

      if (
        operation === 'move' &&
        sourcePane &&
        sourcePane !== targetPane
      ) {
        await sourcePane.refresh();
      } else if (
        operation === 'move' &&
        sourcePane &&
        sourcePane === targetPane &&
        targetPath !== sourcePane.path
      ) {
        await sourcePane.refresh();
      }

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: 'Drop Completed with Errors',
          message: `${result.failed} item${result.failed === 1 ? '' : 's'} could not be ${operation === 'copy' ? 'copied' : 'moved'}.`,
        });
      }
    } catch (error) {
      await this.confirmDialog.alert({
        title: 'Drag & Drop Error',
        message: error instanceof Error ? error.message : String(error),
      });
    } finally {
      this.cdr.markForCheck();
    }
  }

  private readInternalDragPayload(event: DragEvent): {
    sourcePaneId: PaneId;
    entries: Array<{ path: string; name?: string; type?: unknown }>;
  } | null {
    try {
      const raw = event.dataTransfer?.getData('application/json');
      if (!raw) return null;

      const parsed = JSON.parse(raw) as {
        sourcePaneId?: unknown;
        entries?: unknown;
      };

      const sourcePaneId: PaneId =
        parsed.sourcePaneId === 'right' ? 'right' : 'left';

      const entries = Array.isArray(parsed.entries)
        ? parsed.entries.filter(
            (entry): entry is { path: string; name?: string; type?: unknown } =>
              !!entry &&
              typeof entry === 'object' &&
              typeof (entry as { path?: unknown }).path === 'string',
          )
        : [];

      return { sourcePaneId, entries };
    } catch {
      return null;
    }
  }

  openEntryMenu(
    paneId: PaneId,
    event: MouseEvent,
    entry: FileInfo,
  ): void {
    this.activePaneId = paneId;

    const items: ReassemblyContextMenuItem[] = [
      this.menuCommand(COMMAND.open, 'Open', 'Enter'),
      { separator: true },
      this.menuCommand(COMMAND.rename, 'Rename', 'F2'),
      this.menuCommand(COMMAND.copy, 'Copy', 'Ctrl+C'),
      this.menuCommand(COMMAND.cut, 'Cut', 'Ctrl+X'),
      this.menuCommand(COMMAND.paste, 'Paste', 'Ctrl+V'),
      { separator: true },
      this.menuCommand(COMMAND.copyToOther, 'Copy to other pane', 'F5'),
      this.menuCommand(COMMAND.moveToOther, 'Move to other pane', 'F6'),
      { separator: true },
      this.menuCommand(COMMAND.deleteSelected, 'Delete', 'Delete'),
    ];

    this.contextMenu = {
      x: this.clampMenuX(event.clientX),
      y: this.clampMenuY(event.clientY),
      title: entry.name,
      items,
    };
    this.cdr.markForCheck();
  }

  openBackgroundMenu(paneId: PaneId, event: MouseEvent): void {
    this.activePaneId = paneId;

    this.contextMenu = {
      x: this.clampMenuX(event.clientX),
      y: this.clampMenuY(event.clientY),
      title: this.activePane?.path || 'Folder',
      items: [
        this.menuCommand(COMMAND.newFolder, 'New folder', 'F7'),
        this.menuCommand(COMMAND.paste, 'Paste', 'Ctrl+V'),
        { separator: true },
        this.menuCommand(COMMAND.selectAll, 'Select all', 'Ctrl+A'),
        this.menuCommand(COMMAND.refresh, 'Refresh', 'Ctrl+R'),
      ],
    };
    this.cdr.markForCheck();
  }

  dispatchContextCommand(commandId: string): void {
    this.closeContextMenu();
    this.input.dispatchCommandById(commandId, {
      source: 'context-menu',
      context: {
        paneId: this.activePaneId,
        path: this.activePane?.path ?? '',
      },
    });
  }

  closeContextMenu(): void {
    if (!this.contextMenu) return;
    this.contextMenu = null;
    this.cdr.markForCheck();
  }

  private menuCommand(
    commandId: string,
    label: string,
    shortcut?: string,
  ): ReassemblyContextMenuItem {
    const state = this.input.getCommandState(commandId);
    return {
      commandId,
      label,
      shortcut,
      enabled: !!state?.registered && state.enabled,
    };
  }

  private clampMenuX(x: number): number {
    if (typeof window === 'undefined') return x;
    return Math.max(8, Math.min(x, window.innerWidth - 258));
  }

  private clampMenuY(y: number): number {
    if (typeof window === 'undefined') return y;
    return Math.max(8, Math.min(y, window.innerHeight - 360));
  }

  navigateFromHistory(path: string): void {
    const pane = this.activePane;
    if (!pane?.isDirectoryTab) return;
    pane.navigateTo(path);
    pane.lastAction = `history: ${path}`;
    this.cdr.markForCheck();
  }

  navigateFromQuickAccess(path: string): void {
    const pane = this.activePane;
    if (!pane?.isDirectoryTab) return;
    pane.navigateTo(path);
    pane.lastAction = `quick access: ${path}`;
    this.cdr.markForCheck();
  }

  async applyActiveWorkspace(): Promise<void> {
    const workspace = this.workspaceService.getActiveWorkspace();
    if (!workspace) return;

    this.activePaneId =
      workspace.activePaneId === 'right' ? 'right' : 'left';
    this.showHidden = !!workspace.showHidden;

    this.leftPanePercent =
      typeof workspace.splitRatio === 'number' &&
      Number.isFinite(workspace.splitRatio) &&
      workspace.splitRatio > 0
        ? this.clampPanePercent(workspace.splitRatio)
        : 50;

    await Promise.all([
      this.leftPane?.restoreWorkspaceState(),
      this.rightPane?.restoreWorkspaceState(),
    ]);

    this.lastTransfer = 'none';
    this.lastDrop = 'none';
    this.closeContextMenu();
    this.cdr.markForCheck();
  }

  async toggleHidden(): Promise<void> {
    const next = !this.showHidden;
    this.showHidden = next;

    const workspace = this.workspaceService.getActiveWorkspace();
    if (workspace) {
      this.workspaceService.updateWorkspace(workspace.id, {
        showHidden: next,
      });
    }

    const panes = [this.leftPane, this.rightPane]
      .filter((pane): pane is ReassemblyDirectoryPaneComponent => !!pane);

    await Promise.all(
      panes
        .filter(pane => pane.isDirectoryTab)
        .map(pane => pane.refresh()),
    );

    for (const pane of panes) {
      pane.lastAction = next ? 'show hidden' : 'hide hidden';
    }

    this.cdr.markForCheck();
  }

  beginResize(event: PointerEvent): void {
    if (typeof window !== 'undefined' && window.innerWidth <= 820) return;

    const target = event.currentTarget as HTMLElement | null;
    target?.setPointerCapture?.(event.pointerId);
    this.resizing = true;
    target?.classList.add('resizing');
    event.preventDefault();
  }

  resizeFromPointer(event: PointerEvent): void {
    if (!this.resizing) return;

    const root = this.managerRoot?.nativeElement;
    if (!root) return;

    const rect = root.getBoundingClientRect();
    if (rect.width <= 0) return;

    const percent = ((event.clientX - rect.left) / rect.width) * 100;
    this.leftPanePercent = this.clampPanePercent(percent);
    this.cdr.markForCheck();
  }

  endResize(event: PointerEvent): void {
    if (!this.resizing) return;

    const target = event.currentTarget as HTMLElement | null;
    if (target?.hasPointerCapture?.(event.pointerId)) {
      target.releasePointerCapture(event.pointerId);
    }

    this.resizing = false;
    target?.classList.remove('resizing');
    this.workspaceService.setSplitRatio(this.leftPanePercent);
    this.cdr.markForCheck();
  }

  resetSplit(): void {
    this.leftPanePercent = 50;
    this.workspaceService.setSplitRatio(50);
    this.cdr.markForCheck();
  }

  private restoreWorkspaceLayout(): void {
    const workspace = this.workspaceService.getActiveWorkspace();
    if (!workspace) {
      // WorkspaceService hydrates asynchronously. Recheck briefly without
      // creating a second persistence model.
      setTimeout(() => this.restoreWorkspaceLayout(), 100);
      return;
    }

    this.activePaneId = workspace.activePaneId === 'right' ? 'right' : 'left';
    this.showHidden = !!workspace.showHidden;

    if (
      typeof workspace.splitRatio === 'number' &&
      Number.isFinite(workspace.splitRatio) &&
      workspace.splitRatio > 0
    ) {
      this.leftPanePercent = this.clampPanePercent(workspace.splitRatio);
    }

    this.cdr.markForCheck();
  }

  private clampPanePercent(value: number): number {
    return Math.max(
      this.MIN_PANE_PERCENT,
      Math.min(this.MAX_PANE_PERCENT, value),
    );
  }

  private registerCommands(): void {
    const descriptor = (
      id: string,
      title: string,
      category: string,
      execute: () => void | Promise<void>,
      enabled?: () => boolean,
    ) => this.input.registerCommandDescriptor({
      id, title, category, execute, isEnabled: enabled,
    }, OWNER_ID);

    descriptor(COMMAND.switchPane, 'Switch Pane', 'Pane Control', () => this.switchPane());
    descriptor(COMMAND.cursorUp, 'Move Cursor Up', 'Navigation', () => this.activePane?.moveCursor(-1));
    descriptor(COMMAND.cursorDown, 'Move Cursor Down', 'Navigation', () => this.activePane?.moveCursor(1));
    descriptor(COMMAND.open, 'Open Selected Item', 'Navigation', () => {
      const pane = this.activePane;
      const entry = pane?.cursorEntry;
      if (pane && entry) pane.openEntry(entry);
    });
    descriptor(COMMAND.up, 'Parent Directory', 'Navigation', () => this.activePane?.navigateUp());
    descriptor(COMMAND.back, 'Back', 'Navigation', () => this.activePane?.goBack());
    descriptor(COMMAND.forward, 'Forward', 'Navigation', () => this.activePane?.goForward());
    descriptor(COMMAND.location, 'Edit Location', 'Navigation', () => this.activePane?.focusLocation());
    descriptor(COMMAND.selectAll, 'Select All', 'Selection', () => this.activePane?.selectAll());
    descriptor(COMMAND.clearSelection, 'Clear Selection', 'Selection', () => this.activePane?.clearSelection());
    descriptor(COMMAND.refresh, 'Refresh', 'Navigation', () => this.activePane?.refresh());
    descriptor(COMMAND.rename, 'Rename', 'File Operations', () => this.activePane?.renameSelected(), () => !!this.activePane?.canRename);
    descriptor(COMMAND.newFolder, 'New Folder', 'File Operations', () => this.activePane?.newFolder(), () => !!this.activePane?.canCreate);
    descriptor(COMMAND.deleteSelected, 'Delete', 'File Operations', () => this.deleteActiveSelection(), () => this.canDeleteActiveSelection());
    descriptor(COMMAND.copy, 'Copy', 'File Operations', () => this.activePane?.copySelected(), () => !!this.activePane?.hasSelection);
    descriptor(COMMAND.cut, 'Cut', 'File Operations', () => this.activePane?.cutSelected(), () => !!this.activePane?.canCut);
    descriptor(COMMAND.paste, 'Paste', 'File Operations', () => this.activePane?.paste(), () => !!this.activePane?.canPaste);
    descriptor(COMMAND.copyToOther, 'Copy to Other Pane', 'Pane Control', () => this.copyToOther(), () => this.canTransfer('copy'));
    descriptor(COMMAND.moveToOther, 'Move to Other Pane', 'Pane Control', () => this.moveToOther(), () => this.canTransfer('move'));
    descriptor(COMMAND.newTab, 'New Tab', 'Tab Management', () => this.activePane?.newTab());
    descriptor(COMMAND.closeTab, 'Close Tab', 'Tab Management', () => this.activePane?.closeActiveTab());
    descriptor(COMMAND.nextTab, 'Next Tab', 'Tab Management', () => this.activePane?.nextTab());
    descriptor(COMMAND.viewerPrevious, 'Previous Viewer Item', 'Viewer', () => this.invokeViewer('showPrevious'), () => this.hasViewerMethod('showPrevious'));
    descriptor(COMMAND.viewerNext, 'Next Viewer Item', 'Viewer', () => this.invokeViewer('showNext'), () => this.hasViewerMethod('showNext'));
    descriptor(COMMAND.viewerZoomIn, 'Zoom Viewer In', 'Viewer', () => this.invokeViewer('zoomIn'), () => this.hasViewerMethod('zoomIn'));
    descriptor(COMMAND.viewerZoomOut, 'Zoom Viewer Out', 'Viewer', () => this.invokeViewer('zoomOut'), () => this.hasViewerMethod('zoomOut'));
    descriptor(COMMAND.viewerFit, 'Fit Viewer', 'Viewer', () => this.invokeViewer('resetZoom'), () => this.hasViewerMethod('resetZoom'));
    descriptor(COMMAND.viewerActualSize, 'Viewer Actual Size', 'Viewer', () => this.invokeViewer('zoom100'), () => this.hasViewerMethod('zoom100'));
    descriptor(COMMAND.closeContextMenu, 'Close Context Menu', 'Navigation', () => this.closeContextMenu(), () => !!this.contextMenu);
    descriptor(COMMAND.focusFilter, 'Focus Folder Filter', 'Search & Filter', () => this.activePane?.focusFilter(), () => !!this.activePane?.isDirectoryTab);
    descriptor(COMMAND.toggleHidden, 'Toggle Hidden Files', 'Workspace', () => this.toggleHidden());
    descriptor(COMMAND.showWorkspaceSwitcher, 'Show Workspace Switcher', 'Workspace', () => this.workspaceSwitcher?.open());
    descriptor(COMMAND.newWorkspace, 'New Workspace', 'Workspace', () => this.workspaceSwitcher?.createWorkspace());
    descriptor(COMMAND.showPathHistory, 'Show Path History', 'Navigation', () => this.pathHistory?.open());
    descriptor(COMMAND.showQuickAccess, 'Open Quick Access', 'Navigation', () => this.quickAccess?.open());
  }

  private activeViewer(): any {
    const pane = this.activePane;
    if (!pane || pane.activeTab?.kind !== 'viewer') return null;
    return pane.mountedViewer;
  }

  private hasViewerMethod(method: string): boolean {
    const viewer = this.activeViewer();
    return !!viewer && typeof viewer[method] === 'function';
  }

  private invokeViewer(method: string): void {
    const viewer = this.activeViewer();
    const fn = viewer?.[method];
    if (typeof fn !== 'function') return;
    fn.call(viewer);
    if (this.activePane) this.activePane.lastAction = `viewer: ${method}`;
    this.cdr.markForCheck();
  }

  private canDeleteActiveSelection(): boolean {
    const pane = this.activePane;
    if (!pane) return false;
    if (pane.activeTab?.kind === 'viewer') return this.hasViewerMethod('deleteSelected');
    return pane.canDelete;
  }

  private deleteActiveSelection(): void {
    const pane = this.activePane;
    if (!pane) return;
    if (pane.activeTab?.kind === 'viewer') {
      this.invokeViewer('deleteSelected');
      return;
    }
    void pane.deleteSelected();
  }

  private canTransfer(operation: 'copy' | 'move'): boolean {
    const source = this.activePane;
    const destination = this.otherPane;
    if (!source || !destination || !source.hasSelection || !destination.path) return false;
    return canTransferBetween(operation, source.sourceCapabilities, destination.sourceCapabilities).allowed;
  }

  private async transferToOther(operation: 'copy' | 'move'): Promise<void> {
    const source = this.activePane;
    const destination = this.otherPane;
    if (!source || !destination) return;

    const selected = Array.from(source.selectedPaths);
    if (selected.length === 0) return;

    const decision = canTransferBetween(operation, source.sourceCapabilities, destination.sourceCapabilities);
    if (!decision.allowed) {
      await this.confirmDialog.alert(transferBlockedMessage(operation, decision));
      return;
    }

    try {
      const items = await Promise.all(selected.map(async sourcePath => ({
        source: sourcePath,
        destination: await this.api.joinPaths([destination.path, this.leafName(sourcePath)]),
      })));

      const result = operation === 'copy'
        ? await this.api.batchCopy(items, true)
        : await this.api.batchMove(items, true);

      this.lastTransfer = `${operation}: ${result.succeeded}/${result.total}`;
      source.lastAction = `${operation} to ${this.otherPaneId()}: ${result.succeeded}/${result.total}`;
      destination.lastAction = `received ${operation}: ${result.succeeded}/${result.total}`;

      await destination.refresh();
      if (operation === 'move') {
        await source.refresh();
      }

      if (result.failed > 0) {
        await this.confirmDialog.alert({
          title: `${operation === 'copy' ? 'Copy' : 'Move'} Completed with Errors`,
          message: `${result.failed} item${result.failed === 1 ? '' : 's'} could not be transferred.`,
        });
      }
    } catch (error) {
      await this.confirmDialog.alert({
        title: `${operation === 'copy' ? 'Copy' : 'Move'} Error`,
        message: error instanceof Error ? error.message : String(error),
      });
    } finally {
      this.cdr.markForCheck();
    }
  }

  private otherPaneId(): PaneId {
    return this.activePaneId === 'left' ? 'right' : 'left';
  }

  private leafName(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/').replace(/\/+$/, '');
    const index = normalized.lastIndexOf('/');
    return index >= 0 ? normalized.slice(index + 1) : normalized;
  }
}
