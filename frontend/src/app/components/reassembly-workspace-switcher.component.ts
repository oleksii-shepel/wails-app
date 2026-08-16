import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  OnDestroy,
  OnInit,
  Output,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { WorkspaceService } from '../services/workspace.service';
import type { WorkspaceConfig } from '../shared/protocol-enhanced';

const CONTEXT_ID = 'overlay.reassemblyWorkspace';
const OWNER_ID = 'overlay.reassemblyWorkspace';

const COMMAND = {
  close: 'reassembly.workspaceSwitcher.close',
  previous: 'reassembly.workspaceSwitcher.previous',
  next: 'reassembly.workspaceSwitcher.next',
  activate: 'reassembly.workspaceSwitcher.activate',
  create: 'reassembly.workspaceSwitcher.create',
} as const;

const BINDINGS: readonly InputBindingDefinition[] = [
  {
    id: 'reassembly.workspaceSwitcher.escape',
    commandId: COMMAND.close,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Close workspace switcher',
    category: 'Workspace',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'escape' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.up',
    commandId: COMMAND.previous,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Previous workspace',
    category: 'Workspace',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'arrowup' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.down',
    commandId: COMMAND.next,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Next workspace',
    category: 'Workspace',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'arrowdown' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.enter',
    commandId: COMMAND.activate,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace',
    category: 'Workspace',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'enter' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.new.ctrlShiftAltN',
    commandId: COMMAND.create,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Create workspace',
    category: 'Workspace',
    trigger: {
      device: 'keyboard',
      chord: {
        steps: [{
          key: 'n',
          modifiers: ['ctrl', 'shift', 'alt'],
        }],
      },
    },
  },
];

