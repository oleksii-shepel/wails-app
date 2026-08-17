import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  OnDestroy,
  OnInit,
  Output,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { WorkspaceService } from '../services/workspace.service';
import { WorkspaceSurfaceService } from '../services/workspace-surface.service';
import type { WorkspacePresentation } from '../services/workspace-surface.model';
import type { WorkspaceConfig } from '../shared/protocol-enhanced';
import { ReassemblyModalSurfaceDirective } from './reassembly-modal-surface.directive';
type WorkspaceEdit =
  | { kind: 'create'; draft: string }
  | { kind: 'rename'; workspaceId: string; draft: string }
  | null;

const CONTEXT_ID = 'overlay.reassemblyWorkspace';
const OWNER_ID = 'overlay.reassemblyWorkspace';

const COMMAND = {
  close: 'reassembly.workspaceSwitcher.close',
  previous: 'reassembly.workspaceSwitcher.previous',
  next: 'reassembly.workspaceSwitcher.next',
  activate: 'reassembly.workspaceSwitcher.activate',
  create: 'reassembly.workspaceSwitcher.create',
  rename: 'reassembly.workspaceSwitcher.rename',
  remove: 'reassembly.workspaceSwitcher.remove',
  slot1: 'reassembly.workspaceSwitcher.slot1',
  slot2: 'reassembly.workspaceSwitcher.slot2',
  slot3: 'reassembly.workspaceSwitcher.slot3',
  slot4: 'reassembly.workspaceSwitcher.slot4',
  slot5: 'reassembly.workspaceSwitcher.slot5',
  slot6: 'reassembly.workspaceSwitcher.slot6',
  slot7: 'reassembly.workspaceSwitcher.slot7',
  slot8: 'reassembly.workspaceSwitcher.slot8',
  slot9: 'reassembly.workspaceSwitcher.slot9',
} as const;

const BINDINGS: readonly InputBindingDefinition[] = [
  {
    id: 'reassembly.workspaceSwitcher.escape',
    commandId: COMMAND.close,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Close workspace switcher',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'escape' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.up',
    commandId: COMMAND.previous,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Previous workspace',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'arrowup' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.down',
    commandId: COMMAND.next,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Next workspace',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'arrowdown' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.enter',
    commandId: COMMAND.activate,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'enter' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.rename.f2',
    commandId: COMMAND.rename,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Rename workspace',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'f2' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.remove.delete',
    commandId: COMMAND.remove,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Delete workspace',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'delete' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.slot1',
    commandId: COMMAND.slot1,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace 1',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: '1' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.slot2',
    commandId: COMMAND.slot2,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace 2',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: '2' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.slot3',
    commandId: COMMAND.slot3,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace 3',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: '3' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.slot4',
    commandId: COMMAND.slot4,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace 4',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: '4' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.slot5',
    commandId: COMMAND.slot5,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace 5',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: '5' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.slot6',
    commandId: COMMAND.slot6,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace 6',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: '6' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.slot7',
    commandId: COMMAND.slot7,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace 7',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: '7' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.slot8',
    commandId: COMMAND.slot8,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace 8',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: '8' }] } },
  },
  {
    id: 'reassembly.workspaceSwitcher.slot9',
    commandId: COMMAND.slot9,
    contextId: CONTEXT_ID,
    source: 'default',
    description: 'Activate workspace 9',
    category: 'Workspace',
    when: '!typing',
    trigger: { device: 'keyboard', chord: { steps: [{ key: '9' }] } },
  },
];

interface WorkspaceSwitcherItem {
  readonly id: string;
  readonly name: string;
  readonly icon?: string;
  readonly kind: 'files' | 'tool';
  readonly files?: WorkspaceConfig;
  readonly tool?: WorkspacePresentation;
}

