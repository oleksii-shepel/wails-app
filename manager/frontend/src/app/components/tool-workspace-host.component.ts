import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  EventEmitter,
  Input,
  OnDestroy,
  OnInit,
  Output,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { PaneViewHostComponent } from './pane-view-host.component';
import type { PaneDetailsColumn } from './pane-details.component';
import type { FileInfo } from '../shared/protocol';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';
import { FILE_MANAGER_COMMANDS } from './command-registry';

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
const DETAILS_WORKSPACE_INPUT_BINDINGS: readonly InputBindingDefinition[] = [
  {
    id: 'reassembly.workspace.details.switchPane.tab',
    commandId: FILE_MANAGER_COMMANDS.switchPane,
    contextId: DETAILS_WORKSPACE_INPUT_CONTEXT,
    source: 'workspace',
    description: 'Switch Details workspace pane',
    category: 'Workspace',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'tab' }] } },
    preventDefault: true,
    stopPropagation: true,
  },
  {
    id: 'reassembly.workspace.details.switchPane.shiftTab',
    commandId: FILE_MANAGER_COMMANDS.switchPane,
    contextId: DETAILS_WORKSPACE_INPUT_CONTEXT,
    source: 'workspace',
    description: 'Switch Details workspace pane backward',
    category: 'Workspace',
    trigger: { device: 'keyboard', chord: { steps: [{ key: 'tab', modifiers: ['shift'] }] } },
    preventDefault: true,
    stopPropagation: true,
  },
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
              [entries]="treeEntries(surface)"
              [currentPath]="text(surface, 'currentPath') || text(surface, 'selectedPath') || text(surface, 'rootPath')"
              [selectedFiles]="treeSelectedPathSet(surface)"
              [cursorPath]="treeCursorPath(surface)"
              [currentView]="'tree'"
              [treeContentMode]="'folders'"
              [paneActive]="detailsWorkspacePane === 'tree'"
              (selectionChange)="onTreeSelectionChange(surface, $event)"
              (navigate)="emitNamedAction(surface, 'select-folder', $event)"
              (nodeDoubleClick)="emitNamedAction(surface, 'select-folder', $event)">
            </app-pane-view-host>
          </section>

          <section
            *ngSwitchCase="'details-workspace'"
            class="details-surface details-inspector list view-details"
            data-details-workspace-pane="details"
            [class.workspace-pane-active]="detailsWorkspacePane === 'details'"
            [attr.tabindex]="detailsWorkspacePane === 'details' ? 0 : -1"
            (focusin)="activateDetailsWorkspacePane('details')"
            (mousedown)="activateDetailsWorkspacePane('details')">
            <app-pane-view-host
              [surfaceId]="surface.id"
              [surfaceActive]="true"
              [paneId]="'right'"
              [entries]="fileEntries(surface)"
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
      padding: 8px;
    }

    .layout,
    .surface {
      width: 100%;
      height: 100%;
      min-width: 0;
      min-height: 0;
    }

    .split {
      display: grid;
      gap: 8px;
    }

    .split.details-workspace-split {
      gap: 10px;
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
export class ToolWorkspaceHostComponent implements OnInit, OnDestroy {
  @Input({ required: true }) presentation!: WorkspacePresentation;
  @Output() close = new EventEmitter<void>();
  @Output() pin = new EventEmitter<void>();
  @Output() surfaceAction = new EventEmitter<{
    surface: WorkspaceSurface;
    action: string;
    payload?: unknown;
  }>();

  private readonly selectedBySurface = new Map<string, unknown>();

  readonly detailsWorkspaceViewPresets = DETAILS_WORKSPACE_VIEW_PRESETS;
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
    this.input.activateContext(DETAILS_WORKSPACE_INPUT_CONTEXT);
  }

  ngOnDestroy(): void {
    this.input.deactivateContext(DETAILS_WORKSPACE_INPUT_CONTEXT);
    for (const binding of DETAILS_WORKSPACE_INPUT_BINDINGS) {
      this.input.unregisterBinding(binding.id, DETAILS_WORKSPACE_INPUT_OWNER);
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

  onDetailsWorkspaceTab(event: KeyboardEvent): void {
    if (!this.isDetailsWorkspaceSplit(this.presentation.layout)) return;
    event.preventDefault();
    event.stopPropagation();
    this.switchDetailsWorkspacePane();
  }

  switchDetailsWorkspacePane(): boolean {
    if (!this.isDetailsWorkspaceSplit(this.presentation.layout)) return false;

    const active = document.activeElement as HTMLElement | null;
    const currentPane = active?.closest<HTMLElement>('[data-details-workspace-pane]')
      ?.dataset['detailsWorkspacePane'] as DetailsWorkspacePane | undefined;
    const current = currentPane ?? this.detailsWorkspacePane;
    const next: DetailsWorkspacePane = current === 'tree' ? 'details' : 'tree';

    this.detailsWorkspacePane = next;
    this.cdr.markForCheck();

    queueMicrotask(() => {
      const pane = this.host.nativeElement.querySelector<HTMLElement>(
        `[data-details-workspace-pane="${next}"]`,
      );
      if (!pane) return;

      const focusTarget = next === 'tree'
        ? pane.querySelector<HTMLElement>('.file-tree-container[tabindex="0"]') ?? pane
        : pane;
      focusTarget.focus();
    });

    return true;
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
    const selected = this.selected(surface);
    if (Array.isArray(selected)) {
      return new Set(
        selected
          .map(entry => String((entry as any)?.path ?? ''))
          .filter(Boolean),
      );
    }

    const path = String((selected as any)?.path ?? '');
    return path ? new Set([path]) : new Set<string>();
  }

  selectedCursorPath(surface: WorkspaceSurface): string | null {
    const selected = this.selected(surface);
    if (Array.isArray(selected)) {
      return selected.length > 0
        ? String((selected[0] as any)?.path ?? '') || null
        : null;
    }

    const path = String((selected as any)?.path ?? '');
    return path || null;
  }

  sortDirection(surface: WorkspaceSurface): 'asc' | 'desc' {
    return this.text(surface, 'sortDirection') === 'desc' ? 'desc' : 'asc';
  }

  treeSelectedPathSet(surface: WorkspaceSurface): Set<string> {
    const current =
      this.text(surface, 'selectedPath') ||
      this.text(surface, 'currentPath');
    return current ? new Set([current]) : new Set<string>();
  }

  treeCursorPath(surface: WorkspaceSurface): string | null {
    const current =
      this.text(surface, 'selectedPath') ||
      this.text(surface, 'currentPath');
    return current || null;
  }

  onDetailsSelectionChange(
    surface: WorkspaceSurface,
    entries: FileInfo[],
  ): void {
    if (entries.length <= 1) {
      this.select(surface, entries[0] ?? null);
      return;
    }

    this.select(surface, entries);
  }

  onTreeSelectionChange(
    surface: WorkspaceSurface,
    entries: FileInfo[],
  ): void {
    const entry = entries[0];
    if (!entry?.path) {
      return;
    }

    this.emitNamedAction(surface, 'select-folder', entry);
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