@Component({
  selector: 'app-reassembly-workspace-switcher',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop" *ngIf="visible" (mousedown)="close()">
      <section
        class="switcher"
        role="dialog"
        aria-modal="true"
        aria-label="Workspace switcher"
        (mousedown)="$event.stopPropagation()">

        <header>
          <div>
            <strong>Workspaces</strong>
            <span>{{ workspaces.length }}</span>
          </div>

          <button
            type="button"
            class="new"
            title="New workspace (Ctrl+Shift+Alt+N)"
            (click)="createWorkspace()">
            + New
          </button>
        </header>

        <div class="workspace-list" role="listbox">
          <button
            *ngFor="let workspace of workspaces; let i = index; trackBy: trackWorkspace"
            type="button"
            class="workspace"
            role="option"
            [class.selected]="i === selectedIndex"
            [class.active]="workspace.id === activeWorkspaceId"
            [attr.aria-selected]="i === selectedIndex"
            (mouseenter)="selectedIndex = i"
            (click)="activateWorkspace(workspace)">

            <span class="icon">{{ workspace.icon || '📁' }}</span>

            <span class="details">
              <strong>{{ workspace.name }}</strong>
              <small>
                {{ workspace.leftPane.tabs.length }} left ·
                {{ workspace.rightPane.tabs.length }} right
              </small>
            </span>

            <span class="active-mark" *ngIf="workspace.id === activeWorkspaceId">
              active
            </span>
          </button>
        </div>

        <footer>
          <button
            type="button"
            [disabled]="!selectedWorkspace"
            (click)="renameSelected()">
            Rename
          </button>

          <button
            type="button"
            class="danger"
            [disabled]="workspaces.length <= 1 || !selectedWorkspace"
            (click)="deleteSelected()">
            Delete
          </button>

          <span class="keys">↑ ↓ · Enter · Esc</span>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    :host {
      position: fixed;
      inset: 0;
      z-index: 9500;
      pointer-events: none;
    }

    .backdrop {
      position: fixed;
      inset: 0;
      display: grid;
      place-items: start center;
      padding-top: min(12vh, 110px);
      background: rgba(0, 0, 0, .28);
      pointer-events: auto;
    }

    .switcher {
      box-sizing: border-box;
      width: min(520px, calc(100vw - 24px));
      max-height: min(650px, calc(100vh - 40px));
      display: flex;
      flex-direction: column;
      overflow: hidden;
      border: 1px solid var(--vsc-border, #454545);
      border-radius: 8px;
      background: var(--vsc-menu-background, #252526);
      color: var(--vsc-foreground, #cccccc);
      box-shadow: 0 18px 60px rgba(0, 0, 0, .48);
      font: 12px/1.4 var(--vsc-font-family, system-ui, sans-serif);
    }

    header,
    footer {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 9px 10px;
    }

    header {
      justify-content: space-between;
      border-bottom: 1px solid var(--vsc-border-subtle, #2b2b2b);
    }

    header > div {
      display: flex;
      align-items: baseline;
      gap: 8px;
    }

    header span,
    .keys,
    small {
      color: var(--vsc-description-foreground, #969696);
    }

    button {
      border: 1px solid transparent;
      border-radius: 4px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font: inherit;
    }

    button:hover:not(:disabled) {
      background: var(--vsc-list-hover-background, #2a2d2e);
      border-color: var(--vsc-border, #454545);
    }

    button:disabled {
      opacity: .35;
      cursor: default;
    }

    .new,
    footer button {
      min-height: 28px;
      padding: 4px 9px;
    }

    .workspace-list {
      min-height: 0;
      overflow: auto;
      padding: 5px;
    }

    .workspace {
      display: grid;
      grid-template-columns: 30px minmax(0, 1fr) auto;
      gap: 8px;
      align-items: center;
      width: 100%;
      min-height: 48px;
      padding: 6px 8px;
      text-align: left;
    }

    .workspace.selected {
      background: var(--vsc-list-hover-background, #2a2d2e);
    }

    .workspace.active {
      box-shadow: inset 2px 0 0 var(--vsc-accent-blue, #007acc);
    }

    .icon {
      text-align: center;
      font-size: 17px;
    }

    .details {
      min-width: 0;
      display: grid;
    }

    .details strong,
    .details small {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .active-mark {
      color: var(--vsc-accent-blue, #4aa3ff);
      font-size: 10px;
      text-transform: uppercase;
    }

    footer {
      border-top: 1px solid var(--vsc-border-subtle, #2b2b2b);
    }

    footer .danger:not(:disabled) {
      color: var(--vsc-error-foreground, #f48771);
    }

    .keys {
      margin-left: auto;
      font-size: 10px;
    }
  `],
})
export class ReassemblyWorkspaceSwitcherComponent
  implements OnInit, OnDestroy {

  @Output() workspaceActivated = new EventEmitter<void>();

  visible = false;
  selectedIndex = 0;

  constructor(
    private readonly workspacesService: WorkspaceService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly input: InputEngineService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get workspaces(): WorkspaceConfig[] {
    return this.workspacesService.getWorkspaces().workspaces ?? [];
  }

  get activeWorkspaceId(): string {
    return this.workspacesService.getActiveWorkspace()?.id ?? '';
  }

  get selectedWorkspace(): WorkspaceConfig | null {
    return this.workspaces[this.selectedIndex] ?? null;
  }

  ngOnInit(): void {
    this.input.registerContext({
      id: CONTEXT_ID,
      kind: 'overlay',
      priority: 400,
      parentId: 'global',
      description: 'Workspace switcher',
    });

    this.input.registerCommandDescriptor({
      id: COMMAND.close,
      title: 'Close Workspace Switcher',
      category: 'Workspace',
      execute: () => this.close(),
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.previous,
      title: 'Previous Workspace',
      category: 'Workspace',
      execute: () => this.moveSelection(-1),
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.next,
      title: 'Next Workspace',
      category: 'Workspace',
      execute: () => this.moveSelection(1),
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.activate,
      title: 'Activate Workspace',
      category: 'Workspace',
      execute: () => {
        const workspace = this.selectedWorkspace;
        if (workspace) this.activateWorkspace(workspace);
      },
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.create,
      title: 'Create Workspace',
      category: 'Workspace',
      execute: () => this.createWorkspace(),
    }, OWNER_ID);

    for (const binding of BINDINGS) {
      this.input.registerBinding(binding);
    }
  }

  ngOnDestroy(): void {
    this.input.deactivateContext(CONTEXT_ID);

    for (const binding of BINDINGS) {
      this.input.unregisterBinding(binding.id);
    }

    for (const commandId of Object.values(COMMAND)) {
      this.input.unregisterCommandDescriptor(commandId, OWNER_ID);
    }
  }

  open(): void {
    const activeIndex = this.workspaces.findIndex(
      workspace => workspace.id === this.activeWorkspaceId,
    );

    this.selectedIndex = activeIndex >= 0 ? activeIndex : 0;
    this.visible = true;
    this.input.activateContext(CONTEXT_ID);
    this.cdr.markForCheck();
  }

  close(): void {
    if (!this.visible) return;

    this.visible = false;
    this.input.deactivateContext(CONTEXT_ID);
    this.cdr.markForCheck();
  }

  async createWorkspace(): Promise<void> {
    const name = (await this.confirmDialog.prompt({
      title: 'New Workspace',
      message: 'Enter workspace name.',
      confirmText: 'Create',
      placeholder: 'Workspace name',
      defaultValue: '',
      historyKey: 'workspace.name',
    }))?.trim();

    if (!name) return;

    const workspace = await this.workspacesService.createWorkspace(name);
    this.workspacesService.switchWorkspace(workspace.id);
    this.workspaceActivated.emit();

    if (this.visible) {
      this.selectedIndex = Math.max(
        0,
        this.workspaces.findIndex(item => item.id === workspace.id),
      );
      this.cdr.markForCheck();
    }
  }

  activateWorkspace(workspace: WorkspaceConfig): void {
    if (!this.workspacesService.switchWorkspace(workspace.id)) return;

    this.workspaceActivated.emit();
    this.close();
  }

  async renameSelected(): Promise<void> {
    const workspace = this.selectedWorkspace;
    if (!workspace) return;

    const name = (await this.confirmDialog.prompt({
      title: 'Rename Workspace',
      message: 'Enter new workspace name.',
      confirmText: 'Rename',
      defaultValue: workspace.name,
      historyKey: 'workspace.name',
    }))?.trim();

    if (!name || name === workspace.name) return;

    this.workspacesService.updateWorkspace(workspace.id, { name });
    this.cdr.markForCheck();
  }

  async deleteSelected(): Promise<void> {
    const workspace = this.selectedWorkspace;
    if (!workspace || this.workspaces.length <= 1) return;

    const confirmed = await this.confirmDialog.confirm({
      title: 'Delete Workspace',
      message: `Delete workspace "${workspace.name}"?`,
      confirmText: 'Delete',
      danger: true,
    });

    if (!confirmed) return;

    const wasActive = workspace.id === this.activeWorkspaceId;
    if (!this.workspacesService.deleteWorkspace(workspace.id)) return;

    this.selectedIndex = Math.min(
      this.selectedIndex,
      Math.max(0, this.workspaces.length - 1),
    );

    if (wasActive) {
      this.workspaceActivated.emit();
    }

    this.cdr.markForCheck();
  }

  trackWorkspace(_index: number, workspace: WorkspaceConfig): string {
    return workspace.id;
  }

  private moveSelection(delta: -1 | 1): void {
    const count = this.workspaces.length;
    if (count === 0) return;

    this.selectedIndex =
      (this.selectedIndex + delta + count) % count;
    this.cdr.markForCheck();
  }
}
