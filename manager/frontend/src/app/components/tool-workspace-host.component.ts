import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  OnChanges,
  OnDestroy,
  OnInit,
  Output,
  QueryList,
  SimpleChanges,
  ViewChildren,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { PaneViewHostComponent } from './pane-view-host.component';
import type { PaneDetailsColumn } from './pane-details.component';
import { FileType, type FileInfo } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';

import type {
  WorkspaceLayout,
  WorkspacePresentation,
  WorkspaceSurface,
  WorkspaceSurfaceAction,
} from '../services/workspace-surface.model';

type DetailsWorkspacePane = 'tree' | 'details';
type DetailsWorkspaceViewPresetId = 'security' | 'timestamps' | 'filesystem';

const DETAILS_WORKSPACE_VIEW_PRESETS: ReadonlyArray<{ id: DetailsWorkspaceViewPresetId; label: string }> = [
  { id: 'security', label: 'Security' },
  { id: 'timestamps', label: 'Timestamps' },
  { id: 'filesystem', label: 'Filesystem' },
];

function isDetailsWorkspaceViewPresetId(value: string): value is DetailsWorkspaceViewPresetId {
  return DETAILS_WORKSPACE_VIEW_PRESETS.some(preset => preset.id === value);
}

const DETAILS_WORKSPACE_INPUT_CONTEXT = 'reassembly.workspace.details';
const DETAILS_WORKSPACE_INPUT_OWNER = 'reassembly.workspace.details';
const DETAILS_WORKSPACE_COMMANDS = {
  nextPane: 'reassembly.workspace.details.nextPane',
  previousPane: 'reassembly.workspace.details.previousPane',
  cursorUp: 'reassembly.workspace.details.cursorUp',
  cursorDown: 'reassembly.workspace.details.cursorDown',
  firstItem: 'reassembly.workspace.details.firstItem',
  lastItem: 'reassembly.workspace.details.lastItem',
  pageUp: 'reassembly.workspace.details.pageUp',
  pageDown: 'reassembly.workspace.details.pageDown',
  open: 'reassembly.workspace.details.open',
  parent: 'reassembly.workspace.details.parent',
} as const;

const DETAILS_WORKSPACE_INPUT_BINDINGS: readonly InputBindingDefinition[] = [
  { id: 'reassembly.workspace.details.nextPane.tab', commandId: DETAILS_WORKSPACE_COMMANDS.nextPane, contextId: DETAILS_WORKSPACE_INPUT_CONTEXT, source: 'workspace', description: 'Focus next Details workspace pane', category: 'Workspace', trigger: { device: 'keyboard', chord: { steps: [{ key: 'tab' }] } }, preventDefault: true, stopPropagation: true },
  { id: 'reassembly.workspace.details.previousPane.shiftTab', commandId: DETAILS_WORKSPACE_COMMANDS.previousPane, contextId: DETAILS_WORKSPACE_INPUT_CONTEXT, source: 'workspace', description: 'Focus previous Details workspace pane', category: 'Workspace', trigger: { device: 'keyboard', chord: { steps: [{ key: 'tab', modifiers: ['shift'] }] } }, preventDefault: true, stopPropagation: true },
  { id: 'reassembly.workspace.details.cursorUp', commandId: DETAILS_WORKSPACE_COMMANDS.cursorUp, contextId: DETAILS_WORKSPACE_INPUT_CONTEXT, source: 'workspace', description: 'Move Details workspace cursor up', category: 'Workspace', trigger: { device: 'keyboard', chord: { steps: [{ key: 'arrowup' }] } }, preventDefault: true, stopPropagation: true },
  { id: 'reassembly.workspace.details.cursorDown', commandId: DETAILS_WORKSPACE_COMMANDS.cursorDown, contextId: DETAILS_WORKSPACE_INPUT_CONTEXT, source: 'workspace', description: 'Move Details workspace cursor down', category: 'Workspace', trigger: { device: 'keyboard', chord: { steps: [{ key: 'arrowdown' }] } }, preventDefault: true, stopPropagation: true },
  { id: 'reassembly.workspace.details.firstItem', commandId: DETAILS_WORKSPACE_COMMANDS.firstItem, contextId: DETAILS_WORKSPACE_INPUT_CONTEXT, source: 'workspace', description: 'Move to first Details workspace item', category: 'Workspace', trigger: { device: 'keyboard', chord: { steps: [{ key: 'home' }] } }, preventDefault: true, stopPropagation: true },
  { id: 'reassembly.workspace.details.lastItem', commandId: DETAILS_WORKSPACE_COMMANDS.lastItem, contextId: DETAILS_WORKSPACE_INPUT_CONTEXT, source: 'workspace', description: 'Move to last Details workspace item', category: 'Workspace', trigger: { device: 'keyboard', chord: { steps: [{ key: 'end' }] } }, preventDefault: true, stopPropagation: true },
  { id: 'reassembly.workspace.details.pageUp', commandId: DETAILS_WORKSPACE_COMMANDS.pageUp, contextId: DETAILS_WORKSPACE_INPUT_CONTEXT, source: 'workspace', description: 'Move one Details workspace page up', category: 'Workspace', trigger: { device: 'keyboard', chord: { steps: [{ key: 'pageup' }] } }, preventDefault: true, stopPropagation: true },
  { id: 'reassembly.workspace.details.pageDown', commandId: DETAILS_WORKSPACE_COMMANDS.pageDown, contextId: DETAILS_WORKSPACE_INPUT_CONTEXT, source: 'workspace', description: 'Move one Details workspace page down', category: 'Workspace', trigger: { device: 'keyboard', chord: { steps: [{ key: 'pagedown' }] } }, preventDefault: true, stopPropagation: true },
  { id: 'reassembly.workspace.details.open', commandId: DETAILS_WORKSPACE_COMMANDS.open, contextId: DETAILS_WORKSPACE_INPUT_CONTEXT, source: 'workspace', description: 'Open Details workspace item', category: 'Workspace', trigger: { device: 'keyboard', chord: { steps: [{ key: 'enter' }] } }, preventDefault: true, stopPropagation: true },
  { id: 'reassembly.workspace.details.parent', commandId: DETAILS_WORKSPACE_COMMANDS.parent, contextId: DETAILS_WORKSPACE_INPUT_CONTEXT, source: 'workspace', description: 'Inspect parent folder in Details workspace', category: 'Workspace', when: '!typing', trigger: { device: 'keyboard', chord: { steps: [{ key: 'backspace' }] } }, preventDefault: true, stopPropagation: true },
];

