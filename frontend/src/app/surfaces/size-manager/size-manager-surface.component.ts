import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  Input,
  OnChanges,
  OnDestroy,
  SimpleChanges,
} from '@angular/core';
import { ApiService } from '../../services/api.service';
import type { DiskUsageEntry, DiskUsageResult } from '../../shared/protocol-enhanced';
import type { SizeManagerSurfaceContext } from '../surface-context';

@Component({
  selector: 'app-size-manager-surface',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <!-- ── Header bar ─────────────────────────────────────── -->
    <div class="sm-header">
      <div class="sm-header-left">
        <span class="sm-kicker">Disk Usage</span>
        <span class="sm-sep">·</span>
        <span class="sm-current-label">{{ currentLabel }}</span>
        <span class="sm-path" [title]="currentPath">{{ currentPath }}</span>
      </div>
      <div class="sm-header-actions">
        <button class="sm-btn" [disabled]="!parentPath" (click)="goUp()" title="Go to parent">↑ Parent</button>
        <button class="sm-btn" (click)="refresh()">↺ Refresh</button>
        <button class="sm-btn sm-btn--close" (click)="close()" title="Close">✕</button>
      </div>
    </div>

    <!-- ── Loading ────────────────────────────────────────── -->
    <div class="sm-loading" *ngIf="loading">
      <div class="sm-spinner"></div>
      <span>Scanning <strong>{{ currentLabel }}</strong>…</span>
    </div>

    <!-- ── Error ──────────────────────────────────────────── -->
    <div class="sm-error" *ngIf="error && !loading">
      <span class="sm-error-icon">⚠</span>
      <span>{{ error }}</span>
      <button class="sm-btn sm-btn--primary" (click)="refresh()">Retry</button>
    </div>

    <!-- ── Dashboard ──────────────────────────────────────── -->
    <div class="sm-body" *ngIf="result && !loading">

      <!-- Metric strip -->
      <div class="sm-metrics">
        <div class="sm-metric">
          <div class="sm-metric-val">{{ formatBytes(result.totalSize) }}</div>
          <div class="sm-metric-lbl">Total size</div>
        </div>
        <div class="sm-metric">
          <div class="sm-metric-val">{{ totalItems | number }}</div>
          <div class="sm-metric-lbl">{{ result.totalFiles | number }} files · {{ result.totalDirs | number }} dirs</div>
        </div>
        <div class="sm-metric">
          <div class="sm-metric-val">{{ averageEntrySizeLabel }}</div>
          <div class="sm-metric-lbl">Avg child size</div>
        </div>
        <div class="sm-metric">
          <div class="sm-metric-val">{{ result.elapsedMs }} ms</div>
          <div class="sm-metric-lbl">{{ directoryMixLabel }}</div>
        </div>
      </div>

      <!-- Two-col: spotlight + ranking -->
      <div class="sm-cols">

        <!-- Spotlight -->
        <div class="sm-panel sm-spotlight">
          <div class="sm-panel-head">
            <span class="sm-panel-title">Spotlight</span>
            <span class="sm-pill" [class.sm-pill--dir]="selectedEntry.isDir" *ngIf="selectedEntry">
              {{ selectedEntry.isDir ? 'Folder' : 'File' }}
            </span>
          </div>

          <ng-container *ngIf="selectedEntry; else noSpotlight">
            <div class="sm-spotlight-name" [title]="selectedEntry.path">{{ selectedEntry.name }}</div>
            <div class="sm-spotlight-size">{{ formatBytes(selectedEntry.size) }}</div>
            <div class="sm-spotlight-share">{{ pct(selectedEntry) }} of current location</div>

            <!-- Animated bar -->
            <div class="sm-bar-track">
              <div class="sm-bar-fill" [style.width.%]="animatedBarPct" [style.transition]="barTransition"></div>
            </div>

            <div class="sm-insight-row">
              <div class="sm-insight">
                <div class="sm-insight-val">{{ selectedEntry.files | number }}</div>
                <div class="sm-insight-lbl">Files</div>
              </div>
              <div class="sm-insight">
                <div class="sm-insight-val">{{ selectedEntry.dirs | number }}</div>
                <div class="sm-insight-lbl">Folders</div>
              </div>
              <div class="sm-insight sm-insight--path">
                <div class="sm-insight-val sm-insight-path-val" [title]="selectedEntry.path">{{ selectedEntry.path }}</div>
                <div class="sm-insight-lbl">Full path</div>
              </div>
            </div>
          </ng-container>

          <ng-template #noSpotlight>
            <div class="sm-empty">No entries here.</div>
          </ng-template>
        </div>

        <!-- Top 5 ranking -->
        <div class="sm-panel sm-ranking">
          <div class="sm-panel-head">
            <span class="sm-panel-title">Top consumers</span>
          </div>

          <div class="sm-ranking-list" *ngIf="topEntries.length > 0; else noRanking">
            <button
              class="sm-rank-item"
              *ngFor="let e of topEntries; let i = index; trackBy: trackEntry"
              [class.sm-rank-item--active]="selectedEntry?.path === e.path"
              (click)="onEntryClick(e)"
              (dblclick)="onEntryDblClick(e)">

              <span class="sm-rank-num">{{ i + 1 }}</span>

              <div class="sm-rank-info">
                <div class="sm-rank-name" [title]="e.path">{{ e.name }}</div>

                <!-- Inline animated mini-bar -->
                <div class="sm-mini-track">
                  <div class="sm-mini-fill"
                    [class.sm-mini-fill--active]="selectedEntry?.path === e.path"
                    [style.width]="getMiniBarWidth(e)"></div>
                </div>
              </div>

              <div class="sm-rank-right">
                <div class="sm-rank-size">{{ formatBytes(e.size) }}</div>
                <div class="sm-rank-pct">{{ pct(e) }}</div>
              </div>
            </button>
          </div>

          <ng-template #noRanking>
            <div class="sm-empty">Nothing to rank.</div>
          </ng-template>
        </div>
      </div>

      <!-- ── Breakdown table ──────────────────────────────── -->
      <div class="sm-panel sm-breakdown">
        <div class="sm-panel-head">
          <span class="sm-panel-title">Breakdown</span>
          <span class="sm-panel-hint">Single-click to inspect · double-click to drill into folders</span>
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
            (click)="onEntryClick(e)"
            (dblclick)="onEntryDblClick(e)">

            <div class="sm-row-name">
              <span class="sm-row-icon" [class.sm-row-icon--dir]="e.isDir">{{ e.isDir ? '▣' : '•' }}</span>
              <span class="sm-row-label" [title]="e.path">{{ e.name }}</span>
            </div>

            <span class="sm-row-meta">{{ entryComposition(e) }}</span>
            <span class="sm-row-size sm-align-r">{{ formatBytes(e.size) }}</span>

            <!-- Pct + inline bar -->
            <div class="sm-row-pct-cell sm-align-r">
              <span class="sm-row-pct">{{ pct(e) }}</span>
              <div class="sm-row-track">
                <div class="sm-row-bar" [style.width.%]="barPercent(e)"></div>
              </div>
            </div>
          </button>
        </div>

        <ng-template #emptyBreakdown>
          <div class="sm-empty">Folder is empty.</div>
        </ng-template>
      </div>
    </div>
  `,
  styles: [`
    /*
     * All colours are driven by VS Code CSS tokens so the component
     * automatically adapts when the host applies .vscode-light or the
     * default dark theme.  No hardcoded hex values except for the two
     * accent-gradient stops that have explicit light/dark overrides.
     */

    /* ── Token aliases ───────────────────────────────────── *
     * Map the handful of tokens this component actually needs so that
     * we only fall back in one place and the rest of the rules stay clean.
     */
    :host {
      /* Surfaces */
      --_bg:        var(--vsc-editor-background,   #1e1e1e);
      --_surface:   var(--vsc-panel-background,    #252526);
      --_surface2:  var(--vsc-list-hover-background, #2a2d2e);
      --_input-bg:  var(--vsc-input-background,    #3c3c3c);

      /* Text */
      --_text:      var(--vsc-foreground,          #cccccc);
      --_text-dim:  var(--vsc-foreground-dim,      #888888);

      /* Borders */
      --_border:    var(--vsc-border,              #454545);
      --_border-sub: var(--vsc-border-subtle,      #3a3a3a);

      /* Interactive */
      --_focus:     var(--vsc-input-focus-border,  #007acc);
      --_btn-bg:    var(--vsc-button-background,   #0e639c);
      --_btn-hover: var(--vsc-button-hover,        #1177bb);
      --_hover-bg:  var(--vsc-list-hover-background, #2a2d2e);
      --_active-bg: var(--vsc-list-active-background, #094771);

      /* Accent — gradient stops, overridden for light below */
      --_accent-a: #2ec4b6;
      --_accent-b: #3a86ff;

      /* Status */
      --_error:     var(--vsc-error,               #f48771);
      --_blue:      var(--vsc-accent-blue,         #007acc);

      /* File / folder icon tints – dark defaults */
      --_icon-file:   #8fd3ff;
      --_icon-dir:    #dcb67a;
      --_pill-file-text: #9bdcff;
      --_pill-file-bg:   rgba(19, 108, 145, .18);
      --_pill-dir-text:  #6aaa2a;
      --_pill-dir-bg:    rgba(106, 142, 35, .15);
      --_rank-badge-bg:  rgba(58, 134, 255, .14);

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

    /* ── Light-theme token overrides ─────────────────────── *
     * The host applies class="vscode-light" which Angular's :host-context
     * can read.  We re-point just the tokens that differ meaningfully in
     * light mode so every rule below stays theme-agnostic.
     */
    :host-context(.vscode-light) {
      --_bg:        var(--vsc-editor-background,   #ffffff);
      --_surface:   var(--vsc-panel-background,    #f3f3f3);
      --_surface2:  var(--vsc-list-hover-background, #e4e6f1);
      --_input-bg:  var(--vsc-input-background,    #ffffff);
      --_text:      var(--vsc-foreground,          #3b3b3b);
      --_text-dim:  var(--vsc-foreground-dim,      #737373);
      --_border:    var(--vsc-border,              #c4c4c4);
      --_border-sub: var(--vsc-border-subtle,      #e5e5e5);
      --_focus:     var(--vsc-input-focus-border,  #007acc);
      --_btn-bg:    var(--vsc-button-background,   #007acc);
      --_btn-hover: var(--vsc-button-hover,        #005ba1);
      --_hover-bg:  var(--vsc-list-hover-background, #e4e6f1);
      --_active-bg: var(--vsc-list-active-background, #cce0f5);
      --_error:     var(--vsc-error,               #c7372f);
      --_blue:      var(--vsc-accent-blue,         #007acc);

      /* Slightly muted accent gradient — still visible on white */
      --_accent-a: #0fa89e;
      --_accent-b: #1a6fd4;

      /* Icons/pills need darker tints on white backgrounds */
      --_icon-file:   #1a6fd4;
      --_icon-dir:    #a0700a;
      --_pill-file-text: #0a5d8f;
      --_pill-file-bg:   rgba(0, 122, 204, .10);
      --_pill-dir-text:  #5a6e00;
      --_pill-dir-bg:    rgba(90, 110, 0, .10);
      --_rank-badge-bg:  rgba(0, 122, 204, .10);
    }

    /* ── Header ─────────────────────────────────────────── */

    .sm-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      padding: 8px 14px;
      flex-shrink: 0;
      border-bottom: 1px solid var(--_border-sub);
      background: var(--_surface);
    }

    .sm-header-left {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
      overflow: hidden;
    }

    .sm-kicker {
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: .14em;
      color: var(--_text-dim);
      flex-shrink: 0;
    }

    .sm-sep { color: var(--_text-dim); flex-shrink: 0; }

    .sm-current-label {
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      flex-shrink: 0;
      max-width: 200px;
    }

    .sm-path {
      font-size: 11px;
      color: var(--_text-dim);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      min-width: 0;
    }

    .sm-header-actions {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-shrink: 0;
    }

    /* ── Buttons ─────────────────────────────────────────── */

    .sm-btn {
      appearance: none;
      border: 1px solid var(--_border);
      border-radius: var(--vsc-border-radius-lg, 4px);
      padding: 4px 10px;
      font-size: 12px;
      font-weight: 500;
      font-family: inherit;
      cursor: pointer;
      color: var(--_text);
      background: transparent;
      transition:
        background var(--vsc-transition-fast, .1s ease),
        border-color var(--vsc-transition-fast, .1s ease);
      white-space: nowrap;
    }
    .sm-btn:hover:not(:disabled) { background: var(--_hover-bg); border-color: var(--_border); }
    .sm-btn:disabled { opacity: .35; cursor: default; }

    .sm-btn--primary {
      background: var(--_btn-bg);
      color: #fff;
      border-color: transparent;
    }
    .sm-btn--primary:hover:not(:disabled) { background: var(--_btn-hover); }

    .sm-btn--close { padding: 4px 8px; }

    /* ── Loading ─────────────────────────────────────────── */

    .sm-loading {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 16px 14px;
      flex-shrink: 0;
      color: var(--_text-dim);
    }

    .sm-spinner {
      width: 14px;
      height: 14px;
      border: 2px solid var(--_border);
      border-top-color: var(--_blue);
      border-radius: 50%;
      animation: sm-spin .6s linear infinite;
      flex-shrink: 0;
    }
    @keyframes sm-spin { to { transform: rotate(360deg); } }

    /* ── Error ───────────────────────────────────────────── */

    .sm-error {
      display: flex;
      align-items: center;
      gap: 10px;
      padding: 12px 14px;
      flex-shrink: 0;
      color: var(--_error);
      border-bottom: 1px solid color-mix(in srgb, var(--_error) 25%, transparent);
      background: color-mix(in srgb, var(--_error) 8%, var(--_surface));
    }
    .sm-error-icon { font-size: 15px; flex-shrink: 0; }

    /* ── Body scroll container ───────────────────────────── */

    .sm-body {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: auto;
      padding: 12px 14px;
      gap: 12px;
      scrollbar-width: thin;
      scrollbar-color: var(--vsc-scrollbar-thumb, #424242) transparent;
    }

    /* ── Metric strip ────────────────────────────────────── */

    .sm-metrics {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
      gap: 8px;
      flex-shrink: 0;
    }

    .sm-metric {
      padding: 10px 12px;
      border-radius: var(--vsc-border-radius-lg, 4px);
      border: 1px solid var(--_border-sub);
      background: var(--_surface);
    }

    .sm-metric-val {
      font-size: 18px;
      font-weight: 700;
      letter-spacing: -.03em;
      font-variant-numeric: tabular-nums;
      line-height: 1.1;
      color: var(--_text);
    }

    .sm-metric-lbl {
      margin-top: 3px;
      font-size: 11px;
      color: var(--_text-dim);
    }

    /* ── Two-col layout ──────────────────────────────────── */

    .sm-cols {
      display: grid;
      grid-template-columns: minmax(0, 1.2fr) minmax(0, 0.9fr);
      gap: 12px;
      flex-shrink: 0;
    }

    /* ── Panel base ──────────────────────────────────────── */

    .sm-panel {
      display: flex;
      flex-direction: column;
      gap: 10px;
      padding: 12px 14px;
      border-radius: var(--vsc-border-radius-lg, 4px);
      border: 1px solid var(--_border-sub);
      background: var(--_surface);
      min-width: 0;
    }

    .sm-panel-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
    }

    .sm-panel-title {
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: .12em;
      color: var(--_text-dim);
    }

    .sm-panel-hint {
      font-size: 11px;
      color: var(--_text-dim);
    }

    /* ── Pills ───────────────────────────────────────────── */

    .sm-pill {
      font-size: 10px;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: .08em;
      padding: 2px 7px;
      border-radius: 999px;
      color: var(--_pill-file-text);
      background: var(--_pill-file-bg);
    }
    .sm-pill--dir {
      color: var(--_pill-dir-text);
      background: var(--_pill-dir-bg);
    }

    /* ── Spotlight ───────────────────────────────────────── */

    .sm-spotlight-name {
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      color: var(--_text);
    }

    .sm-spotlight-size {
      font-size: 28px;
      font-weight: 700;
      letter-spacing: -.05em;
      font-variant-numeric: tabular-nums;
      line-height: 1;
      color: var(--_text);
    }

    .sm-spotlight-share {
      font-size: 11px;
      color: var(--_text-dim);
    }

    /* ── Animated spotlight bar ──────────────────────────── */

    .sm-bar-track {
      height: 5px;
      border-radius: 999px;
      background: var(--_input-bg);
      border: 1px solid var(--_border-sub);
      overflow: hidden;
    }

    .sm-bar-fill {
      height: 100%;
      border-radius: inherit;
      background: linear-gradient(90deg, var(--_accent-a), var(--_accent-b));
      width: 0%;
    }

    /* ── Insight boxes ───────────────────────────────────── */

    .sm-insight-row {
      display: grid;
      grid-template-columns: 1fr 1fr 2fr;
      gap: 8px;
    }

    .sm-insight {
      padding: 8px 10px;
      border-radius: var(--vsc-border-radius-lg, 4px);
      border: 1px solid var(--_border-sub);
      background: var(--_bg);
      min-width: 0;
    }

    .sm-insight-val {
      font-size: 14px;
      font-weight: 600;
      font-variant-numeric: tabular-nums;
      color: var(--_text);
    }

    .sm-insight-lbl {
      font-size: 10px;
      margin-top: 2px;
      color: var(--_text-dim);
    }

    .sm-insight-path-val {
      font-size: 11px;
      font-weight: 400;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    /* ── Ranking list ────────────────────────────────────── */

    .sm-ranking-list {
      display: flex;
      flex-direction: column;
      gap: 3px;
    }

    .sm-rank-item {
      appearance: none;
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius-lg, 4px);
      padding: 7px 10px;
      display: flex;
      align-items: center;
      gap: 10px;
      cursor: pointer;
      color: var(--_text);
      background: transparent;
      text-align: left;
      transition: background var(--vsc-transition-fast, .1s ease);
      width: 100%;
      min-width: 0;
      font-family: inherit;
    }
    .sm-rank-item:hover { background: var(--_hover-bg); }
    .sm-rank-item--active {
      background: color-mix(in srgb, var(--_focus) 10%, var(--_surface2) 90%);
      border-color: color-mix(in srgb, var(--_focus) 40%, transparent);
    }

    .sm-rank-num {
      width: 20px;
      height: 20px;
      flex-shrink: 0;
      display: grid;
      place-items: center;
      border-radius: 50%;
      font-size: 10px;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      background: var(--_rank-badge-bg);
      color: var(--_blue);
    }

    .sm-rank-info {
      display: flex;
      flex-direction: column;
      gap: 5px;
      flex: 1 1 auto;
      min-width: 0;
    }

    .sm-rank-name {
      font-weight: 600;
      font-size: 12px;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      color: var(--_text);
    }

    /* ── Mini bar ────────────────────────────────────────── */

    .sm-mini-track {
      height: 3px;
      border-radius: 999px;
      background: var(--_input-bg);
      border: 1px solid var(--_border-sub);
      overflow: hidden;
    }

    .sm-mini-fill {
      height: 100%;
      border-radius: inherit;
      background: color-mix(in srgb, var(--_blue) 55%, transparent);
      transition: width .6s cubic-bezier(.4, 0, .2, 1);
    }
    .sm-mini-fill--active {
      background: var(--_accent-b);
    }

    .sm-rank-right {
      display: flex;
      flex-direction: column;
      align-items: flex-end;
      flex-shrink: 0;
      gap: 2px;
    }

    .sm-rank-size {
      font-size: 12px;
      font-weight: 600;
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
      color: var(--_text);
    }

    .sm-rank-pct {
      font-size: 10px;
      color: var(--_text-dim);
      font-variant-numeric: tabular-nums;
    }

    /* ── Breakdown table ─────────────────────────────────── */

    .sm-breakdown { gap: 8px; }

    .sm-table-head {
      display: grid;
      grid-template-columns: minmax(0, 2fr) minmax(100px, 1fr) 90px 110px;
      gap: 10px;
      padding: 0 8px 6px;
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: .12em;
      color: var(--_text-dim);
      border-bottom: 1px solid var(--_border-sub);
    }

    .sm-table-body {
      display: flex;
      flex-direction: column;
      gap: 1px;
    }

    .sm-row {
      appearance: none;
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius-lg, 4px);
      display: grid;
      grid-template-columns: minmax(0, 2fr) minmax(100px, 1fr) 90px 110px;
      gap: 10px;
      align-items: center;
      padding: 6px 8px;
      cursor: pointer;
      color: var(--_text);
      background: transparent;
      text-align: left;
      transition: background var(--vsc-transition-fast, .1s ease);
      width: 100%;
      min-width: 0;
      font-family: inherit;
    }
    .sm-row:hover { background: var(--_hover-bg); }
    .sm-row--active {
      background: color-mix(in srgb, var(--_focus) 10%, var(--_surface2) 90%);
      border-color: color-mix(in srgb, var(--_focus) 35%, transparent);
    }

    .sm-row-name {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
    }

    .sm-row-icon {
      flex-shrink: 0;
      font-size: 11px;
      color: var(--_icon-file);
    }
    .sm-row-icon--dir { color: var(--_icon-dir); }

    .sm-row-label {
      font-weight: 500;
      font-size: 12px;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .sm-row-meta {
      font-size: 11px;
      color: var(--_text-dim);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .sm-row-size {
      font-size: 12px;
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
      color: var(--_text);
    }

    .sm-row-pct-cell {
      display: flex;
      flex-direction: column;
      gap: 3px;
      align-items: flex-end;
    }

    .sm-row-pct {
      font-size: 11px;
      font-variant-numeric: tabular-nums;
      color: var(--_text-dim);
    }

    .sm-row-track {
      width: 100%;
      height: 3px;
      border-radius: 999px;
      background: var(--_input-bg);
      border: 1px solid var(--_border-sub);
      overflow: hidden;
    }

    .sm-row-bar {
      height: 100%;
      border-radius: inherit;
      background: linear-gradient(90deg, var(--_accent-a), var(--_accent-b));
      transition: width .5s cubic-bezier(.4, 0, .2, 1);
    }

    /* ── Shared ──────────────────────────────────────────── */

    .sm-align-r { text-align: right; }

    .sm-empty {
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 72px;
      font-size: 12px;
      color: var(--_text-dim);
    }

    /* ── Responsive ──────────────────────────────────────── */

    @media (max-width: 900px) {
      .sm-cols { grid-template-columns: 1fr; }
      .sm-table-head,
      .sm-row { grid-template-columns: minmax(0, 1.8fr) minmax(80px, 1fr) 80px 90px; }
    }

    @media (max-width: 640px) {
      .sm-table-head { display: none; }
      .sm-row { grid-template-columns: minmax(0, 1fr) auto; }
      .sm-row-meta { display: none; }
      .sm-insight-row { grid-template-columns: 1fr 1fr; }
      .sm-insight--path { display: none; }
    }
  `],
})
export class SizeManagerSurfaceComponent implements OnChanges, OnDestroy {
  @Input({ required: true }) ctx!: SizeManagerSurfaceContext;

  loading = false;
  error: string | null = null;
  result: DiskUsageResult | null = null;
  entries: DiskUsageEntry[] = [];
  topEntries: DiskUsageEntry[] = [];
  selectedEntry: DiskUsageEntry | null = null;
  currentPath = '';
  parentPath: string | null = null;

  // Animated bar state
  animatedBarPct = 0;
  barTransition = 'none';

  private destroyed = false;

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['ctx']) {
      const path = this.ctx?.sizeVm?.analysisPath;
      if (path && path !== this.currentPath) {
        this.analyse(path);
      }
    }
  }

  ngOnDestroy(): void {
    this.destroyed = true;
  }

  // ── Actions ──────────────────────────────────────────────────────────────

  close(): void { this.ctx?.sizeVm?.onClose(); }

  goUp(): void {
    if (this.parentPath) this.analyse(this.parentPath);
  }

  refresh(): void {
    if (this.currentPath) this.analyse(this.currentPath);
  }

  onEntryClick(e: DiskUsageEntry): void {
    this.selectedEntry = e;
    this.triggerBarAnimation(e);
  }

  onEntryDblClick(e: DiskUsageEntry): void {
    if (e.isDir) this.analyse(e.path);
  }

  trackEntry(_: number, e: DiskUsageEntry): string { return e.path; }

  // ── View helpers ─────────────────────────────────────────────────────────

  get currentLabel(): string {
    return this.labelFromPath(this.currentPath) || this.currentPath || 'Disk Usage';
  }

  get totalItems(): number {
    if (!this.result) return 0;
    return this.result.totalFiles + this.result.totalDirs;
  }

  get averageEntrySizeLabel(): string {
    if (this.entries.length === 0 || !this.result) return '0 B';
    return this.formatBytes(this.result.totalSize / this.entries.length);
  }

  get directoryMixLabel(): string {
    const total = this.entries.length;
    if (total === 0) return 'No children';
    const folders = this.entries.filter(e => e.isDir).length;
    const share = (folders / total) * 100;
    return share < 0.1 ? 'Mostly files' : `${share.toFixed(0)}% folders`;
  }

  barPercent(e: DiskUsageEntry): number {
    if (!this.result || this.result.totalSize === 0) return 0;
    return Math.max(0.3, (e.size / this.result.totalSize) * 100);
  }

  /** Returns a CSS width string for the animated mini-bar in the ranking list. */
  getMiniBarWidth(e: DiskUsageEntry): string {
    return this.barPercent(e).toFixed(1) + '%';
  }

  pct(e: DiskUsageEntry): string {
    if (!this.result || this.result.totalSize === 0) return '—';
    const p = (e.size / this.result.totalSize) * 100;
    return p < 0.1 ? '<0.1%' : p.toFixed(1) + '%';
  }

  formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const k = 1024;
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), units.length - 1);
    const val = bytes / Math.pow(k, i);
    return (i === 0 ? val : val.toFixed(1)) + ' ' + units[i];
  }

  entryComposition(entry: DiskUsageEntry): string {
    return entry.isDir ? `${entry.files} files · ${entry.dirs} dirs` : 'File';
  }

  // ── Spotlight bar animation ───────────────────────────────────────────────

  private triggerBarAnimation(e: DiskUsageEntry): void {
    // Reset to 0 instantly, then animate to target on next frame
    this.barTransition = 'none';
    this.animatedBarPct = 0;
    this.cdr.markForCheck();

    requestAnimationFrame(() => {
      this.barTransition = 'width .55s cubic-bezier(.4,0,.2,1)';
      this.animatedBarPct = this.barPercent(e);
      this.cdr.markForCheck();
    });
  }

  // ── Core analysis ────────────────────────────────────────────────────────

  private async analyse(path: string): Promise<void> {
    this.ctx?.sizeVm?.onAnalyse(path);
    this.currentPath = path;
    this.parentPath = this.getParent(path);
    this.loading = true;
    this.error = null;
    this.animatedBarPct = 0;
    this.barTransition = 'none';
    this.cdr.markForCheck();

    try {
      const result = await this.api.analyzeDiskUsage(path);
      if (this.destroyed || this.currentPath !== path) return;
      this.result = result;
      this.entries = result.entries;
      this.topEntries = result.entries.slice(0, 5);
      this.selectedEntry = this.topEntries[0] ?? null;
      if (this.selectedEntry) {
        // Kick off the bar animation after data arrives
        setTimeout(() => {
          this.triggerBarAnimation(this.selectedEntry!);
        }, 60);
      }
    } catch (err: any) {
      if (this.destroyed || this.currentPath !== path) return;
      this.error = err?.message || String(err);
      this.result = null;
      this.entries = [];
      this.topEntries = [];
      this.selectedEntry = null;
    } finally {
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  private getParent(path: string): string | null {
    const normalized = path.replace(/\\/g, '/').replace(/\/+$/, '');
    if (/^[a-zA-Z]:?\/?$/.test(normalized) || normalized === '' || normalized === '/') return null;
    const i = normalized.lastIndexOf('/');
    if (i <= 0) return normalized.charAt(0) === '/' ? '/' : null;
    const parent = normalized.substring(0, i) || '/';
    return /^[a-zA-Z]:$/.test(parent) ? parent + '/' : parent;
  }

  private labelFromPath(path: string): string {
    const normalized = path.replace(/\\/g, '/').replace(/\/+$/, '');
    if (!normalized) return '';
    if (/^[a-zA-Z]:$/.test(normalized)) return normalized + '/';
    const segments = normalized.split('/');
    return segments[segments.length - 1] || normalized;
  }
}