@Component({
  selector: 'app-reassembly-workspace-switcher',
  standalone: true,
  imports: [CommonModule, FormsModule, ReassemblyModalSurfaceDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop" *ngIf="visible" (mousedown)="close()">
      <section
        class="switcher"
        role="dialog"
        aria-modal="true"
        aria-label="Workspace switcher"
        (mousedown)="$event.stopPropagation()"
        reassemblyModalSurface
        (reassemblyModalDismiss)="close()"
        reassemblyModalInitial="[aria-selected='true']"
        tabindex="-1">

        <header>
          <div>
            <strong>Workspaces</strong>
            <span>{{ workspaces.length }}</span>
          </div>

          <button
            *ngIf="!edit"
            type="button"
            class="new"
            title="New workspace (Ctrl+Alt+Shift+N)"
            (click)="createWorkspace()">
            + New
          </button>
        </header>

        <div class="workspace-list" role="listbox" aria-label="Workspaces">
          <div
            *ngIf="edit?.kind === 'create'"
            class="workspace workspace-edit-row selected"
            role="option"
            aria-selected="true"
            aria-label="New workspace">
            <span class="slot-placeholder">+</span>

            <span class="workspace-main">
              <input
                #nameInput
                class="workspace-name-input"
                type="text"
                [(ngModel)]="edit.draft"
                [ngModelOptions]="{ standalone: true }"
                autocomplete="off"
                spellcheck="false"
                aria-label="New workspace name"
                data-dialog-initial-focus
                (click)="$event.stopPropagation()"
                (mousedown)="$event.stopPropagation()"
                (keydown.enter)="commitEdit(); $event.preventDefault(); $event.stopPropagation()"
                (keydown.escape)="cancelEdit(); $event.preventDefault(); $event.stopPropagation()" />

              <small class="edit-hint">Enter to create · Esc to cancel</small>
            </span>
          </div>

          <div
            *ngFor="let workspace of workspaces; let i = index; trackBy: trackWorkspace"
            class="workspace"
            role="option"
            [attr.tabindex]="i === selectedIndex && !edit ? 0 : -1"
            [class.selected]="i === selectedIndex"
            [class.active]="workspace.id === activeWorkspaceId"
            [attr.aria-selected]="i === selectedIndex"
            [attr.aria-posinset]="i + 1"
            [attr.aria-setsize]="workspaces.length"
            (mouseenter)="!edit && (selectedIndex = i)"
            (click)="!edit && activateWorkspace(workspace)">

            <span class="workspace-index">
              <kbd *ngIf="i < 9">{{ i + 1 }}</kbd>
              <span class="workspace-icon">{{ workspace.icon || '📁' }}</span>
            </span>

            <span class="details">
              <ng-container *ngIf="edit?.kind !== 'rename' || edit.workspaceId !== workspace.id; else renameWorkspaceInput">
                <strong>{{ workspace.name }}</strong>
              </ng-container>

              <ng-template #renameWorkspaceInput>
                <input
                  #nameInput
                  class="workspace-name-input"
                  type="text"
                  [(ngModel)]="edit!.draft"
                  [ngModelOptions]="{ standalone: true }"
                  autocomplete="off"
                  spellcheck="false"
                  aria-label="Workspace name"
                  data-dialog-initial-focus
                  (click)="$event.stopPropagation()"
                  (mousedown)="$event.stopPropagation()"
                  (keydown.enter)="commitEdit(); $event.preventDefault(); $event.stopPropagation()"
                  (keydown.escape)="cancelEdit(); $event.preventDefault(); $event.stopPropagation()" />
                <small class="edit-hint">Enter to save · Esc to cancel</small>
              </ng-template>

              <small *ngIf="edit?.kind !== 'rename' || edit.workspaceId !== workspace.id">
                {{ workspaceSummary(workspace) }} ·
                {{ workspaceKind(workspace) }}
              </small>
            </span>

            <span class="active-mark" *ngIf="workspace.id === activeWorkspaceId">
              Active
            </span>
          </div>
        </div>

        <footer>
          <ng-container *ngIf="!edit">
            <button
              type="button"
              [disabled]="!selectedWorkspace"
              (click)="renameSelected()">
              F2&nbsp;&nbsp;Rename
            </button>

            <button
              type="button"
              class="danger"
              [disabled]="!canDeleteSelected()"
              (click)="deleteSelected()">
              Del&nbsp;&nbsp;Delete
            </button>
          </ng-container>

          <span class="keys">
            {{ edit ? 'Enter to save · Esc to cancel' : '1–9 switch · ↑ ↓ select · Enter open · Esc close' }}
          </span>
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
      background: var(--vsc-scrim-soft);
      pointer-events: auto;
    }

    .switcher {
      box-sizing: border-box;
      width: min(500px, calc(100vw - 24px));
      max-height: min(620px, calc(100vh - 40px));
      display: flex;
      flex-direction: column;
      overflow: hidden;
      border: 1px solid var(--vsc-border, var(--vsc-border));
      border-radius: 8px;
      background: var(--vsc-menu-background, var(--vsc-panel-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
      box-shadow: 0 18px 60px var(--vsc-scrim);
      font: 12px/1.4 var(--vsc-font-family, system-ui, sans-serif);
    }

    header,
    footer {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 10px;
    }

    header {
      justify-content: space-between;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
    }

    header > div {
      display: flex;
      align-items: baseline;
      gap: 8px;
    }

    header span,
    .keys,
    small {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
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
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
      border-color: var(--vsc-border, var(--vsc-border));
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
      padding: 4px;
    }

    .workspace {
      display: grid;
      grid-template-columns: 40px minmax(0, 1fr) auto;
      gap: 10px;
      align-items: center;
      width: 100%;
      min-height: 44px;
      padding: 6px 9px;
      text-align: left;
      border: 1px solid transparent;
      border-radius: 4px;
      cursor: pointer;
    }

    .workspace:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .workspace-edit-row:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .workspace.selected {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .workspace.active {
      box-shadow: inset 2px 0 0 var(--vsc-accent-blue, var(--vsc-accent-blue));
    }

    .workspace-index {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 5px;
      min-width: 0;
    }

    .workspace-index kbd {
      display: inline-grid;
      place-items: center;
      min-width: 20px;
      height: 20px;
      padding: 0 3px;
      border: 1px solid var(--vsc-border, var(--vsc-border));
      border-radius: 3px;
      background: var(--vsc-editor-background, var(--vsc-editor-background));
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font: 10px var(--vsc-font-family-mono, ui-monospace, monospace);
    }

    .workspace-icon {
      font-size: 16px;
      line-height: 1;
    }

    .slot-placeholder {
      display: grid;
      place-items: center;
      width: 24px;
      height: 24px;
      margin: 0 auto;
      border-radius: 50%;
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 18px;
      line-height: 1;
    }

    .workspace-main {
      min-width: 0;
      display: grid;
      gap: 3px;
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

    .workspace-edit-row {
      margin-bottom: 3px;
      border-color: var(--vsc-border-subtle, var(--vsc-border-subtle));
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
      cursor: default;
    }

    .workspace-name-input {
      box-sizing: border-box;
      width: 100%;
      min-width: 0;
      height: 28px;
      padding: 3px 7px;
      border: 1px solid var(--vsc-focus-border, var(--vsc-accent-blue));
      border-radius: 3px;
      outline: none;
      background: var(--vsc-input-background, var(--vsc-popover-background));
      color: var(--vsc-input-foreground, var(--vsc-foreground, var(--vsc-foreground)));
      font: inherit;
    }

    .workspace-name-input:focus {
      outline: none;
    }

    .edit-hint {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 10px;
      line-height: 1.2;
    }

    .active-mark {
      padding: 2px 6px;
      border-radius: 10px;
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 16%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      font-size: 9px;
      font-weight: 600;
      letter-spacing: .03em;
      text-transform: uppercase;
    }

    footer {
      min-height: 38px;
      border-top: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
    }

    footer .danger:not(:disabled) {
      color: var(--vsc-error-foreground, var(--vsc-error));
    }

    .keys {
      margin-left: auto;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 9px;
      white-space: nowrap;
    }
  `],
})
export class ReassemblyWorkspaceSwitcherComponent
  implements OnInit, OnDestroy {

  @Output() workspaceActivated = new EventEmitter<void>();
  @Output() toolWorkspaceActivated =
    new EventEmitter<WorkspacePresentation>();
  @ViewChild('nameInput') private nameInput?: ElementRef<HTMLInputElement>;

  visible = false;
  selectedIndex = 0;
  edit: WorkspaceEdit = null;
  private editGeneration = 0;

  constructor(
    private readonly workspacesService: WorkspaceService,
    private readonly workspaceSurfaces: WorkspaceSurfaceService,
    private readonly confirmDialog: ConfirmDialogService,
    private readonly input: InputEngineService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get workspaces(): WorkspaceSwitcherItem[] {
    const files = (
      this.workspacesService.getWorkspaces().workspaces ?? []
    ).map<WorkspaceSwitcherItem>(workspace => ({
      id: workspace.id,
      name: workspace.name,
      icon: workspace.icon || '📁',
      kind: 'files',
      files: workspace,
    }));

    const tools = this.workspaceSurfaces
      .listToolWorkspaces()
      .map<WorkspaceSwitcherItem>(presentation => ({
        id: presentation.workspaceId,
        name: presentation.title,
        icon: '◫',
        kind: 'tool',
        tool: presentation,
      }));

    return [...files, ...tools];
  }

  get activeWorkspaceId(): string {
    const activePresentation = this.workspaceSurfaces.active;
    if (activePresentation?.kind === 'tool') {
      return activePresentation.workspaceId;
    }

    return this.workspacesService.getActiveWorkspace()?.id ?? '';
  }

  get selectedWorkspace(): WorkspaceSwitcherItem | null {
    return this.workspaces[this.selectedIndex] ?? null;
  }

  workspaceKind(workspace: WorkspaceSwitcherItem): string {
    return workspace.kind;
  }

  workspaceSummary(workspace: WorkspaceSwitcherItem): string {
    if (workspace.kind === 'tool') {
      return workspace.tool?.lifetime === 'persistent'
        ? 'full workspace · pinned'
        : 'full workspace · temporary';
    }

    const files = workspace.files;
    return files
      ? `${files.leftPane.tabs.length} left · ${files.rightPane.tabs.length} right`
      : 'files';
  }

  canDeleteSelected(): boolean {
    const selected = this.selectedWorkspace;
    if (!selected) return false;
    if (selected.kind === 'tool') return true;

    const fileCount = this.workspaces.filter(
      workspace => workspace.kind === 'files',
    ).length;
    return fileCount > 1;
  }

  ngOnInit(): void {
    this.input.registerContext({
      id: CONTEXT_ID,
      kind: 'overlay',
      priority: 400, exclusive: true,
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
        if (this.edit) return;
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
    this.input.registerCommandDescriptor({
      id: COMMAND.rename,
      title: 'Rename Workspace',
      category: 'Workspace',
      execute: () => this.renameSelected(),
      isEnabled: () => !this.edit && !!this.selectedWorkspace,
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.remove,
      title: 'Delete Workspace',
      category: 'Workspace',
      execute: () => this.deleteSelected(),
      isEnabled: () => !this.edit && this.canDeleteSelected(),
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.slot1,
      title: 'Activate Workspace 1',
      category: 'Workspace',
      execute: () => this.activateSlot(0),
      isEnabled: () => this.workspaces.length > 0,
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.slot2,
      title: 'Activate Workspace 2',
      category: 'Workspace',
      execute: () => this.activateSlot(1),
      isEnabled: () => this.workspaces.length > 1,
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.slot3,
      title: 'Activate Workspace 3',
      category: 'Workspace',
      execute: () => this.activateSlot(2),
      isEnabled: () => this.workspaces.length > 2,
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.slot4,
      title: 'Activate Workspace 4',
      category: 'Workspace',
      execute: () => this.activateSlot(3),
      isEnabled: () => this.workspaces.length > 3,
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.slot5,
      title: 'Activate Workspace 5',
      category: 'Workspace',
      execute: () => this.activateSlot(4),
      isEnabled: () => this.workspaces.length > 4,
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.slot6,
      title: 'Activate Workspace 6',
      category: 'Workspace',
      execute: () => this.activateSlot(5),
      isEnabled: () => this.workspaces.length > 5,
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.slot7,
      title: 'Activate Workspace 7',
      category: 'Workspace',
      execute: () => this.activateSlot(6),
      isEnabled: () => this.workspaces.length > 6,
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.slot8,
      title: 'Activate Workspace 8',
      category: 'Workspace',
      execute: () => this.activateSlot(7),
      isEnabled: () => this.workspaces.length > 7,
    }, OWNER_ID);

    this.input.registerCommandDescriptor({
      id: COMMAND.slot9,
      title: 'Activate Workspace 9',
      category: 'Workspace',
      execute: () => this.activateSlot(8),
      isEnabled: () => this.workspaces.length > 8,
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

    this.cancelEdit(false);
    this.visible = false;
    this.input.deactivateContext(CONTEXT_ID);
    
    
    this.cdr.markForCheck();
  }

  createWorkspace(): void {
    if (!this.visible) {
      this.open();
    }

    this.edit = {
      kind: 'create',
      draft: 'New Workspace',
    };
    this.editGeneration += 1;
    this.focusNameInput(this.editGeneration);
    this.cdr.markForCheck();
  }

  activateWorkspace(workspace: WorkspaceSwitcherItem): void {
    if (workspace.kind === 'tool') {
      const presentation = workspace.tool;
      if (
        !presentation ||
        !this.workspaceSurfaces.activate(presentation.workspaceId)
      ) {
        return;
      }

      this.toolWorkspaceActivated.emit(
        this.workspaceSurfaces.get(presentation.workspaceId) ??
          presentation,
      );
      this.close();
      return;
    }

    if (
      !workspace.files ||
      !this.workspacesService.switchWorkspace(workspace.files.id)
    ) {
      return;
    }

    this.workspaceActivated.emit();
    this.close();
  }

  renameSelected(): void {
    const workspace = this.selectedWorkspace;
    if (!workspace) return;

    this.edit = {
      kind: 'rename',
      workspaceId: workspace.id,
      draft: workspace.name,
    };
    this.editGeneration += 1;
    this.focusNameInput(this.editGeneration);
    this.cdr.markForCheck();
  }

  async commitEdit(): Promise<void> {
    const edit = this.edit;
    if (!edit) return;

    const name = edit.draft.trim();
    if (!name) {
      this.focusNameInput(this.editGeneration);
      return;
    }

    if (edit.kind === 'create') {
      const workspace = await this.workspacesService.createWorkspace(name);

      // The edit may have been cancelled while home-path resolution was pending.
      if (this.edit !== edit) return;

      this.edit = null;
      this.workspacesService.switchWorkspace(workspace.id);
      this.selectedIndex = Math.max(
        0,
        this.workspaces.findIndex(item => item.id === workspace.id),
      );
      this.workspaceActivated.emit();
      this.cdr.markForCheck();
      return;
    }

    const workspace = this.workspaces.find(
      item => item.id === edit.workspaceId,
    );
    if (!workspace) {
      this.cancelEdit();
      return;
    }

    if (name !== workspace.name) {
      if (workspace.kind === 'tool') {
        this.workspaceSurfaces.rename(workspace.id, name);
      } else if (workspace.files) {
        this.workspacesService.updateWorkspace(
          workspace.files.id,
          { name },
        );
      }
    }

    this.edit = null;
    this.cdr.markForCheck();
  }

  cancelEdit(markForCheck = true): void {
    if (!this.edit) return;

    this.edit = null;
    this.editGeneration += 1;

    if (markForCheck) {
      this.cdr.markForCheck();
    }
  }

  private focusNameInput(generation: number, attempt = 0): void {
    setTimeout(() => {
      if (!this.edit || generation !== this.editGeneration) return;

      const input = this.nameInput?.nativeElement;
      if (!input) {
        if (attempt < 5) {
          this.focusNameInput(generation, attempt + 1);
        }
        return;
      }

      input.focus({ preventScroll: true });
      input.select();
    }, attempt === 0 ? 0 : 16);
  }

  async deleteSelected(): Promise<void> {
    const workspace = this.selectedWorkspace;
    if (!workspace || !this.canDeleteSelected()) return;

    const confirmed = await this.confirmDialog.confirm({
      title: 'Delete Workspace',
      message: `Delete workspace "${workspace.name}"?`,
      confirmText: 'Delete',
      danger: true,
    });

    if (!confirmed) return;

    const wasActive = workspace.id === this.activeWorkspaceId;

    if (workspace.kind === 'tool') {
      const presentation = workspace.tool;
      if (!presentation) return;

      const removed =
        presentation.lifetime === 'persistent'
          ? this.workspaceSurfaces.removePersistent(workspace.id)
          : this.workspaceSurfaces.closeTransient(workspace.id);

      if (!removed) return;

      if (wasActive) {
        const files = this.workspacesService.getActiveWorkspace();
        if (files) {
          this.workspaceSurfaces.activateFilesWorkspace(files);
        }
        this.workspaceActivated.emit();
      }
    } else {
      if (
        !workspace.files ||
        !this.workspacesService.deleteWorkspace(workspace.files.id)
      ) {
        return;
      }

      if (wasActive) {
        this.workspaceActivated.emit();
      }
    }

    this.selectedIndex = Math.min(
      this.selectedIndex,
      Math.max(0, this.workspaces.length - 1),
    );

    this.cdr.markForCheck();
  }

  trackWorkspace(
    _index: number,
    workspace: WorkspaceSwitcherItem,
  ): string {
    return workspace.id;
  }

  private activateSlot(index: number): void {
    if (this.edit) return;

    const workspace = this.workspaces[index];
    if (workspace) this.activateWorkspace(workspace);
  }

  private moveSelection(delta: -1 | 1): void {
    if (this.edit) return;

    const count = this.workspaces.length;
    if (count === 0) return;

    this.selectedIndex =
      (this.selectedIndex + delta + count) % count;
    this.cdr.markForCheck();
  }
}