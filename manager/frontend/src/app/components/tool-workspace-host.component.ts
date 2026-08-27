import {
  ChangeDetectionStrategy,
  Component,
  EventEmitter,
  Input,
  Output,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import type {
  WorkspaceLayout,
  WorkspacePresentation,
  WorkspaceSurface,
  WorkspaceSurfaceAction,
} from '../services/workspace-surface.model';

@Component({
  selector: 'app-tool-workspace-host',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section
      class="tool-workspace"
      role="region"
      [attr.aria-label]="presentation.title">

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

    <ng-template #detailsTreeNodes let-nodes="nodes" let-surface="surface" let-depth="depth">
      <ng-container *ngFor="let node of nodes; trackBy: trackTreeNode">
        <div
          class="details-tree-row"
          [class.cursor]="text(surface, 'selectedPath') === node.path"
          [style.--tree-depth]="depth"
          [attr.aria-current]="text(surface, 'selectedPath') === node.path ? 'true' : null"
          (click)="emitNamedAction(surface, 'select-folder', node)">
          <button
            type="button"
            class="tree-expander"
            [class.placeholder]="node.loaded && (!node.children || node.children.length === 0)"
            [attr.aria-label]="node.expanded ? 'Collapse folder' : 'Expand folder'"
            (click)="emitNamedAction(surface, 'toggle-folder', node); $event.stopPropagation()">
            {{ node.loading ? '…' : node.expanded ? '▾' : '▸' }}
          </button>
          <span class="tree-folder-icon">📁</span>
          <span class="tree-folder-name" [title]="node.path">{{ node.name }}</span>
        </div>
        <ng-container *ngIf="node.expanded && node.children?.length">
          <ng-container
            *ngTemplateOutlet="detailsTreeNodes; context: { nodes: node.children, surface: surface, depth: depth + 1 }">
          </ng-container>
        </ng-container>
      </ng-container>
    </ng-template>

    <ng-template #renderSurface let-surface>
      <article class="surface" [attr.data-surface-id]="surface.surfaceId">
        <header class="surface-header">
          <strong>{{ text(surface, 'title') || surface.title }}</strong>

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

            <span>{{ surface.role || 'primary' }}</span>
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

          <section *ngSwitchCase="'details-tree'" class="details-tree-surface">
            <ng-container
              *ngTemplateOutlet="detailsTreeNodes; context: { nodes: array(surface, 'nodes'), surface: surface, depth: 0 }">
            </ng-container>
          </section>

          <section *ngSwitchCase="'details-workspace'" class="details-surface details-inspector">
            <div class="details-summary" *ngIf="object(surface, 'containmentSummary') as summary">
              <div><strong>{{ summary.folders || 0 }}</strong><span>Folders</span></div>
              <div><strong>{{ summary.files || 0 }}</strong><span>Files</span></div>
              <div><strong>{{ formatSize(summary.totalSize) }}</strong><span>Total size</span></div>
            </div>

            <section class="details-section containment-section">
              <div class="details-section-heading">
                <div class="details-heading-copy">
                  <h3>Selected folder contents</h3>
                  <span class="details-current-path" [title]="text(surface, 'path')">
                    {{ text(surface, 'path') }}
                  </span>
                </div>
                <span>{{ array(surface, 'containment').length }} immediate items</span>
              </div>
              <div class="containment-table">
                <div class="containment-row containment-header">
                  <span>Name</span><span>Type</span><span>Size</span><span>Modified</span>
                </div>
                <div class="containment-row" *ngFor="let entry of array(surface, 'containment'); trackBy: trackContainment">
                  <span class="containment-name" [title]="entry.path">
                    <span class="containment-icon">{{ entry.isDirectory ? '📁' : '📄' }}</span>{{ entry.name }}
                  </span>
                  <span>{{ entry.type }}</span>
                  <span>{{ entry.isDirectory ? '—' : formatSize(entry.size) }}</span>
                  <span>{{ entry.modified }}</span>
                </div>
                <div class="containment-empty" *ngIf="array(surface, 'containment').length === 0">Folder is empty.</div>
              </div>
            </section>

            <section class="details-section" *ngFor="let section of array(surface, 'sections'); trackBy: trackDetailSection">
              <h3>{{ section.title }}</h3>
              <div class="detail-grid">
                <div class="detail-row" *ngFor="let item of section.items; trackBy: trackDetail">
                  <span>{{ item.label }}</span>
                  <strong [title]="item.value">{{ item.value }}</strong>
                </div>
              </div>
            </section>

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
      padding: 4px;
      overflow: auto;
    }

    .split.details-workspace-split .details-tree-surface {
      padding: 4px 2px 4px 4px;
    }

    .details-tree-row {
      display: grid;
      grid-template-columns: 16px 18px minmax(0, 1fr);
      align-items: center;
      box-sizing: border-box;
      min-height: 27px;
      padding: 1px 6px 1px calc(4px + (var(--tree-depth) * 14px));
      border-radius: 3px;
      cursor: default;
      user-select: none;
    }

    .details-tree-row:hover:not(.cursor) {
      background: var(--vsc-list-hover-background);
    }

    .details-tree-row.cursor {
      box-shadow:
        inset 3px 0 0 var(--vsc-accent-blue, var(--vsc-focus-border)),
        inset 0 0 0 1px color-mix(
          in srgb,
          var(--vsc-accent-blue, var(--vsc-focus-border)) 42%,
          transparent
        );
      background: color-mix(
        in srgb,
        var(--vsc-list-hover-background, var(--vsc-list-hover-background)) 75%,
        transparent
      );
    }

    .tree-expander {
      width: 16px;
      height: 20px;
      min-height: 20px;
      padding: 0;
      border: 0;
      background: transparent;
      color: inherit;
    }

    .tree-folder-icon {
      width: 18px;
      text-align: center;
    }

    .tree-folder-name {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .tree-expander.placeholder {
      visibility: hidden;
      pointer-events: none;
    }

    .tree-folder-icon {
      font-size: 13px;
      line-height: 1;
    }

    .tree-folder-name {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .details-inspector {
      padding: 0 12px 16px;
    }

    .split.details-workspace-split .details-inspector {
      padding-right: 16px;
    }

    .details-summary {
      position: sticky;
      top: 0;
      z-index: 2;
      display: grid;
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 1px;
      margin: 0 -12px 14px;
      border-bottom: 1px solid var(--vsc-border-subtle);
      background: var(--vsc-border-subtle);
    }

    .details-summary > div {
      display: flex;
      flex-direction: column;
      gap: 2px;
      padding: 10px 12px;
      background: var(--vsc-side-bar-background, var(--vsc-panel-background));
    }

    .details-summary strong { font-size: 16px; font-weight: 600; }
    .details-summary span,
    .details-section-heading span {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 10px;
    }

    .details-section {
      margin-top: 14px;
    }

    .details-section h3 {
      margin: 0 0 6px;
      color: var(--vsc-foreground-bright, var(--vsc-foreground));
      font-size: 12px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: .04em;
    }

    .details-section-heading {
      display: flex;
      align-items: baseline;
      justify-content: space-between;
      gap: 12px;
    }

    .details-heading-copy {
      display: flex;
      flex-direction: column;
      min-width: 0;
      gap: 3px;
    }

    .details-current-path {
      display: block;
      min-width: 0;
      max-width: 100%;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
      text-transform: none;
      letter-spacing: 0;
    }

    .detail-grid {
      border: 1px solid var(--vsc-border-subtle, var(--vsc-control-background));
      border-radius: 4px;
      overflow: hidden;
    }

    .detail-row {
      display: grid;
      grid-template-columns: minmax(140px, 210px) minmax(0, 1fr);
      gap: 12px;
      min-height: 32px;
      align-items: center;
      padding: 4px 10px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-control-background));
    }

    .detail-row:last-child { border-bottom: 0; }

    .detail-row > span {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .detail-row > strong {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-weight: 500;
    }

    .containment-table {
      min-width: 620px;
      border: 1px solid var(--vsc-border-subtle, var(--vsc-control-background));
      border-radius: 4px;
      overflow: hidden;
    }

    .containment-row {
      display: grid;
      grid-template-columns: minmax(180px, 1fr) 90px 100px 160px;
      align-items: center;
      min-height: 30px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-control-background));
    }

    .containment-row:last-child { border-bottom: 0; }
    .containment-row > span { padding: 4px 8px; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
    .containment-header {
      position: sticky;
      top: 0;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      background: var(--vsc-side-bar-background, var(--vsc-panel-background));
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: .04em;
    }
    .containment-name { display: flex; align-items: center; gap: 6px; }
    .containment-icon { flex: 0 0 auto; font-size: 13px; }
    .containment-empty { padding: 18px 10px; color: var(--vsc-description-foreground); text-align: center; }

    .unknown-surface {
      padding: 16px;
    }
  `],
})
export class ToolWorkspaceHostComponent {
  @Input({ required: true }) presentation!: WorkspacePresentation;
  @Output() close = new EventEmitter<void>();
  @Output() pin = new EventEmitter<void>();
  @Output() surfaceAction = new EventEmitter<{
    surface: WorkspaceSurface;
    action: string;
    payload?: unknown;
  }>();

  private readonly selectedBySurface = new Map<string, unknown>();

  trackSurface = (_: number, surface: WorkspaceSurface) => surface.id;
  trackAction = (_: number, action: WorkspaceSurfaceAction) => action.id;
  trackEntry = (index: number, entry: any) => entry?.path ?? `${index}`;
  trackCompareRow = (index: number, row: any) => row?.key ?? `${index}`;
  trackSummary = (index: number, item: any) => item?.label ?? `${index}`;
  trackDetail = (index: number, item: any) => item?.label ?? `${index}`;
  trackDetailSection = (index: number, section: any) => section?.title ?? `${index}`;
  trackContainment = (index: number, entry: any) => entry?.path ?? `${index}`;
  trackTreeNode = (index: number, node: any) => node?.path ?? `${index}`;

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

  object(surface: WorkspaceSurface, key: string): any {
    const value = surface.state?.[key];
    return value && typeof value === 'object' && !Array.isArray(value) ? value : {};
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
