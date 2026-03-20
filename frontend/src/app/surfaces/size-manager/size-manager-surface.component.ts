import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  Input,
  OnChanges,
  OnDestroy,
  Output,
  SimpleChanges,
} from '@angular/core';
import { ApiService } from '../../services/api.service';
import { DEFAULT_FILE_CATEGORIES, type FileCategory } from '../../components/filter-bar.types';
import type { DiskUsageEntry, DiskUsageProgress, DiskUsageResult, DiskUsageTypeStat } from '../../shared/protocol-enhanced';
import type { SizeManagerSurfaceContext } from '../surface-context';
import { DiskPieChartComponent } from 'src/app/components/disk-usage-diagram.component';
import { VscodeScrollbarDirective } from 'src/app/directives/vscode-scrollbar.directive';
import { buildSegments, normalizePath, parentPath } from '../../components/address-bar.utils';

// ── Pie nav frame ─────────────────────────────────────────────────────────────

interface PieFrame {
  label: string;
  path?: string;
  sourceEntry?: DiskUsageEntry | null;
  entries: DiskUsageEntry[];
  /**
   * Denominator for % calculations at this level.
   * Root: directory totalSize.
   * Sub-levels: sum of the Other bucket that was drilled into.
   */
  totalSize: number;
}

interface FileTypeBreakdownRow {
  id: string;
  name: string;
  icon: string;
  count: number;
  totalSize: number;
}

interface FileTypeBreakdownSource {
  label: string;
  fileCount: number;
  totalSize: number;
  types: Record<string, DiskUsageTypeStat>;
}

const SIZE_MANAGER_FILE_CATEGORY_IDS = [
  'video',
  'images',
  'audio',
  'documents',
  'archives',
] as const;

const SIZE_MANAGER_FILE_CATEGORIES: FileCategory[] = SIZE_MANAGER_FILE_CATEGORY_IDS
  .map((id) => DEFAULT_FILE_CATEGORIES.find((category) => category.id === id))
  .filter((category): category is FileCategory => !!category);

const SIZE_MANAGER_CATEGORY_BY_EXTENSION = new Map<string, FileCategory>(
  SIZE_MANAGER_FILE_CATEGORIES.flatMap((category) =>
    category.extensions.map((extension) => [extension, category] as const),
  ),
);

const PIE_PREFETCH_LIMIT = 5;

// ─────────────────────────────────────────────────────────────────────────────