@Component({
  selector: 'app-tool-workspace-host',
  standalone: true,
  imports: [CommonModule, PaneViewHostComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section
      class="tool-workspace"
      role="region"
      [attr.aria-label]="presentation.title"
      (keydown.tab)="onDetailsWorkspaceTab($event)">

      <header class="workspace-header">
        <div class="workspace-title">
          <span class="eyebrow">Workspace</span>
          <strong>{{ presentation.title }}</strong>
          <span class="lifetime">
            {{ presentation.lifetime === 'transient' ? 'Temporary' : 'Pinned' }}
          </span>
        </div>

        <div class="workspace-actions">
          <button
            *ngIf="presentation.lifetime === 'transient'"
            type="button"
            (click)="pin.emit()">
            Pin Workspace
          </button>
          <button type="button" (click)="close.emit()">Close</button>
        </div>
      </header>

      <div class="workspace-body">
        <ng-container
          *ngTemplateOutlet="renderLayout; context: { $implicit: presentation.layout }">
        </ng-container>
      </div>
    </section>

    <ng-template #renderLayout let-layout>
      <div
        *ngIf="layout.kind === 'split'; else nonSplit"
        class="layout split"
        [class.details-workspace-split]="isDetailsWorkspaceSplit(layout)"
        [class.horizontal]="layout.direction === 'horizontal'"
        [class.vertical]="layout.direction === 'vertical'"
        [style.--workspace-ratio.%]="layout.ratio ?? 50">

        <div class="split-first">
          <ng-container
            *ngTemplateOutlet="renderLayout; context: { $implicit: layout.first }">
          </ng-container>
        </div>

        <div class="split-second">
          <ng-container
            *ngTemplateOutlet="renderLayout; context: { $implicit: layout.second }">
          </ng-container>
        </div>
      </div>

      <ng-template #nonSplit>
        <div *ngIf="layout.kind === 'stack'; else leaf" class="layout stack">
          <article
            *ngFor="let surface of layout.surfaces; trackBy: trackSurface"
            class="stack-surface"
            [class.active]="surface.id === layout.activeSurfaceId">
            <ng-container
              *ngTemplateOutlet="renderSurface; context: { $implicit: surface }">
            </ng-container>
          </article>
        </div>

        <ng-template #leaf>
          <ng-container
            *ngTemplateOutlet="renderSurface; context: { $implicit: layout.surface }">
          </ng-container>
        </ng-template>
      </ng-template>
    </ng-template>

    <ng-template #renderSurface let-surface>
      <article class="surface" [attr.data-surface-id]="surface.surfaceId">
        <header class="surface-header">
          <strong>{{ text(surface, 'title') || surface.title }}</strong>

          <select
            *ngIf="surface.surfaceId === 'details-workspace'"
            class="details-view-select"
            aria-label="Details view"
            title="Details view"
            [value]="detailsWorkspaceViewPreset"
            (change)="setDetailsWorkspaceViewPreset($any($event.target).value)">
            <option *ngFor="let preset of detailsWorkspaceViewPresets" [value]="preset.id">
              {{ preset.label }}
            </option>
          </select>

          <span class="surface-actions">
            <span
              *ngIf="text(surface, 'busy') === 'true'"
              class="live-state">
              working…
            </span>

            <button
              *ngFor="let action of surface.actions || []; trackBy: trackAction"
              type="button"
              [class.primary]="action.intent === 'primary'"
              [class.danger]="action.intent === 'danger'"
              [disabled]="
                text(surface, 'busy') === 'true' ||
                (action.requiresSelection && !selected(surface))
              "
              (click)="emitAction(surface, action)">
              {{ action.title }}
            </button>

          </span>
        </header>

        <div
          *ngIf="text(surface, 'error')"
          class="surface-error"
          role="alert">
          {{ text(surface, 'error') }}
        </div>

        <ng-container [ngSwitch]="surface.surfaceId">
          <section *ngSwitchCase="'search-workspace'" class="search-surface">
            <div class="surface-meta">
              <span>{{ text(surface, 'query') }}</span>
              <span>{{ text(surface, 'rootPath') }}</span>
              <span *ngIf="text(surface, 'refreshedAt')">
                refreshed {{ text(surface, 'refreshedAt') }}
              </span>
            </div>

            <div class="result-table">
              <div class="result-row header-row">
                <span>Name</span>
                <span>Path</span>
                <span>Size</span>
              </div>
              <div
                class="result-row selectable"
                *ngFor="let entry of array(surface, 'entries'); trackBy: trackEntry"
                [class.selected]="isSelected(surface, entry)"
                (click)="select(surface, entry)"
                (dblclick)="emitNamedAction(surface, 'open', entry)">
                <span class="name">{{ entry.name }}</span>
                <span class="path" [title]="entry.path">{{ entry.path }}</span>
                <span class="size">{{ formatSize(entry.size) }}</span>
              </div>
            </div>
          </section>

          <section *ngSwitchCase="'compare-workspace'" class="compare-surface">
            <div class="compare-paths">
              <span [title]="text(surface, 'leftPath')">
                {{ text(surface, 'leftPath') }}
              </span>
              <span>↔</span>
              <span [title]="text(surface, 'rightPath')">
                {{ text(surface, 'rightPath') }}
              </span>
              <span *ngIf="text(surface, 'refreshedAt')">
                {{ text(surface, 'refreshedAt') }}
              </span>
            </div>

            <div class="compare-table">
              <div class="compare-row header-row">
                <span>Status</span>
                <span>Item</span>
                <span>Left</span>
                <span>Right</span>
              </div>
              <div
                class="compare-row selectable"
                *ngFor="let row of array(surface, 'rows'); trackBy: trackCompareRow"
                [class.selected]="isSelected(surface, row)"
                (click)="select(surface, row)">
                <span class="status">{{ row.status }}</span>
                <span>{{ row.key }}</span>
                <span class="path" [title]="row.leftPath">{{ row.leftPath || '—' }}</span>
                <span class="path" [title]="row.rightPath">{{ row.rightPath || '—' }}</span>
              </div>
            </div>
          </section>

          <section *ngSwitchCase="'compare-summary'" class="summary-surface">
            <div
              class="summary-card"
              *ngFor="let item of array(surface, 'items'); trackBy: trackSummary">
              <strong>{{ item.value }}</strong>
              <span>{{ item.label }}</span>
            </div>
          </section>

          <section
            *ngSwitchCase="'details-tree'"
            class="details-tree-surface tree-view"
            data-details-workspace-pane="tree"
            [class.workspace-pane-active]="detailsWorkspacePane === 'tree'"
            [attr.tabindex]="detailsWorkspacePane === 'tree' ? 0 : -1"
            (focusin)="activateDetailsWorkspacePane('tree')"
            (mousedown)="activateDetailsWorkspacePane('tree')">
            <app-pane-view-host
              [surfaceId]="surface.id"
              [surfaceActive]="true"
              [paneId]="'left'"
              [showHidden]="showHidden"
              [entries]="treeEntries(surface)"
              [currentPath]="text(surface, 'currentPath') || text(surface, 'selectedPath') || text(surface, 'rootPath')"
              [selectedFiles]="treeSelectedPathSet(surface)"
              [cursorPath]="treeCursorPath(surface)"
              [currentView]="'tree'"
              [treeContentMode]="'folders'"
              [paneActive]="detailsWorkspacePane === 'tree'"
              (selectionChange)="onTreeSelectionChange(surface, $event)"
              (navigate)="emitNamedAction(surface, 'select-folder', $event)">
            </app-pane-view-host>
          </section>

          <section
            *ngSwitchCase="'details-workspace'"
            class="details-surface details-inspector list view-details"
            data-details-workspace-pane="details"
            [class.workspace-pane-active]="detailsWorkspacePane === 'details'"
            [attr.tabindex]="detailsWorkspacePane === 'details' ? 0 : -1"
            (focusin)="activateDetailsWorkspacePane('details')"
            (mousedown)="activateDetailsWorkspacePane('details')"
            (keydown.backspace)="onDetailsWorkspaceBackspace($event)">
            <app-pane-view-host
              [surfaceId]="surface.id"
              [surfaceActive]="true"
              [paneId]="'right'"
              [showHidden]="showHidden"
              [entries]="detailsWorkspaceEntries(surface)"
              [currentPath]="text(surface, 'path')"
              [selectedFiles]="selectedPathSet(surface)"
              [cursorPath]="selectedCursorPath(surface)"
              [error]="text(surface, 'error') || null"
              [currentView]="'details'"
              [sortField]="text(surface, 'sortField') || 'name'"
              [sortDirection]="sortDirection(surface)"
              [detailsColumns]="detailsWorkspaceColumns"
              [paneActive]="detailsWorkspacePane === 'details'"
              (selectionChange)="onDetailsSelectionChange(surface, $event)"
              (navigate)="emitNamedAction(surface, 'open', $event)">
            </app-pane-view-host>
          </section>

          <section *ngSwitchDefault class="unknown-surface">
            <strong>{{ surface.surfaceId }}</strong>
            <p>This surface is registered with the workspace but has no built-in full renderer yet.</p>
          </section>
        </ng-container>
      </article>
    </ng-template>
  `,
  styles: [`
    :host {
      display: block;
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
    }

    .tool-workspace {
      display: flex;
      flex-direction: column;
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-editor-background, var(--vsc-editor-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
    }

    .workspace-header,
    .surface-header,
    .surface-meta,
    .compare-paths {
      display: flex;
      align-items: center;
      gap: 10px;
    }

    .workspace-header {
      flex: 0 0 auto;
      justify-content: space-between;
      min-height: 44px;
      padding: 6px 10px;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-control-background));
      background: var(--vsc-side-bar-background, var(--vsc-panel-background));
    }

    .workspace-title {
      display: flex;
      align-items: baseline;
      min-width: 0;
      gap: 8px;
    }

    .eyebrow,
    .lifetime,
    .surface-header span,
    .surface-meta,
    .compare-paths {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
    }

    .workspace-actions {
      display: flex;
      gap: 6px;
    }

    .surface-actions {
      display: inline-flex;
      align-items: center;
      gap: 6px;
    }

    .surface-actions button {
      min-height: 24px;
      padding: 2px 7px;
    }

    .details-view-select {
      height: 24px;
      min-width: 104px;
      margin-left: auto;
      padding: 0 24px 0 7px;
      border: 1px solid var(--vsc-dropdown-border, var(--vsc-border));
      border-radius: 3px;
      background: var(--vsc-dropdown-background, var(--vsc-menu-background, var(--vsc-panel-background)));
      color: var(--vsc-dropdown-foreground, var(--vsc-foreground));
      font: inherit;
      font-size: 11px;
      cursor: pointer;
    }

    .details-view-select:focus-visible,
    [data-details-workspace-pane]:focus-visible {
      outline: 1px solid var(--vsc-focus-border);
      outline-offset: -1px;
    }

    [data-details-workspace-pane] {
      min-width: 0;
      min-height: 0;
    }

    [data-details-workspace-pane].workspace-pane-active {
      border-color: var(--vsc-focus-border, var(--vsc-accent-blue));
    }

    .live-state {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 10px;
    }

    .surface-error {
      flex: 0 0 auto;
      padding: 6px 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      color: var(--vsc-error-foreground, var(--vsc-error));
      background: color-mix(in srgb, var(--vsc-error-foreground, var(--vsc-error)) 8%, transparent);
      font-size: 11px;
    }

    button {
      min-height: 28px;
      padding: 3px 9px;
      border: 1px solid var(--vsc-border, var(--vsc-border));
      border-radius: 4px;
      background: var(--vsc-button-secondary-background, var(--vsc-control-background));
      color: inherit;
      font: inherit;
      cursor: pointer;
    }

    .workspace-body {
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      padding: 0;
      box-sizing: border-box;
    }

    .layout,
    .surface {
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
      box-sizing: border-box;
    }

    .split {
      display: grid;
      gap: 8px;
    }

    .split.details-workspace-split {
      gap: 5px;
      padding: 5px;
      box-sizing: border-box;
    }

    .split.horizontal {
      grid-template-columns:
        minmax(0, calc(var(--workspace-ratio) * 1%))
        minmax(0, 1fr);
    }

    .split.vertical {
      grid-template-rows:
        minmax(0, calc(var(--workspace-ratio) * 1%))
        minmax(0, 1fr);
    }

    .split-first,
    .split-second {
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      box-sizing: border-box;
    }

    .split.details-workspace-split.horizontal {
      grid-template-columns: minmax(220px, 25%) minmax(0, 1fr);
    }

    .stack {
      display: grid;
      grid-template-columns: 1fr;
      gap: 8px;
    }

    .stack-surface:not(.active) {
      display: none;
    }

    .surface {
      display: flex;
      flex-direction: column;
      overflow: hidden;
      border: 1px solid var(--vsc-border, var(--vsc-control-background));
      border-radius: 5px;
      background: var(--vsc-editor-background, var(--vsc-editor-background));
    }

    .surface-header {
      flex: 0 0 auto;
      justify-content: space-between;
      min-height: 32px;
      padding: 5px 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
      background: var(--vsc-side-bar-background, var(--vsc-panel-background));
    }

    .search-surface,
    .compare-surface,
    .details-tree-surface,
    .details-surface {
      flex: 1 1 auto;
      min-height: 0;
      overflow: auto;
      overscroll-behavior: none;
    }

    .surface-meta,
    .compare-paths {
      padding: 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border-subtle));
    }

    .surface-meta span,
    .compare-paths span {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .result-table,
    .compare-table {
      min-width: 650px;
    }

    .result-row,
    .compare-row {
      display: grid;
      align-items: center;
      min-height: 28px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-control-background));
    }

    .result-row {
      grid-template-columns: minmax(160px, .8fr) minmax(280px, 2fr) 100px;
    }

    .compare-row {
      grid-template-columns: 110px minmax(160px, .8fr) minmax(220px, 1.3fr) minmax(220px, 1.3fr);
    }

    .selectable {
      cursor: default;
    }

    .selectable:hover {
      background: var(--vsc-list-hover-background, var(--vsc-list-hover-background));
    }

    .selectable.selected {
      background: var(--vsc-list-active-selection-background, var(--vsc-list-selected-emphasis));
      color: var(--vsc-list-active-selection-foreground, var(--vsc-on-accent));
    }

    button.primary {
      border-color: var(--vsc-focus-border, var(--vsc-accent-blue));
    }

    button.danger {
      color: var(--vsc-error-foreground, var(--vsc-error));
    }

    .result-row > span,
    .compare-row > span {
      min-width: 0;
      padding: 4px 8px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .header-row {
      position: sticky;
      top: 0;
      z-index: 2;
      background: var(--vsc-side-bar-background, var(--vsc-panel-background));
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
    }

    .summary-surface {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
      gap: 8px;
      padding: 8px;
      overflow: auto;
      overscroll-behavior: none;
    }

    .summary-card {
      display: flex;
      flex-direction: column;
      justify-content: center;
      min-height: 72px;
      padding: 10px;
      border: 1px solid var(--vsc-border-subtle, var(--vsc-activitybar-background));
      border-radius: 4px;
    }

    .summary-card strong {
      font-size: 20px;
    }

    .summary-card span {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .details-tree-surface {
      display: flex;
      padding: 0;
      overflow: hidden;
    }

    .details-tree-surface app-pane-view-host {
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
    }

    .details-inspector {
      display: flex;
      padding: 0;
      overflow: hidden;
    }

    .details-inspector app-pane-view-host {
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
    }

    .unknown-surface {
      padding: 16px;
    }
  `],
})
export class ToolWorkspaceHostComponent implements OnInit, OnChanges, OnDestroy {
  @Input({ required: true }) presentation!: WorkspacePresentation;
  @Input() showHidden = false;
  @Output() close = new EventEmitter<void>();
  @Output() pin = new EventEmitter<void>();
  @Output() surfaceAction = new EventEmitter<{
    surface: WorkspaceSurface;
    action: string;
    payload?: unknown;
  }>();

  @ViewChildren(PaneViewHostComponent) private paneViewHosts!: QueryList<PaneViewHostComponent>;

  private readonly selectedBySurface = new Map<string, unknown>();
  private detailsWorkspaceInputRegistered = false;

  readonly detailsWorkspaceViewPresets = DETAILS_WORKSPACE_VIEW_PRESETS;

  private readonly detailsWorkspaceTabFallback = (event: KeyboardEvent): void => {
    if (event.key !== 'Tab' || event.defaultPrevented) return;
    if (!this.isDetailsWorkspaceSplit(this.presentation.layout)) return;

    const target = event.target instanceof Element ? event.target : null;
    if (!target || !this.host.nativeElement.contains(target)) return;
    if (!target.closest('[data-details-workspace-pane]')) return;

    event.preventDefault();
    event.stopPropagation();
    this.focusDetailsWorkspacePane(event.shiftKey ? -1 : 1);
  };
  detailsWorkspacePane: DetailsWorkspacePane = 'tree';

  detailsWorkspaceViewPreset: DetailsWorkspaceViewPresetId = 'security';

  private readonly detailsWorkspaceAllColumns: readonly PaneDetailsColumn[] = [
    { field: 'name', header: 'Name', flex: 2, sortable: true, minWidth: 140 },
    { field: 'owner', header: 'Owner', width: '170px', sortable: true, minWidth: 110, maxWidth: 320 },
    { field: 'permissions', header: 'Permissions', width: '150px', sortable: true, minWidth: 100, maxWidth: 260 },
    { field: 'attributes', header: 'Attributes', width: '100px', sortable: true, minWidth: 80, maxWidth: 180 },
    { field: 'created', header: 'Created', width: '150px', sortable: true, minWidth: 110, maxWidth: 260, formatter: value => this.formatDate(value) },
    { field: 'modified', header: 'Modified', width: '150px', sortable: true, minWidth: 110, maxWidth: 260, formatter: value => this.formatDate(value) },
    { field: 'accessed', header: 'Accessed', width: '150px', sortable: true, minWidth: 110, maxWidth: 260, formatter: value => this.formatDate(value) },
    { field: 'type', header: 'Type', width: '100px', sortable: true, minWidth: 80, maxWidth: 220 },
    { field: 'isHidden', header: 'Hidden', width: '72px', sortable: true, minWidth: 60, maxWidth: 100, formatter: value => value ? 'Yes' : '' },
    { field: 'isSymlink', header: 'Link', width: '64px', sortable: true, minWidth: 54, maxWidth: 100, formatter: value => value ? 'Yes' : '' },
  ];

  detailsWorkspaceColumns: PaneDetailsColumn[] = [];

  ngOnInit(): void {
    this.loadDetailsWorkspaceViewPreset();
    this.applyDetailsWorkspaceColumns();

    this.input.registerContext({
      id: DETAILS_WORKSPACE_INPUT_CONTEXT,
      kind: 'widget',
      priority: 180,
      parentId: 'reassembly.manager',
      description: 'Details workspace input context',
    });
    for (const binding of DETAILS_WORKSPACE_INPUT_BINDINGS) {
      this.input.registerBinding(binding, DETAILS_WORKSPACE_INPUT_OWNER);
    }
    this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.nextPane, () => { this.focusDetailsWorkspacePane(1); }, DETAILS_WORKSPACE_INPUT_OWNER);
    this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.previousPane, () => { this.focusDetailsWorkspacePane(-1); }, DETAILS_WORKSPACE_INPUT_OWNER);
    this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.cursorUp, () => { this.navigateDetailsWorkspace(-1); }, DETAILS_WORKSPACE_INPUT_OWNER);
    this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.cursorDown, () => { this.navigateDetailsWorkspace(1); }, DETAILS_WORKSPACE_INPUT_OWNER);
    this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.firstItem, () => { this.navigateDetailsWorkspace('first'); }, DETAILS_WORKSPACE_INPUT_OWNER);
    this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.lastItem, () => { this.navigateDetailsWorkspace('last'); }, DETAILS_WORKSPACE_INPUT_OWNER);
    this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.pageUp, () => { this.navigateDetailsWorkspace('page-up'); }, DETAILS_WORKSPACE_INPUT_OWNER);
    this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.pageDown, () => { this.navigateDetailsWorkspace('page-down'); }, DETAILS_WORKSPACE_INPUT_OWNER);
    this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.open, () => { this.openDetailsWorkspaceCursor(); }, DETAILS_WORKSPACE_INPUT_OWNER);
    this.input.registerCommand(DETAILS_WORKSPACE_COMMANDS.parent, () => {
      if (this.detailsWorkspacePane === 'details') this.navigateDetailsWorkspaceParent();
    }, DETAILS_WORKSPACE_INPUT_OWNER);
    this.detailsWorkspaceInputRegistered = true;
    this.syncDetailsWorkspaceInputContext();
    window.addEventListener('keydown', this.detailsWorkspaceTabFallback, { capture: true });
  }

  ngOnChanges(_changes: SimpleChanges): void {
    if (!this.detailsWorkspaceInputRegistered) return;
    this.syncDetailsWorkspaceInputContext();
  }

  ngOnDestroy(): void {
    window.removeEventListener('keydown', this.detailsWorkspaceTabFallback, { capture: true });
    this.input.deactivateContext(DETAILS_WORKSPACE_INPUT_CONTEXT);
    for (const binding of DETAILS_WORKSPACE_INPUT_BINDINGS) {
      this.input.unregisterBinding(binding.id, DETAILS_WORKSPACE_INPUT_OWNER);
    }
    for (const commandId of Object.values(DETAILS_WORKSPACE_COMMANDS)) {
      this.input.unregisterCommand(commandId, DETAILS_WORKSPACE_INPUT_OWNER);
    }
  }

  setDetailsWorkspaceViewPreset(value: string): void {
    if (!isDetailsWorkspaceViewPresetId(value) || value === this.detailsWorkspaceViewPreset) return;
    this.detailsWorkspaceViewPreset = value;
    this.applyDetailsWorkspaceColumns();
    try { localStorage.setItem('reassembly.details-workspace.view', value); } catch {}
    this.cdr.markForCheck();
  }

  activateDetailsWorkspacePane(pane: DetailsWorkspacePane): void {
    this.detailsWorkspacePane = pane;
  }

  onDetailsWorkspaceTab(event: Event): void {
    if (!(event instanceof KeyboardEvent)) return;
    if (!this.isDetailsWorkspaceSplit(this.presentation.layout)) return;
    event.preventDefault();
    event.stopPropagation();
    this.focusDetailsWorkspacePane(event.shiftKey ? -1 : 1);
  }

  onDetailsWorkspaceBackspace(event: Event): void {
    if (!(event instanceof KeyboardEvent)) return;
    if (!this.isDetailsWorkspaceSplit(this.presentation.layout)) return;

    const target = event.target as HTMLElement | null;
    const pane = target?.closest<HTMLElement>('[data-details-workspace-pane]');
    if (pane?.dataset['detailsWorkspacePane'] !== 'details') return;

    if (!this.navigateDetailsWorkspaceParent()) return;
    event.preventDefault();
    event.stopPropagation();
  }

  private navigateDetailsWorkspaceParent(): boolean {
    const detailsSurface = this.findSurfaceById(this.presentation.layout, 'details-workspace');
    const treeSurface = this.findSurfaceById(this.presentation.layout, 'details-tree');
    if (!detailsSurface || !treeSurface) return false;

    const currentPath = this.text(detailsSurface, 'path');
    if (!currentPath || this.isDetailsWorkspaceDriveRoot(currentPath)) return false;

    this.emitNamedAction(detailsSurface, 'open', {
      path: PARENT_ENTRY_PATH,
      name: '..',
      type: FileType.DIRECTORY,
    });
    return true;
  }

  switchDetailsWorkspacePane(): boolean {
    return this.focusDetailsWorkspacePane(1);
  }

  private focusDetailsWorkspacePane(direction: -1 | 1): boolean {
    if (!this.isDetailsWorkspaceSplit(this.presentation.layout)) return false;

    const active = document.activeElement as HTMLElement | null;
    const currentPane = active?.closest<HTMLElement>('[data-details-workspace-pane]')
      ?.dataset['detailsWorkspacePane'] as DetailsWorkspacePane | undefined;
    const current = currentPane ?? this.detailsWorkspacePane;
    const order: readonly DetailsWorkspacePane[] = ['tree', 'details'];
    const currentIndex = Math.max(0, order.indexOf(current));
    const nextIndex = (currentIndex + direction + order.length) % order.length;
    const next = order[nextIndex];

    this.detailsWorkspacePane = next;
    this.cdr.markForCheck();

    queueMicrotask(() => this.focusDetailsWorkspacePaneElement(next));
    return true;
  }

  private focusDetailsWorkspacePaneElement(paneId: DetailsWorkspacePane): void {
    const pane = this.host.nativeElement.querySelector<HTMLElement>(
      `[data-details-workspace-pane="${paneId}"]`,
    );
    if (!pane) return;

    if (paneId === 'details') {
      const surface = this.findSurfaceById(this.presentation.layout, 'details-workspace');
      const host = this.detailsWorkspaceViewHost('details');
      if (surface) {
        const entries = (host?.getSortedEntries() ?? this.detailsWorkspaceEntries(surface))
          .filter(entry => !!entry?.path);
        if (entries.length > 0) {
          const currentPath = this.selectedCursorPath(surface);
          const current = currentPath
            ? entries.find(entry => entry.path === currentPath)
            : undefined;
          const cursor = current ?? entries[0];
          this.select(surface, cursor);
          this.cdr.detectChanges();

          const row = pane.querySelector<HTMLElement>(
            `.details-row[data-path="${CSS.escape(cursor.path)}"]`,
          );
          row?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
        }
      }
    }

    const focusTarget = paneId === 'tree'
      ? pane.querySelector<HTMLElement>('.file-tree-container[tabindex="0"]') ?? pane
      : pane;
    focusTarget.focus();
  }

  private navigateDetailsWorkspace(move: -1 | 1 | 'first' | 'last' | 'page-up' | 'page-down'): boolean {
    if (!this.isDetailsWorkspaceSplit(this.presentation.layout)) return false;
    return this.detailsWorkspacePane === 'tree'
      ? this.navigateDetailsWorkspaceTree(move)
      : this.navigateDetailsWorkspaceList(move);
  }

  private navigateDetailsWorkspaceTree(move: -1 | 1 | 'first' | 'last' | 'page-up' | 'page-down'): boolean {
    const surface = this.findSurfaceById(this.presentation.layout, 'details-tree');
    const host = this.detailsWorkspaceViewHost('tree');
    if (!surface || !host) return false;

    const entries = host.getVisibleTreeNodes().filter(node => !!node?.path);
    if (entries.length === 0) return false;

    const currentPath = host.getTreeFocusedPath() ?? this.treeCursorPath(surface);
    const currentIndex = currentPath
      ? entries.findIndex(entry => entry.path === currentPath)
      : -1;
    const nextIndex = this.workspaceNavigationIndex(
      currentIndex,
      entries.length,
      move,
      Math.max(1, host.getApproxPageSize()),
    );
    const next = entries[nextIndex];
    if (!next?.path) return false;

    this.detailsWorkspacePane = 'tree';
    host.focusTreeNode(next.path);
    this.select(surface, next);
    this.cdr.markForCheck();
    return true;
  }

  private navigateDetailsWorkspaceList(move: -1 | 1 | 'first' | 'last' | 'page-up' | 'page-down'): boolean {
    const surface = this.findSurfaceById(this.presentation.layout, 'details-workspace');
    const host = this.detailsWorkspaceViewHost('details');
    if (!surface) return false;

    const entries = (host?.getSortedEntries() ?? this.detailsWorkspaceEntries(surface))
      .filter(entry => !!entry?.path);
    if (entries.length === 0) return false;

    const currentPath = this.selectedCursorPath(surface);
    const currentIndex = currentPath
      ? entries.findIndex(entry => entry.path === currentPath)
      : -1;
    const nextIndex = this.workspaceNavigationIndex(
      currentIndex,
      entries.length,
      move,
      Math.max(1, host?.getApproxPageSize() ?? 10),
    );
    const next = entries[nextIndex];
    if (!next?.path) return false;

    this.detailsWorkspacePane = 'details';
    this.select(surface, next);
    this.cdr.markForCheck();
    queueMicrotask(() => {
      const row = this.host.nativeElement.querySelector<HTMLElement>(
        `.details-row[data-path="${CSS.escape(next.path)}"]`,
      );
      row?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
    });
    return true;
  }

  private workspaceNavigationIndex(
    currentIndex: number,
    length: number,
    move: -1 | 1 | 'first' | 'last' | 'page-up' | 'page-down',
    pageSize: number,
  ): number {
    if (length <= 1) return 0;
    switch (move) {
      case 'first': return 0;
      case 'last': return length - 1;
      case 'page-up': return currentIndex < 0 ? 0 : Math.max(0, currentIndex - pageSize);
      case 'page-down': return currentIndex < 0 ? 0 : Math.min(length - 1, currentIndex + pageSize);
      case -1: return currentIndex < 0 ? length - 1 : Math.max(0, currentIndex - 1);
      case 1: return currentIndex < 0 ? 0 : Math.min(length - 1, currentIndex + 1);
    }
  }

  private openDetailsWorkspaceCursor(): boolean {
    if (!this.isDetailsWorkspaceSplit(this.presentation.layout)) return false;

    if (this.detailsWorkspacePane === 'tree') {
      const surface = this.findSurfaceById(this.presentation.layout, 'details-tree');
      const host = this.detailsWorkspaceViewHost('tree');
      if (!surface || !host) return false;
      const path = host.getTreeFocusedPath() ?? this.treeCursorPath(surface);
      const entry = host.getVisibleTreeNodes().find(node => node.path === path);
      if (!entry) return false;
      this.emitNamedAction(surface, 'select-folder', entry);
      return true;
    }

    const surface = this.findSurfaceById(this.presentation.layout, 'details-workspace');
    if (!surface) return false;
    const path = this.selectedCursorPath(surface);
    const entry = this.detailsWorkspaceEntries(surface).find(item => item.path === path);
    if (!entry) return false;
    this.emitNamedAction(surface, 'open', entry);
    return true;
  }

  private detailsWorkspaceViewHost(pane: DetailsWorkspacePane): PaneViewHostComponent | null {
    return this.paneViewHosts?.find(host =>
      pane === 'tree' ? host.currentView === 'tree' : host.currentView === 'details'
    ) ?? null;
  }

  private syncDetailsWorkspaceInputContext(): void {
    if (!this.detailsWorkspaceInputRegistered) return;
    if (this.presentation?.layout && this.isDetailsWorkspaceSplit(this.presentation.layout)) {
      this.input.activateContext(DETAILS_WORKSPACE_INPUT_CONTEXT);
    } else {
      this.input.deactivateContext(DETAILS_WORKSPACE_INPUT_CONTEXT);
    }
  }

  constructor(
    private readonly host: ElementRef<HTMLElement>,
    private readonly input: InputEngineService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  trackSurface = (_: number, surface: WorkspaceSurface) => surface.id;
  trackAction = (_: number, action: WorkspaceSurfaceAction) => action.id;
  trackEntry = (index: number, entry: any) => entry?.path ?? `${index}`;
  trackCompareRow = (index: number, row: any) => row?.key ?? `${index}`;
  trackSummary = (index: number, item: any) => item?.label ?? `${index}`;

  isDetailsWorkspaceSplit(layout: WorkspaceLayout): boolean {
    if (layout.kind !== 'split' || layout.direction !== 'horizontal') {
      return false;
    }

    return (
      this.layoutContainsSurface(layout.first, 'details-tree') &&
      this.layoutContainsSurface(layout.second, 'details-workspace')
    );
  }

  text(surface: WorkspaceSurface, key: string): string {
    const value = surface.state?.[key];
    return value === null || value === undefined ? '' : String(value);
  }

  array(surface: WorkspaceSurface, key: string): any[] {
    const value = surface.state?.[key];
    return Array.isArray(value) ? value : [];
  }

  fileEntries(surface: WorkspaceSurface): FileInfo[] {
    return this.array(surface, 'containment') as FileInfo[];
  }

  treeEntries(surface: WorkspaceSurface): FileInfo[] {
    return this.array(surface, 'entries') as FileInfo[];
  }

  selected(surface: WorkspaceSurface): unknown {
    return this.selectedBySurface.get(surface.id);
  }

  select(surface: WorkspaceSurface, value: unknown): void {
    this.selectedBySurface.set(surface.id, value);
  }

  isSelected(surface: WorkspaceSurface, value: any): boolean {
    const selected = this.selected(surface) as any;
    if (!selected) return false;
    return (
      (selected.path && value?.path && selected.path === value.path) ||
      (selected.key && value?.key && selected.key === value.key) ||
      selected === value
    );
  }

  selectedPathSet(surface: WorkspaceSurface): Set<string> {
    const selectableEntries = surface.surfaceId === 'details-workspace'
      ? this.detailsWorkspaceEntries(surface)
      : this.fileEntries(surface);
    const validPaths = new Set(selectableEntries.map(entry => entry.path));
    const selected = this.selected(surface);
    if (Array.isArray(selected)) {
      const paths = selected
        .map(entry => String((entry as any)?.path ?? ''))
        .filter(path => !!path && validPaths.has(path));
      if (paths.length > 0) return new Set(paths);
    } else {
      const path = String((selected as any)?.path ?? '');
      if (path && validPaths.has(path)) return new Set([path]);
    }

    const statePath = this.text(surface, 'selectedPath');
    return statePath && validPaths.has(statePath)
      ? new Set([statePath])
      : new Set<string>();
  }

  selectedCursorPath(surface: WorkspaceSurface): string | null {
    const paths = this.selectedPathSet(surface);
    return paths.values().next().value ?? null;
  }

  sortDirection(surface: WorkspaceSurface): 'asc' | 'desc' {
    return this.text(surface, 'sortDirection') === 'desc' ? 'desc' : 'asc';
  }

  treeSelectedPathSet(surface: WorkspaceSurface): Set<string> {
    const local = this.selected(surface) as { path?: string } | null;
    const localPath = String(local?.path ?? '');
    if (localPath) return new Set([localPath]);

    const current =
      this.text(surface, 'selectedPath') ||
      this.text(surface, 'currentPath');
    return current ? new Set([current]) : new Set<string>();
  }

  treeCursorPath(surface: WorkspaceSurface): string | null {
    const local = this.selected(surface) as { path?: string } | null;
    const localPath = String(local?.path ?? '');
    if (localPath) return localPath;

    const current =
      this.text(surface, 'selectedPath') ||
      this.text(surface, 'currentPath');
    return current || null;
  }

  detailsWorkspaceEntries(surface: WorkspaceSurface): FileInfo[] {
    const entries = this.fileEntries(surface);
    const currentPath = this.text(surface, 'path');
    const regularEntries = entries.filter(entry => entry.path !== PARENT_ENTRY_PATH);

    if (!currentPath || this.isDetailsWorkspaceDriveRoot(currentPath)) {
      return regularEntries;
    }

    const existingParent = entries.find(entry => entry.path === PARENT_ENTRY_PATH);
    const parentEntry: FileInfo = existingParent ?? {
      name: '..',
      path: PARENT_ENTRY_PATH,
      type: FileType.DIRECTORY,
      size: 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
    };

    return [parentEntry, ...regularEntries];
  }

  onDetailsSelectionChange(
    surface: WorkspaceSurface,
    entries: FileInfo[],
  ): void {
    if (entries.length <= 1) {
      const entry = entries[0] ?? null;
      this.select(surface, entry);
      return;
    }

    this.select(surface, entries);
  }

  onTreeSelectionChange(
    surface: WorkspaceSurface,
    entries: FileInfo[],
  ): void {
    const entry = entries[0];
    if (!entry?.path) return;

    // Tree selection is cursor movement only. Enter/double-click owns containment changes.
    this.select(surface, entry);
    this.cdr.markForCheck();
  }

  private isDetailsWorkspaceDriveRoot(path: string): boolean {
    const normalized = (path ?? '').replace(/\\/g, '/').replace(/\/+$/, '');
    return /^[a-z]:$/i.test(normalized) || /^filesystem:\/\/[a-z]$/i.test(normalized);
  }

  emitAction(
    surface: WorkspaceSurface,
    action: WorkspaceSurfaceAction,
  ): void {
    this.surfaceAction.emit({
      surface,
      action: action.id,
      payload: action.requiresSelection
        ? this.selected(surface)
        : undefined,
    });
  }

  emitNamedAction(
    surface: WorkspaceSurface,
    action: string,
    payload?: unknown,
  ): void {
    this.surfaceAction.emit({ surface, action, payload });
  }

  formatSize(value: unknown): string {
    const bytes = Number(value);
    if (!Number.isFinite(bytes) || bytes < 0) return '—';
    if (bytes < 1024) return `${bytes} B`;

    const units = ['KB', 'MB', 'GB', 'TB', 'PB'];
    let amount = bytes / 1024;
    let unit = units[0];

    for (let i = 1; i < units.length && amount >= 1024; i++) {
      amount /= 1024;
      unit = units[i];
    }

    return `${amount.toFixed(amount >= 100 ? 0 : amount >= 10 ? 1 : 2)} ${unit}`;
  }

  private formatDate(value: unknown): string {
    const timestamp = Number(value);
    if (!Number.isFinite(timestamp) || timestamp <= 0) return '';
    const date = new Date(timestamp > 10_000_000_000 ? timestamp : timestamp * 1000);
    return Number.isNaN(date.getTime()) ? '' : date.toLocaleString();
  }

  private loadDetailsWorkspaceViewPreset(): void {
    try {
      const stored = localStorage.getItem('reassembly.details-workspace.view');
      if (stored && isDetailsWorkspaceViewPresetId(stored)) {
        this.detailsWorkspaceViewPreset = stored;
      }
    } catch {}
  }

  private applyDetailsWorkspaceColumns(): void {
    const fields: Readonly<Record<DetailsWorkspaceViewPresetId, readonly string[]>> = {
      security: ['name', 'owner', 'permissions', 'attributes'],
      timestamps: ['name', 'created', 'modified', 'accessed'],
      filesystem: ['name', 'type', 'attributes', 'isHidden', 'isSymlink'],
    };
    const selected = new Set(fields[this.detailsWorkspaceViewPreset]);
    this.detailsWorkspaceColumns = this.detailsWorkspaceAllColumns
      .filter(column => selected.has(String(column.field)))
      .map(column => ({ ...column }));
  }

  private findSurfaceById(
    layout: WorkspaceLayout,
    surfaceId: string,
  ): WorkspaceSurface | null {
    switch (layout.kind) {
      case 'surface':
        return layout.surface.surfaceId === surfaceId ? layout.surface : null;
      case 'split':
        return this.findSurfaceById(layout.first, surfaceId)
          ?? this.findSurfaceById(layout.second, surfaceId);
      case 'stack':
        return layout.surfaces.find(surface => surface.surfaceId === surfaceId) ?? null;
    }
  }

  private layoutContainsSurface(
    layout: WorkspaceLayout,
    surfaceId: string,
  ): boolean {
    switch (layout.kind) {
      case 'surface':
        return layout.surface.surfaceId === surfaceId;

      case 'split':
        return (
          this.layoutContainsSurface(layout.first, surfaceId) ||
          this.layoutContainsSurface(layout.second, surfaceId)
        );

      case 'stack':
        return layout.surfaces.some(surface => surface.surfaceId === surfaceId);
    }
  }
}
