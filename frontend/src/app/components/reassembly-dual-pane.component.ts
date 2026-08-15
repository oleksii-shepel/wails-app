import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  OnDestroy,
  OnInit,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { ApiService } from '../services/api.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { canTransferBetween, transferBlockedMessage } from '../services/transfer-capabilities';
import { ReassemblyDirectoryPaneComponent } from './reassembly-directory-pane.component';

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
];

@Component({
  selector: 'app-reassembly-dual-pane',
  standalone: true,
  imports: [CommonModule, ReassemblyDirectoryPaneComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section class="manager">
      <div class="pane-slot">
        <app-reassembly-directory-pane
          #leftPane
          paneId="left"
          [active]="activePaneId === 'left'"
          (activateRequested)="activate('left')">
        </app-reassembly-directory-pane>
      </div>

      <div class="divider" aria-hidden="true"></div>

      <div class="pane-slot">
        <app-reassembly-directory-pane
          #rightPane
          paneId="right"
          [active]="activePaneId === 'right'"
          (activateRequested)="activate('right')">
        </app-reassembly-directory-pane>
      </div>
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
      grid-template-columns: minmax(0, 1fr) 5px minmax(0, 1fr);
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
      background:
        linear-gradient(
          90deg,
          transparent,
          var(--vsc-border, #3c3c3c) 40%,
          var(--vsc-border, #3c3c3c) 60%,
          transparent
        );
    }

    @media (max-width: 820px) {
      .manager {
        grid-template-columns: 1fr;
        grid-template-rows: minmax(0, 1fr) 5px minmax(0, 1fr);
      }

      .divider {
        background:
          linear-gradient(
            180deg,
            transparent,
            var(--vsc-border, #3c3c3c) 40%,
            var(--vsc-border, #3c3c3c) 60%,
            transparent
          );
      }
    }
  `],
})
export class ReassemblyDualPaneComponent implements OnInit, OnDestroy {
  @ViewChild('leftPane') leftPane?: ReassemblyDirectoryPaneComponent;
  @ViewChild('rightPane') rightPane?: ReassemblyDirectoryPaneComponent;

  activePaneId: PaneId = 'left';
  lastTransfer = 'none';

  constructor(
    private readonly input: InputEngineService,
    private readonly api: ApiService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get activePane(): ReassemblyDirectoryPaneComponent | undefined {
    return this.activePaneId === 'left' ? this.leftPane : this.rightPane;
  }

  get otherPane(): ReassemblyDirectoryPaneComponent | undefined {
    return this.activePaneId === 'left' ? this.rightPane : this.leftPane;
  }

  ngOnInit(): void {
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
    if (this.activePaneId === paneId) return;
    this.activePaneId = paneId;
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
    descriptor(COMMAND.deleteSelected, 'Delete', 'File Operations', () => this.activePane?.deleteSelected(), () => !!this.activePane?.canDelete);
    descriptor(COMMAND.copy, 'Copy', 'File Operations', () => this.activePane?.copySelected(), () => !!this.activePane?.hasSelection);
    descriptor(COMMAND.cut, 'Cut', 'File Operations', () => this.activePane?.cutSelected(), () => !!this.activePane?.canCut);
    descriptor(COMMAND.paste, 'Paste', 'File Operations', () => this.activePane?.paste(), () => !!this.activePane?.canPaste);
    descriptor(COMMAND.copyToOther, 'Copy to Other Pane', 'Pane Control', () => this.copyToOther(), () => this.canTransfer('copy'));
    descriptor(COMMAND.moveToOther, 'Move to Other Pane', 'Pane Control', () => this.moveToOther(), () => this.canTransfer('move'));
    descriptor(COMMAND.newTab, 'New Tab', 'Tab Management', () => this.activePane?.newTab());
    descriptor(COMMAND.closeTab, 'Close Tab', 'Tab Management', () => this.activePane?.closeActiveTab());
    descriptor(COMMAND.nextTab, 'Next Tab', 'Tab Management', () => this.activePane?.nextTab());
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