@Component({
  selector: 'app-size-manager-surface',
  standalone: true,
  imports: [CommonModule, DiskPieChartComponent, VscodeScrollbarDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <!-- ── Header bar ─────────────────────────────────────── -->
    <div class="sm-header">
      <div class="sm-header-left">
        <span class="sm-kicker">Disk Usage</span>
        <span class="sm-sep">·</span>
        <span class="sm-current-label">{{ headerLabel }}</span>
        <span class="sm-path" *ngIf="showHeaderPath" [title]="headerPath">{{ headerPath }}</span>
      </div>
      <div class="sm-header-actions">
        <button class="sm-btn" [disabled]="!parentPath" (click)="goUp()" title="Go to parent">↑ Parent</button>
        <button class="sm-btn" (click)="refresh()">↺ Refresh</button>
        <button class="sm-btn sm-btn--close" (click)="close()" title="Close">✕</button>
      </div>
    </div>

    <!-- ── Error ──────────────────────────────────────────── -->
    <div class="sm-error" *ngIf="error && !isTraversing">
      <span class="sm-error-icon">⚠</span>
      <span>{{ error }}</span>
      <button class="sm-btn sm-btn--primary" (click)="refresh()">Retry</button>
    </div>

    <!-- ── Dashboard ──────────────────────────────────────── -->
    <div class="sm-body" *ngIf="!error || isTraversing" appVscodeScrollbar>

      <div class="sm-inline-status" *ngIf="isTraversing">
        <span
          class="sm-inline-activity"
          [attr.aria-label]="headerProgressLabel"
          [title]="headerProgressLabel"
          role="status">
          <span class="sm-inline-activity-dot"></span>
        </span>
        <span class="sm-inline-elapsed">{{ liveElapsedLabel }}</span>
        <span class="sm-inline-sep">·</span>
        <span class="sm-inline-throughput">{{ liveThroughputLabel }}</span>
        <span class="sm-inline-sep">·</span>
        <span class="sm-inline-traversal">{{ liveTraversalSummary }}</span>
      </div>

      <!-- Metric strip -->
      <div class="sm-metrics">
        <div class="sm-metric">
          <div class="sm-metric-kicker">{{ scopeMetricTitle }}</div>
          <div class="sm-metric-val">{{ directChildrenCount | number }}</div>
          <div class="sm-metric-lbl">{{ scopeMetricHint }}</div>
        </div>
        <div class="sm-metric">
          <div class="sm-metric-kicker">Footprint</div>
          <div class="sm-metric-val">{{ totalFootprintLabel }}</div>
          <div class="sm-metric-lbl">{{ totalCompositionLabel }}</div>
        </div>
        <div class="sm-metric">
          <div class="sm-metric-kicker">Top File Type</div>
          <div class="sm-metric-val">{{ dominantFileTypeValue }}</div>
          <div class="sm-metric-lbl">{{ dominantFileTypeHint }}</div>
        </div>
      </div>

      <!-- ── Pie panel ──────────────────────────────────────── -->
      <div class="sm-panel">

        <!-- Distribution breadcrumb nav -->
        <div class="sm-pie-nav" *ngIf="pieStack.length > 0">
          <button
            *ngFor="let frame of pieStack; let i = index; let last = last"
            class="sm-pie-crumb"
            [class.sm-pie-crumb--active]="last"
            [attr.aria-disabled]="last ? 'true' : null"
            [attr.aria-current]="last ? 'page' : null"
            [attr.tabindex]="last ? -1 : 0"
            (click)="pieGoTo(i)">{{ frame.label }}</button>
          <span class="sm-pie-loading" *ngIf="pieDistributionLoadingPath">
            <span class="sm-pie-loading-dot"></span>
            <span class="sm-pie-loading-copy">Loading children for {{ pieDistributionLoadingLabel }}</span>
          </span>
        </div>

        <ng-container *ngIf="currentPieFrame.entries.length > 0; else emptyFootprintState">
          <!-- Pure display chart — receives only the current frame's data -->
          <app-disk-pie-chart
            [entries]="currentPieFrame.entries"
            [totalSize]="currentPieFrame.totalSize"
            [minPct]="10"
            [isTraversing]="isTraversing"
            [selectedPath]="selectedEntry?.path ?? null"
            (select)="onChartSelect($event)"
            (activate)="onChartActivate($event)"
            (otherDrill)="onOtherDrill($event)"
             (drillDown)="onChartActivate($event)">
          </app-disk-pie-chart>
        </ng-container>

        <ng-template #emptyFootprintState>
          <div class="sm-pie-empty" [class.sm-pie-empty--loading]="isTraversing">
            <div class="sm-pie-empty-icon" aria-hidden="true">{{ isTraversing ? '◌' : '○' }}</div>
            <div class="sm-pie-empty-copy">
              <div class="sm-pie-empty-title">{{ pieEmptyStateTitle }}</div>
              <div class="sm-pie-empty-hint">{{ pieEmptyStateHint }}</div>
            </div>
          </div>
        </ng-template>

      </div>

      <!-- ── File type breakdown ─────────────────────────── -->
      <div class="sm-panel sm-file-types">
        <div class="sm-panel-head">
          <span class="sm-panel-title">File Types</span>
          <span class="sm-panel-hint">{{ fileTypePanelHint }}</span>
        </div>

        <div class="sm-table-head sm-type-table-head" *ngIf="fileTypeRows.length > 0">
          <span>Type</span>
          <span class="sm-align-r">Files</span>
          <span class="sm-align-r">Size</span>
          <span class="sm-align-r">Share</span>
        </div>

        <div class="sm-table-body" *ngIf="fileTypeRows.length > 0; else emptyFileTypes">
          <div class="sm-type-row" *ngFor="let row of fileTypeRows; trackBy: trackFileTypeRow">
            <div class="sm-row-name">
              <span class="sm-row-icon">{{ row.icon }}</span>
              <span class="sm-row-label">{{ row.name }}</span>
            </div>
            <span class="sm-row-size sm-align-r">{{ row.count | number }}</span>
            <span class="sm-row-size sm-align-r">{{ formatBytes(row.totalSize) }}</span>
            <div class="sm-row-pct-cell sm-align-r">
              <span class="sm-row-pct">{{ fileTypePct(row) }}</span>
              <div class="sm-row-track">
                <div class="sm-row-bar" [style.width.%]="fileTypeBarPercent(row)"></div>
              </div>
            </div>
          </div>
        </div>

        <ng-template #emptyFileTypes>
          <div class="sm-empty">{{ fileTypeEmptyLabel }}</div>
        </ng-template>
      </div>

      <!-- ── Breakdown table ──────────────────────────────── -->
      <div class="sm-panel sm-breakdown">
        <div class="sm-panel-head">
          <span class="sm-panel-title">Breakdown</span>
          <span class="sm-panel-hint">Click a folder to drill in · click a file to inspect</span>
        </div>

        <div class="sm-table-head" *ngIf="entries.length > 0">
          <span>Name</span>
          <span>Composition</span>
          <span class="sm-align-r">Size</span>
          <span class="sm-align-r">Share</span>
        </div>

        <div class="sm-table-body" *ngIf="entries.length > 0; else emptyBreakdown">
          <button
            class="sm-row"
            *ngFor="let e of entries; trackBy: trackEntry"
            [class.sm-row--dir]="e.isDir"
            [class.sm-row--active]="selectedEntry?.path === e.path"
            (click)="onEntryClick(e)">
            <div class="sm-row-name">
              <span class="sm-row-icon" [class.sm-row-icon--dir]="e.isDir">{{ e.isDir ? '▣' : '•' }}</span>
              <span class="sm-row-label" [title]="e.path">{{ e.name }}</span>
            </div>
            <span class="sm-row-meta">{{ entryComposition(e) }}</span>
            <span class="sm-row-size sm-align-r">{{ formatBytes(e.size) }}</span>
            <div class="sm-row-pct-cell sm-align-r">
              <span class="sm-row-pct">{{ pct(e) }}</span>
              <div class="sm-row-track">
                <div class="sm-row-bar" [style.width.%]="barPercent(e)"></div>
              </div>
            </div>
          </button>
        </div>

        <ng-template #emptyBreakdown>
          <div class="sm-empty">{{ breakdownEmptyLabel }}</div>
        </ng-template>
      </div>
    </div>
  `,
  styles: [`
    :host {
      --_bg:         var(--vsc-editor-background,    #1e1e1e);
      --_surface:    var(--vsc-panel-background,     #252526);
      --_surface2:   var(--vsc-list-hover-background,#2a2d2e);
      --_input-bg:   var(--vsc-input-background,     #3c3c3c);
      --_text:       var(--vsc-foreground,           #cccccc);
      --_text-dim:   var(--vsc-foreground-dim,       #888888);
      --_border:     var(--vsc-border,               #454545);
      --_border-sub: var(--vsc-border-subtle,        #3a3a3a);
      --_focus:      var(--vsc-input-focus-border,   #007acc);
      --_btn-bg:     var(--vsc-button-background,    #0e639c);
      --_btn-hover:  var(--vsc-button-hover,         #1177bb);
      --_hover-bg:   var(--vsc-list-hover-background,#2a2d2e);
      --_active-bg:  var(--vsc-list-active-background,#094771);
      --_accent-a:   #2ec4b6;
      --_accent-b:   #3a86ff;
      --_error:      var(--vsc-error,                #f48771);
      --_blue:       var(--vsc-accent-blue,          #007acc);
      --_icon-file:  #8fd3ff;
      --_icon-dir:   #dcb67a;

      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      font-size: var(--vsc-font-size, 13px);
      font-family: var(--vsc-font-family, system-ui, sans-serif);
      color: var(--_text);
      background: var(--_bg);
    }

    :host-context(.vscode-light) {
      --_bg: #ffffff; --_surface: #f3f3f3; --_surface2: #e4e6f1;
      --_input-bg: #ffffff; --_text: #3b3b3b; --_text-dim: #737373;
      --_border: #c4c4c4; --_border-sub: #e5e5e5; --_focus: #007acc;
      --_btn-bg: #007acc; --_btn-hover: #005ba1;
      --_hover-bg: #e4e6f1; --_active-bg: #cce0f5;
      --_error: #c7372f; --_blue: #007acc;
      --_accent-a: #0fa89e; --_accent-b: #1a6fd4;
      --_icon-file: #1a6fd4; --_icon-dir: #a0700a;
    }

    .sm-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 14px; flex-shrink: 0; border-bottom: 1px solid var(--_border-sub); background: var(--_surface); }
    .sm-header-left { display: flex; align-items: center; gap: 8px; min-width: 0; overflow: hidden; }
    .sm-kicker { font-size: 10px; text-transform: uppercase; letter-spacing: .14em; color: var(--_text-dim); flex-shrink: 0; }
    .sm-sep { color: var(--_text-dim); flex-shrink: 0; }
    .sm-current-label { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; flex-shrink: 0; max-width: 200px; }
    .sm-path { font-size: 11px; color: var(--_text-dim); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0; }
    .sm-header-actions { display: flex; align-items: center; gap: 6px; flex-shrink: 0; }

    .sm-btn { appearance: none; border: 1px solid var(--_border); border-radius: var(--vsc-border-radius-lg, 4px); padding: 4px 10px; font-size: 12px; font-weight: 500; font-family: inherit; cursor: pointer; color: var(--_text); background: transparent; transition: background .1s, border-color .1s; white-space: nowrap; }
    .sm-btn:hover:not(:disabled) { background: var(--_hover-bg); }
    .sm-btn:disabled { opacity: .35; cursor: default; }
    .sm-btn--primary { background: var(--_btn-bg); color: #fff; border-color: transparent; }
    .sm-btn--primary:hover:not(:disabled) { background: var(--_btn-hover); }
    .sm-btn--close { padding: 4px 8px; }

    .sm-loading { display: flex; align-items: center; gap: 10px; padding: 16px 14px; flex-shrink: 0; color: var(--_text-dim); }
    @keyframes sm-pulse-green {
      0% { box-shadow: 0 0 0 0 rgba(43, 178, 76, 0.45); }
      70% { box-shadow: 0 0 0 8px rgba(43, 178, 76, 0); }
      100% { box-shadow: 0 0 0 0 rgba(43, 178, 76, 0); }
    }

    .sm-error { display: flex; align-items: center; gap: 10px; padding: 12px 14px; flex-shrink: 0; color: var(--_error); border-bottom: 1px solid color-mix(in srgb, var(--_error) 25%, transparent); background: color-mix(in srgb, var(--_error) 8%, var(--_surface)); }
    .sm-error-icon { font-size: 15px; flex-shrink: 0; }

    .sm-body { display: flex; flex-direction: column; flex: 1 1 0%; min-height: 0; overflow: auto; padding: 12px 14px; gap: 12px; scrollbar-width: thin; scrollbar-color: var(--vsc-scrollbar-thumb, rgba(255, 255, 255, 0.16)) transparent; }
    .sm-inline-status { display: flex; align-items: center; gap: 10px; padding: 2px 0; color: var(--_text-dim); }
    .sm-inline-elapsed { font-variant-numeric: tabular-nums; white-space: nowrap; color: #2bb24c; font-weight: 600; }
    .sm-inline-throughput { font-variant-numeric: tabular-nums; white-space: nowrap; color: var(--_text-dim); }
    .sm-inline-traversal { font-variant-numeric: tabular-nums; white-space: nowrap; color: var(--_text-dim); }
    .sm-inline-sep { color: var(--_text-dim); }
    .sm-inline-activity { display: inline-flex; align-items: center; justify-content: center; width: 18px; height: 18px; flex-shrink: 0; border-radius: 999px; background: color-mix(in srgb, #2bb24c 14%, transparent); border: 1px solid color-mix(in srgb, #2bb24c 35%, var(--_border)); }
    .sm-inline-activity-dot { width: 10px; height: 10px; border-radius: 999px; background: #2bb24c; border: 1px solid rgba(255, 255, 255, 0.35); box-shadow: 0 0 0 0 rgba(43, 178, 76, 0.45); animation: sm-pulse-green 1.3s ease-out infinite; }

    .sm-metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(120px, 1fr)); gap: 8px; flex-shrink: 0; }
    .sm-metric { padding: 10px 12px; border-radius: var(--vsc-border-radius-lg, 4px); border: 1px solid var(--_border-sub); background: var(--_surface); min-width: 0; }
    .sm-metric-kicker { font-size: 10px; text-transform: uppercase; letter-spacing: .12em; color: var(--_text-dim); }
    .sm-metric-val { font-size: 18px; font-weight: 700; letter-spacing: -.03em; font-variant-numeric: tabular-nums; line-height: 1.1; }
    .sm-metric-val--text { font-size: 16px; font-variant-numeric: normal; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .sm-metric-lbl { margin-top: 3px; font-size: 11px; color: var(--_text-dim); }

    .sm-panel { display: flex; flex-direction: column; gap: 10px; padding: 12px 14px; border-radius: var(--vsc-border-radius-lg, 4px); border: 1px solid var(--_border-sub); background: var(--_surface); min-width: 0; flex-shrink: 0; }
    .sm-panel-head { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
    .sm-panel-title { font-size: 10px; text-transform: uppercase; letter-spacing: .12em; color: var(--_text-dim); }
    .sm-panel-hint  { font-size: 11px; color: var(--_text-dim); }

    /* Pie breadcrumb nav */
    .sm-pie-nav { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; user-select: none; -webkit-user-select: none; }
    .sm-pie-nav::selection { background: transparent; color: inherit; }
    .sm-pie-nav *::selection { background: transparent; color: inherit; }
    .sm-pie-crumb { appearance: none; border: none; background: transparent; -webkit-text-fill-color: currentColor; -webkit-tap-highlight-color: transparent; -webkit-touch-callout: none; caret-color: transparent; font-family: inherit; font-size: 10px; text-transform: uppercase; letter-spacing: .16em; color: var(--vsc-text-link, var(--_blue)); cursor: pointer; padding: 2px 6px; border-radius: var(--vsc-border-radius-sm, 2px); transition: color .12s, background-color .12s; user-select: none; -webkit-user-select: none; }
    .sm-pie-crumb:not(.sm-pie-crumb--active):hover { color: var(--vsc-text-link, var(--_blue)); background: color-mix(in srgb, var(--vsc-text-link, var(--_blue)) 10%, transparent); text-decoration: none; }
    .sm-pie-crumb:focus-visible { outline: 1px solid var(--_focus); outline-offset: 1px; color: var(--vsc-text-link, var(--_blue)); }
    .sm-pie-crumb--active { color: var(--_text-dim); background: transparent; cursor: default; opacity: 1; text-decoration: none; pointer-events: none; }
    .sm-pie-crumb:not(:last-child)::after { content: '›'; color: var(--_text-dim); margin-left: 8px; }
    .sm-pie-loading { display: inline-flex; align-items: center; gap: 8px; margin-left: 4px; min-width: 0; }
    .sm-pie-loading-dot { width: 8px; height: 8px; border-radius: 999px; background: var(--_focus); box-shadow: 0 0 0 0 color-mix(in srgb, var(--_focus) 40%, transparent); animation: sm-pie-pulse 1.2s ease-out infinite; flex-shrink: 0; }
    .sm-pie-loading-copy { font-size: 11px; color: var(--_text-dim); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .sm-pie-empty { display: flex; align-items: center; justify-content: center; gap: 14px; min-height: 254px; padding: 20px; border: 1px dashed color-mix(in srgb, var(--_border) 80%, transparent); border-radius: var(--vsc-border-radius-lg, 4px); background: color-mix(in srgb, var(--_surface2) 36%, transparent); color: var(--_text-dim); }
    .sm-pie-empty--loading { border-style: solid; }
    .sm-pie-empty-icon { display: inline-flex; align-items: center; justify-content: center; width: 38px; height: 38px; border-radius: 999px; border: 1px solid color-mix(in srgb, var(--_border) 70%, transparent); background: color-mix(in srgb, var(--_surface) 70%, transparent); font-size: 18px; color: var(--_text); flex-shrink: 0; }
    .sm-pie-empty-copy { display: flex; flex-direction: column; gap: 4px; max-width: 360px; }
    .sm-pie-empty-title { font-size: 15px; font-weight: 600; color: var(--_text); }
    .sm-pie-empty-hint { font-size: 12px; line-height: 1.45; }

    /* Breakdown */
    .sm-breakdown { gap: 8px; }
    .sm-table-head { display: grid; grid-template-columns: minmax(0,2fr) minmax(100px,1fr) 90px 110px; gap: 10px; padding: 0 8px 6px; font-size: 10px; text-transform: uppercase; letter-spacing: .12em; color: var(--_text-dim); border-bottom: 1px solid var(--_border-sub); }
    .sm-table-body { display: flex; flex-direction: column; gap: 1px; }
    .sm-file-types { gap: 8px; }
    .sm-type-table-head, .sm-type-row { grid-template-columns: minmax(0,2fr) 70px 90px 110px; }
    .sm-type-row { display: grid; gap: 10px; align-items: center; padding: 6px 8px; min-width: 0; border-radius: var(--vsc-border-radius-lg, 4px); }
    .sm-row { appearance: none; border: 1px solid transparent; border-radius: var(--vsc-border-radius-lg, 4px); display: grid; grid-template-columns: minmax(0,2fr) minmax(100px,1fr) 90px 110px; gap: 10px; align-items: center; padding: 6px 8px; cursor: pointer; color: var(--_text); background: transparent; text-align: left; transition: background .1s; width: 100%; min-width: 0; font-family: inherit; }
    .sm-row:hover { background: var(--_hover-bg); }
    .sm-row--active { background: color-mix(in srgb, var(--_focus) 10%, var(--_surface2) 90%); border-color: color-mix(in srgb, var(--_focus) 35%, transparent); }
    .sm-row-name { display: flex; align-items: center; gap: 8px; min-width: 0; }
    .sm-row-icon { flex-shrink: 0; font-size: 11px; color: var(--_icon-file); }
    .sm-row-icon--dir { color: var(--_icon-dir); }
    .sm-row-label { font-weight: 500; font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .sm-row-meta  { font-size: 11px; color: var(--_text-dim); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .sm-row-size  { font-size: 12px; font-variant-numeric: tabular-nums; white-space: nowrap; }
    .sm-row-pct-cell { display: flex; flex-direction: column; gap: 3px; align-items: flex-end; }
    .sm-row-pct   { font-size: 11px; font-variant-numeric: tabular-nums; color: var(--_text-dim); }
    .sm-row-track { width: 100%; height: 3px; border-radius: 999px; background: var(--_input-bg); border: 1px solid var(--_border-sub); overflow: hidden; }
    .sm-row-bar   { height: 100%; border-radius: inherit; background: linear-gradient(90deg, var(--_accent-a), var(--_accent-b)); transition: width .5s cubic-bezier(.4,0,.2,1); }

    .sm-align-r { text-align: right; }
    .sm-empty { display: flex; align-items: center; justify-content: center; min-height: 72px; font-size: 12px; color: var(--_text-dim); }

    @keyframes sm-pie-pulse {
      0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--_focus) 40%, transparent); }
      70% { box-shadow: 0 0 0 8px color-mix(in srgb, var(--_focus) 0%, transparent); }
      100% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--_focus) 0%, transparent); }
    }

    @media (max-width: 640px) {
      .sm-table-head { display: none; }
      .sm-row { grid-template-columns: minmax(0,1fr) auto; }
      .sm-type-row { grid-template-columns: minmax(0,1fr) auto; }
      .sm-row-meta { display: none; }
    }
  `],
})
export class SizeManagerSurfaceComponent implements OnChanges, OnDestroy {
  @Input({ required: true }) ctx!: SizeManagerSurfaceContext;
  @Output() firstEntryArrived = new EventEmitter<void>();

  loading = false;
  isTraversing = false;
  error: string | null = null;
  private scanStartedAt = 0;
  private tickInterval: ReturnType<typeof setInterval> | null = null;
  liveElapsedMs = 0;
  liveProgress: DiskUsageProgress | null = null;
  result: DiskUsageResult | null = null;
  entries: DiskUsageEntry[] = [];
  fileTypeRows: FileTypeBreakdownRow[] = [];
  fileTypeSourceLabel = 'Current folder';
  fileTypeSourceFileCount = 0;
  fileTypeSourceBytes = 0;
  selectedEntry: DiskUsageEntry | null = null;
  currentPath = '';
  parentPath: string | null = null;

  // ── Pie nav stack — owned here, not in the chart ──────────────────────────

  pieStack: PieFrame[] = [];

  get currentPieFrame(): PieFrame {
    return this.pieStack[this.pieStack.length - 1] ?? { label: '', sourceEntry: null, entries: [], totalSize: 0 };
  }

  private destroyed = false;
  private activeTabId = '';
  private activeScanRunId = 0;
  private syncedAnalysisPath = '';
  private readonly pieFrameCache = new Map<string, PieFrame>();
  private readonly pendingPieFrames = new Map<string, Promise<PieFrame | null>>();
  private activePieRequestId = 0;
  private piePrefetchRunId = 0;
  pieDistributionLoadingPath = '';

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['ctx']) {
      const tabId = this.ctx?.tab?.id ?? '';
      const path = this.ctx?.sizeVm?.analysisPath;
      const tabChanged = !!tabId && tabId !== this.activeTabId;
      if (tabChanged) {
        this.activeTabId = tabId;
        this.syncedAnalysisPath = '';
        this.resetAnalysisState();
      }
      if (path && (tabChanged || path !== this.syncedAnalysisPath)) {
        this.syncedAnalysisPath = path;
        this.analyse(path);
      }
    }
  }

  ngOnDestroy(): void { this.destroyed = true; this.stopTick(); }

  // ── Actions ───────────────────────────────────────────────────────────────

  close():   void { this.ctx?.sizeVm?.onClose(); }
  goUp():    void { if (this.parentPath) this.navigateToAnalysisPath(this.parentPath); }
  refresh(): void { if (this.currentPath) this.navigateToAnalysisPath(this.currentPath); }

  returnToRoot(): void {
    if (this.syncedAnalysisPath) this.navigateToAnalysisPath(this.syncedAnalysisPath);
  }

  onChartSelect(entry: DiskUsageEntry | null): void {
    this.selectEntry(entry);
  }

  async onChartActivate(entry: DiskUsageEntry): Promise<void> {
    if (!entry.isDir) {
      this.cdr.markForCheck();
      return;
    }

    this.selectEntry(entry);

    const currentFrame = this.currentPieFrame;
    if (currentFrame.path === entry.path) return;

    const cachedFrame = this.pieFrameCache.get(entry.path);
    if (cachedFrame) {
      this.pushPieFrame(cachedFrame);
      return;
    }

    const requestId = ++this.activePieRequestId;
    const analysisRoot = this.currentPath;
    this.pieDistributionLoadingPath = entry.path;
    this.cdr.markForCheck();

    try {
      const frame = await this.loadPieFrame(entry);
      if (this.destroyed || requestId !== this.activePieRequestId || this.currentPath !== analysisRoot) return;

      this.pieDistributionLoadingPath = '';
      if (frame) this.pushPieFrame(frame);
    } catch {
      if (this.destroyed || requestId !== this.activePieRequestId || this.currentPath !== analysisRoot) return;
      this.pieDistributionLoadingPath = '';
      this.cdr.markForCheck();
    }
  }

  onEntryClick(e: DiskUsageEntry): void {
    this.selectEntry(e);
    if (e.isDir) {
      void this.onChartActivate(e);
    } else {
      this.cdr.markForCheck();
    }
  }

  onEntryDblClick(e: DiskUsageEntry): void {
    if (e.isDir) this.navigateToAnalysisPath(e.path);
  }

  /** Chart emits bucketed Other entries — we push a new pie frame. */
  onOtherDrill(otherEntries: DiskUsageEntry[]): void {
    const depth = this.pieStack.length;
    const label = depth === 1 ? 'Other' : `Other (${depth})`;
    const totalSize = otherEntries.reduce((s, e) => s + e.size, 0);
    this.pieStack = [...this.pieStack, { label, sourceEntry: null, entries: otherEntries, totalSize }];
    this.selectEntry(null);
    this.cdr.markForCheck();
  }

  /** Jump to any frame in the pie breadcrumb trail. */
  pieGoTo(index: number): void {
    if (index >= this.pieStack.length - 1) return;
    this.pieStack = this.pieStack.slice(0, index + 1);
    this.selectEntry(this.currentPieFrame.sourceEntry ?? null);
    this.cdr.markForCheck();
  }

  private navigateToAnalysisPath(path: string): void {
    this.syncedAnalysisPath = path;
    this.ctx?.sizeVm?.onAnalyse(path);
    void this.analyse(path);
  }

  private selectEntry(entry: DiskUsageEntry | null): void {
    this.selectedEntry = entry;
    this.rebuildFileTypeBreakdown();
    this.cdr.markForCheck();
  }

  trackEntry(_: number, e: DiskUsageEntry): string { return e.path; }
  trackFileTypeRow(_: number, row: FileTypeBreakdownRow): string { return row.id; }

  // ── View helpers ──────────────────────────────────────────────────────────

  get headerPath(): string {
    return this.syncedAnalysisPath || this.currentPath;
  }

  get headerLabel(): string {
    return this.labelFromPath(this.headerPath) || this.headerPath || 'Disk Usage';
  }

  get showHeaderPath(): boolean {
    return this.normalizePathDisplay(this.headerLabel) !== this.normalizePathDisplay(this.headerPath);
  }

  get hasVisibleContent(): boolean {
    return !!this.result || this.entries.length > 0;
  }

  get scopeMetricTitle(): string {
    return 'Direct children';
  }

  get scopeMetricHint(): string {
    if (this.isTraversing) return `${this.entries.length.toLocaleString()} direct children loaded so far`;
    return `${this.entries.length.toLocaleString()} direct children in view`;
  }

  get directChildrenCount(): number {
    return this.entries.length;
  }

  get totalFootprintLabel(): string {
    return this.formatBytes(this.result?.totalSize ?? this.liveProgress?.totalSize ?? this.visibleEntriesSize);
  }

  get totalCompositionLabel(): string {
    const files = this.result?.totalFiles ?? this.liveProgress?.totalFiles ?? this.entries.reduce((sum, entry) => sum + (entry.isDir ? entry.files : 1), 0);
    const dirs = this.result?.totalDirs ?? this.liveProgress?.totalDirs ?? this.entries.reduce((sum, entry) => sum + (entry.isDir ? 1 + entry.dirs : 0), 0);
    return `${files.toLocaleString()} files · ${dirs.toLocaleString()} dirs`;
  }

  get headerProgressLabel(): string {
    if (!this.entries.length) return 'Loading entries...';
    return `Loading entries... ${this.entries.length} loaded`;
  }

  get liveElapsedLabel(): string {
    const ms = this.result?.elapsedMs ?? this.liveProgress?.elapsedMs ?? this.liveElapsedMs;
    if (ms < 1000) return `${ms} ms`;
    return `${(ms / 1000).toFixed(1)} s`;
  }

  get liveThroughputLabel(): string {
    const elapsedMs = this.result?.elapsedMs ?? this.liveProgress?.elapsedMs ?? this.liveElapsedMs;
    const totalSize = this.result?.totalSize ?? this.liveProgress?.totalSize ?? this.visibleEntriesSize;
    const secs = elapsedMs / 1000;
    if (secs < 0.5 || !totalSize) return 'scanning…';
    const bytesPerSec = totalSize / secs;
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const k = 1024;
    const i = bytesPerSec > 0 ? Math.min(Math.floor(Math.log(bytesPerSec) / Math.log(k)), units.length - 1) : 0;
    const val = bytesPerSec / Math.pow(k, i);
    const fmt = i === 0 ? Math.round(val).toString() : val.toFixed(1);
    return `${fmt} ${units[i]}/s`;
  }

  get liveTraversalSummary(): string {
    let dirs: number;
    let files: number;
    if (this.result) {
      dirs  = this.result.totalDirs;
      files = this.result.totalFiles;
    } else if (this.liveProgress) {
      dirs  = this.liveProgress.totalDirs;
      files = this.liveProgress.totalFiles;
    } else {
      dirs  = this.entries.reduce((s, e) => s + (e.isDir ? 1 + e.dirs  : 0), 0);
      files = this.entries.reduce((s, e) => s + (e.isDir ? e.files     : 1), 0);
    }
    const total = dirs + files;
    if (!total) return 'scanning…';
    return `${total.toLocaleString()} entries (${dirs.toLocaleString()} dirs · ${files.toLocaleString()} files)`;
  }

  get dominantFileTypeValue(): string {
    return this.rootDominantFileType?.name ?? 'Uncategorized';
  }

  get dominantFileTypeHint(): string {
    const topType = this.rootDominantFileType;
    if (topType) return `${topType.count.toLocaleString()} files · ${this.rootFileTypePct(topType)}`;
    if (this.totalFileCount === 0) return 'No files in this folder tree';
    return 'No matched categories in this folder tree';
  }

  get fileTypePanelHint(): string {
    return `${this.fileTypeSourceLabel} subtree · ${this.fileTypeSourceFileCount.toLocaleString()} files`;
  }

  get fileTypeEmptyLabel(): string {
    if (this.isTraversing && !this.fileTypeSourceFileCount) return 'Scanning file types…';
    if (this.fileTypeSourceFileCount === 0) return 'No files found in the selected subtree.';
    return 'No common file types matched the selected subtree.';
  }

  get breakdownEmptyLabel(): string {
    if (this.isTraversing && !this.entries.length) return 'Scanning folder…';
    return 'Folder is empty.';
  }

  get pieEmptyStateTitle(): string {
    if (this.isTraversing) return 'Scanning folder footprint';
    if (this.currentPieFrame.label && this.currentPieFrame.label !== 'Subtree') {
      return `${this.currentPieFrame.label} is empty`;
    }
    return 'This folder is empty';
  }

  get pieEmptyStateHint(): string {
    if (this.isTraversing) {
      return 'Distribution will appear as soon as files or child folders are discovered.';
    }
    return 'No files or child folders are available to visualize in this location.';
  }

  get pieDistributionLoadingLabel(): string {
    return this.labelFromPath(this.pieDistributionLoadingPath) || this.pieDistributionLoadingPath || 'selected folder';
  }

  barPercent(e: DiskUsageEntry): number {
    const totalSize = this.result?.totalSize ?? this.visibleEntriesSize;
    if (!totalSize) return 0;
    return Math.max(0.3, (e.size / totalSize) * 100);
  }

  pct(e: DiskUsageEntry): string {
    const totalSize = this.result?.totalSize ?? this.visibleEntriesSize;
    if (!totalSize) return '—';
    const p = (e.size / totalSize) * 100;
    return p < 0.1 ? '<0.1%' : p.toFixed(1) + '%';
  }

  fileTypeBarPercent(row: FileTypeBreakdownRow): number {
    if (!this.fileTypeSourceBytes) return 0;
    return Math.max(0.3, (row.totalSize / this.fileTypeSourceBytes) * 100);
  }

  fileTypePct(row: FileTypeBreakdownRow): string {
    if (!this.fileTypeSourceBytes) return '—';
    const pct = (row.totalSize / this.fileTypeSourceBytes) * 100;
    return pct < 0.1 ? '<0.1%' : pct.toFixed(1) + '%';
  }

  rootFileTypePct(row: FileTypeBreakdownRow): string {
    const totalSize = this.result?.totalSize ?? this.visibleEntriesSize;
    if (!totalSize) return '—';
    const pct = (row.totalSize / totalSize) * 100;
    return pct < 0.1 ? '<0.1%' : pct.toFixed(1) + '%';
  }

  formatBytes(bytes: number): string {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const k = 1024, i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), units.length - 1);
    const val = bytes / Math.pow(k, i);
    return (i === 0 ? val : val.toFixed(1)) + ' ' + units[i];
  }

  entryComposition(entry: DiskUsageEntry): string {
    return entry.isDir ? `${entry.files} files · ${entry.dirs} dirs` : 'File';
  }

  private get visibleEntriesSize(): number {
    return this.entries.reduce((sum, entry) => sum + entry.size, 0);
  }

  private isDiskUsageResult(item: DiskUsageEntry | DiskUsageProgress | DiskUsageResult): item is DiskUsageResult {
    return Array.isArray((item as DiskUsageResult).entries);
  }

  private isDiskUsageProgress(item: DiskUsageEntry | DiskUsageProgress | DiskUsageResult): item is DiskUsageProgress {
    return !this.isDiskUsageResult(item) && 'elapsedMs' in item && 'totalFiles' in item && !('path' in item);
  }

  private syncRootPieFrame(totalSize: number): void {
    const rootFrame: PieFrame = {
      label: 'Subtree',
      path: this.currentPath,
      sourceEntry: null,
      entries: this.entries,
      totalSize,
    };
    this.pieFrameCache.set(this.currentPath, rootFrame);
    if (this.pieStack.length <= 1) {
      this.pieStack = [rootFrame];
    }
  }

  private pushPieFrame(frame: PieFrame): void {
    const currentFrame = this.currentPieFrame;
    if (currentFrame.path === frame.path && currentFrame.entries === frame.entries && currentFrame.totalSize === frame.totalSize) {
      this.cdr.markForCheck();
      return;
    }

    const existingIndex = frame.path
      ? this.pieStack.findIndex((item) => item.path === frame.path)
      : -1;

    if (existingIndex >= 0) {
      this.pieStack = this.pieStack.slice(0, existingIndex + 1);
    } else {
      this.pieStack = [...this.pieStack, frame];
    }

    this.schedulePieFramePrefetch(frame.entries, frame.path);
    this.cdr.markForCheck();
  }

  private loadPieFrame(entry: DiskUsageEntry): Promise<PieFrame | null> {
    const cachedFrame = this.pieFrameCache.get(entry.path);
    if (cachedFrame) return Promise.resolve(cachedFrame);

    const pendingFrame = this.pendingPieFrames.get(entry.path);
    if (pendingFrame) return pendingFrame;

    const promise = this.api.analyzeDiskUsage(entry.path)
      .then((result) => {
        const frame: PieFrame = {
          label: entry.name || this.labelFromPath(entry.path) || 'Folder',
          path: entry.path,
          sourceEntry: entry,
          entries: result.entries ?? [],
          totalSize: result.totalSize ?? entry.size,
        };
        this.pieFrameCache.set(entry.path, frame);
        return frame;
      })
      .catch(() => null)
      .finally(() => {
        this.pendingPieFrames.delete(entry.path);
      });

    this.pendingPieFrames.set(entry.path, promise);
    return promise;
  }

  private schedulePieFramePrefetch(entries: DiskUsageEntry[], framePath?: string): void {
    if (this.isTraversing || !entries.length) return;

    const candidates = entries
      .filter((entry) => entry.isDir && entry.path !== framePath && !this.pieFrameCache.has(entry.path) && !this.pendingPieFrames.has(entry.path))
      .sort((left, right) => right.size - left.size)
      .slice(0, PIE_PREFETCH_LIMIT);

    if (!candidates.length) return;

    const runId = ++this.piePrefetchRunId;
    const analysisRoot = this.currentPath;
    void this.runPieFramePrefetch(candidates, runId, analysisRoot);
  }

  private async runPieFramePrefetch(entries: DiskUsageEntry[], runId: number, analysisRoot: string): Promise<void> {
    for (const entry of entries) {
      if (this.destroyed || runId !== this.piePrefetchRunId || this.currentPath !== analysisRoot) return;
      await this.loadPieFrame(entry);
    }
  }

  private get totalFileCount(): number {
    return this.result?.totalFiles ?? this.liveProgress?.totalFiles ?? this.entries.reduce((sum, entry) => sum + (entry.isDir ? entry.files : 1), 0);
  }

  private get rootDominantFileType(): FileTypeBreakdownRow | null {
    return this.buildFileTypeRows(this.result?.types ?? this.mergeEntryTypes(this.entries))[0] ?? null;
  }

  private buildFileTypeRows(types: Record<string, DiskUsageTypeStat>): FileTypeBreakdownRow[] {
    const rows = new Map<string, FileTypeBreakdownRow>();

    for (const [extension, stat] of Object.entries(types)) {
      const category = SIZE_MANAGER_CATEGORY_BY_EXTENSION.get(extension);
      if (!category) continue;

      const existing = rows.get(category.id);
      if (existing) {
        existing.count += stat.count;
        existing.totalSize += stat.size;
      } else {
        rows.set(category.id, {
          id: category.id,
          name: category.name,
          icon: category.icon,
          count: stat.count,
          totalSize: stat.size,
        });
      }
    }

    return Array.from(rows.values()).sort((a, b) => {
      if (b.totalSize !== a.totalSize) return b.totalSize - a.totalSize;
      if (b.count !== a.count) return b.count - a.count;
      return a.name.localeCompare(b.name);
    });
  }

  private rebuildFileTypeBreakdown(): void {
    const source = this.getFileTypeSource();

    this.fileTypeSourceLabel = source.label;
    this.fileTypeSourceFileCount = source.fileCount;
    this.fileTypeSourceBytes = source.totalSize;
    this.fileTypeRows = this.buildFileTypeRows(source.types);
  }

  private getFileTypeSource(): FileTypeBreakdownSource {
    const selected = this.selectedEntry;
    if (selected) {
      return {
        label: this.labelFromPath(selected.path) || selected.name || 'Selected item',
        fileCount: selected.isDir ? selected.files : 1,
        totalSize: selected.size,
        types: selected.types ?? {},
      };
    }

    const rootTypes = this.result?.types ?? this.mergeEntryTypes(this.entries);
    const rootFileCount = this.result?.totalFiles ?? this.liveProgress?.totalFiles ?? this.entries.reduce((sum, entry) => sum + (entry.isDir ? entry.files : 1), 0);
    const rootSize = this.result?.totalSize ?? this.liveProgress?.totalSize ?? this.visibleEntriesSize;
    return {
      label: this.headerLabel || 'Current folder',
      fileCount: rootFileCount,
      totalSize: rootSize,
      types: rootTypes,
    };
  }

  private mergeEntryTypes(entries: DiskUsageEntry[]): Record<string, DiskUsageTypeStat> {
    const merged: Record<string, DiskUsageTypeStat> = {};
    for (const entry of entries) {
      for (const [extension, stat] of Object.entries(entry.types ?? {})) {
        const current = merged[extension] ?? { count: 0, size: 0 };
        merged[extension] = {
          count: current.count + stat.count,
          size: current.size + stat.size,
        };
      }
    }
    return merged;
  }

  private syncSelectedEntry(): void {
    if (!this.selectedEntry) return;
    const match = this.entries.find((entry) => entry.path === this.selectedEntry?.path)
      ?? this.currentPieFrame.entries.find((entry) => entry.path === this.selectedEntry?.path)
      ?? null;
    this.selectedEntry = match;
  }

  // ── Core analysis ─────────────────────────────────────────────────────────

  private resetAnalysisState(): void {
    this.stopTick();
    this.loading = false;
    this.isTraversing = false;
    this.error = null;
    this.liveElapsedMs = 0;
    this.liveProgress = null;
    this.result = null;
    this.entries = [];
    this.fileTypeRows = [];
    this.fileTypeSourceLabel = 'Current folder';
    this.fileTypeSourceFileCount = 0;
    this.fileTypeSourceBytes = 0;
    this.selectedEntry = null;
    this.currentPath = '';
    this.parentPath = null;
    this.pieStack = [];
    this.pieFrameCache.clear();
    this.pendingPieFrames.clear();
    this.activePieRequestId++;
    this.piePrefetchRunId++;
    this.pieDistributionLoadingPath = '';
  }

  async analyse(path: string): Promise<void> {
    const scanRunId = ++this.activeScanRunId;
    this.currentPath = path;
    this.parentPath  = this.getScopedParent(path);
    this.loading     = true;
    this.isTraversing = true;
    this.error       = null;
    this.liveProgress = null;
    this.result      = null;
    this.entries     = [];
    this.fileTypeRows = [];
    this.fileTypeSourceLabel = 'Current folder';
    this.fileTypeSourceFileCount = 0;
    this.fileTypeSourceBytes = 0;
    this.selectedEntry = null;
    this.pieStack    = [];
    this.pieFrameCache.clear();
    this.pendingPieFrames.clear();
    this.activePieRequestId++;
    this.piePrefetchRunId++;
    this.pieDistributionLoadingPath = '';
    this.startTick();
    this.cdr.markForCheck();

    let firstEntryArrived = false;
    try {
      for await (const item of this.api.streamDiskUsage(path)) {
        if (this.destroyed || this.activeScanRunId !== scanRunId || this.currentPath !== path) break;
        if (this.isDiskUsageResult(item)) {
          this.liveProgress = {
            totalSize: item.totalSize,
            totalFiles: item.totalFiles,
            totalDirs: item.totalDirs,
            elapsedMs: item.elapsedMs,
          };
          this.result = item;
          this.entries = item.entries?.length ? item.entries : this.entries;
          this.syncRootPieFrame(item.totalSize);
          this.syncSelectedEntry();
          this.rebuildFileTypeBreakdown();
          if (!firstEntryArrived) {
            firstEntryArrived = true;
            this.firstEntryArrived.emit();
          }
          this.cdr.markForCheck();
        } else if (this.isDiskUsageProgress(item)) {
          this.liveProgress = item;
          this.cdr.markForCheck();
        } else {
          const existingIndex = this.entries.findIndex((entry) => entry.path === item.path);
          if (existingIndex >= 0) {
            const nextEntries = [...this.entries];
            nextEntries[existingIndex] = item;
            this.entries = nextEntries;
          } else {
            this.entries = [...this.entries, item];
          }
          this.syncRootPieFrame(this.visibleEntriesSize);
          this.syncSelectedEntry();
          this.rebuildFileTypeBreakdown();
          if (!firstEntryArrived) {
            firstEntryArrived = true;
            this.firstEntryArrived.emit();
          }
          this.cdr.markForCheck();
        }
      }
    } catch (err: any) {
      if (this.destroyed || this.activeScanRunId !== scanRunId || this.currentPath !== path) return;
      this.error   = err?.message || String(err);
      this.liveProgress = null;
      this.result  = null;
      this.entries = [];
      this.fileTypeRows = [];
      this.fileTypeSourceLabel = 'Current folder';
      this.fileTypeSourceFileCount = 0;
      this.fileTypeSourceBytes = 0;
      this.pieStack = [];
      this.pieDistributionLoadingPath = '';
    } finally {
      if (this.destroyed || this.activeScanRunId !== scanRunId || this.currentPath !== path) return;
      this.loading = false;
      this.isTraversing = false;
      this.stopTick();
      this.schedulePieFramePrefetch(this.currentPieFrame.entries, this.currentPieFrame.path);
      this.cdr.markForCheck();
    }
  }

  private startTick(): void {
    this.stopTick();
    this.scanStartedAt = Date.now();
    this.liveElapsedMs = 0;
    this.tickInterval = setInterval(() => {
      this.liveElapsedMs = Date.now() - this.scanStartedAt;
      this.cdr.markForCheck();
    }, 100);
  }

  private stopTick(): void {
    if (this.tickInterval !== null) {
      clearInterval(this.tickInterval);
      this.tickInterval = null;
    }
  }

  private getParent(path: string): string | null {
    const normalized = normalizePath(path || '');
    if (!normalized || normalized === '/') return null;
    const parent = parentPath(normalized);
    return parent === normalized ? null : parent;
  }

  private getScopedParent(path: string): string | null {
    const parent = this.getParent(path);
    if (!parent) return null;

    const root = this.normalizePathDisplay(this.syncedAnalysisPath || '');
    if (!root) return parent;

    const current = this.normalizePathDisplay(path);
    if (current === root) return null;

    const normalizedParent = this.normalizePathDisplay(parent);
    if (normalizedParent === root) return parent;
    if (normalizedParent.startsWith(root + '/')) return parent;
    return null;
  }

  private labelFromPath(path: string): string {
    const normalized = normalizePath(path || '');
    if (!normalized || normalized === '/') return '';
    const segments = buildSegments(normalized);
    return segments[segments.length - 1]?.label || normalized;
  }

  private normalizePathDisplay(path: string): string {
    return normalizePath(path || '');
  }
}