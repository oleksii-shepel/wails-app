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
import type { DiskUsageEntry, DiskUsageResult } from '../../shared/protocol-enhanced';
import type { SizeManagerSurfaceContext } from '../surface-context';

// ─────────────────────────────────────────────────────────────────────────────
// Types shared by the pie chart
// ─────────────────────────────────────────────────────────────────────────────

interface PieSlice {
  index: number; // -1 = "Other" bucket
  startAngle: number;
  endAngle: number;
  midAngle: number;
  tx: number; // unit-vector x for radial hover translate
  ty: number; // unit-vector y
  color: string;
  pctNum: number;
}

interface TooltipState {
  visible: boolean;
  x: number; // SVG-space
  y: number;
  name: string;
  size: string;
  pct: string;
  side: number; // 1 = open right, -1 = open left
  isOther: boolean;
}

interface NavFrame {
  label: string;
  entries: DiskUsageEntry[];
  total: number;
}

// ─────────────────────────────────────────────────────────────────────────────
// DiskPieChartComponent  (inlined — no separate file needed)
// ─────────────────────────────────────────────────────────────────────────────

@Component({
  selector: 'app-disk-pie-chart',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <!-- ── Header: back button + breadcrumbs + hint ─────── -->
    <div class="pc-header">
      <div class="pc-header-left">
        <!-- Back button — only visible when inside an Other level -->
        <button
          *ngIf="navStack.length > 1"
          class="pc-back-btn"
          (click)="goToFrame(navStack.length - 2)"
          title="Back">
          ‹ Back
        </button>

        <!-- Breadcrumb trail -->
        <div class="pc-breadcrumbs">
          <button
            *ngFor="let frame of navStack; let i = index; let last = last"
            class="pc-crumb"
            [class.pc-crumb--active]="last"
            [disabled]="last"
            (click)="goToFrame(i)">{{ frame.label }}</button>
        </div>
      </div>
      <span class="pc-hint">click · dbl-click folder to open</span>
    </div>

    <!-- ── Main area ──────────────────────────────────────── -->
    <ng-container *ngIf="displayEntries.length > 0; else noData">
      <div class="pc-body">

        <!-- Left: donut SVG -->
        <svg class="pc-svg" viewBox="-120 -120 240 240"
          (mousemove)="onSvgMouseMove($event)"
          (mouseleave)="onSvgMouseLeave()"
          (click)="onSvgClick($event)"
          (dblclick)="onSvgDblClick($event)">

          <g *ngFor="let s of slices; trackBy: trackSlice"
             class="pc-slice-g"
             [class.pc-slice-g--hovered]="hoveredIndex === s.index"
             [class.pc-slice-g--selected]="selectedIndex === s.index"
             [class.pc-slice-g--other]="s.index === -1"
             [style.--tx]="s.tx" [style.--ty]="s.ty">
            <path class="pc-path"
                  [class.pc-path--other]="s.index === -1"
                  [attr.d]="getSlicePath(s)"
                  [attr.fill]="s.color"/>
          </g>

          <!-- Hole -->
          <circle cx="0" cy="0" [attr.r]="innerR" class="pc-hole" pointer-events="none"/>

          <!-- Center: selected -->
          <g *ngIf="selectedIndex !== null && selectedIndex >= 0 && hoveredIndex !== -1"
             class="pc-center" pointer-events="none">
            <text y="-7"  text-anchor="middle" class="pc-center-name">{{ centerName }}</text>
            <text y="10" text-anchor="middle" class="pc-center-pct">{{ centerPct }}</text>
          </g>

          <!-- Center: other drill hint -->
          <g *ngIf="hoveredIndex === -1" class="pc-center" pointer-events="none">
            <text y="-4" text-anchor="middle" class="pc-center-other1">Other</text>
            <text y="11" text-anchor="middle" class="pc-center-other2">click to expand</text>
          </g>

          <!-- SVG tooltip -->
          <g *ngIf="tooltip.visible" class="pc-tip" pointer-events="none"
             [attr.transform]="tooltipTransform">
            <rect class="pc-tip-bg" x="-2" y="-14" [attr.width]="tooltipWidth" height="32" rx="3"/>
            <text class="pc-tip-name" x="6" y="0">{{ tooltip.name }}</text>
            <text class="pc-tip-meta" x="6" y="14">{{ tooltip.size }} · {{ tooltip.pct }}</text>
          </g>
        </svg>

        <!-- Right: spotlight + legend stacked -->
        <div class="pc-right">

          <!-- Spotlight card — shown when something is selected -->
          <div class="pc-spotlight" *ngIf="selectedEntry; else noSelection">
            <div class="pc-spot-top">
              <span class="pc-spot-icon">{{ selectedEntry.isDir ? '▣' : '•' }}</span>
              <span class="pc-spot-name" [title]="selectedEntry.path">{{ selectedEntry.name }}</span>
              <span class="pc-pill" [class.pc-pill--dir]="selectedEntry.isDir">
                {{ selectedEntry.isDir ? 'Folder' : 'File' }}
              </span>
            </div>
            <div class="pc-spot-size">{{ formatBytes(selectedEntry.size) }}</div>
            <div class="pc-spot-share">{{ pctStr(selectedEntry) }} of this location</div>
            <div class="pc-bar-track">
              <div class="pc-bar-fill"
                   [style.width.%]="animatedBarPct"
                   [style.background]="selectedIndex !== null ? getColor(selectedIndex) : ''"
                   [style.transition]="barTransition">
              </div>
            </div>
            <div class="pc-spot-stats" *ngIf="selectedEntry.isDir">
              <div class="pc-stat">
                <div class="pc-stat-val">{{ selectedEntry.files | number }}</div>
                <div class="pc-stat-lbl">files</div>
              </div>
              <div class="pc-stat">
                <div class="pc-stat-val">{{ selectedEntry.dirs | number }}</div>
                <div class="pc-stat-lbl">folders</div>
              </div>
            </div>
          </div>

          <ng-template #noSelection>
            <div class="pc-no-selection">← select a slice</div>
          </ng-template>

          <!-- Legend -->
          <div class="pc-legend">
            <button
              *ngFor="let e of displayEntries; let i = index"
              class="pc-legend-row"
              [class.pc-legend-row--active]="selectedIndex === i"
              [class.pc-legend-row--hovered]="hoveredIndex === i"
              (click)="selectSlice(i)"
              (dblclick)="onLegendDblClick(e)"
              (mouseenter)="hoveredIndex = i; cdr.markForCheck()"
              (mouseleave)="hoveredIndex = null; cdr.markForCheck()">
              <span class="pc-swatch" [style.background]="getColor(i)"></span>
              <span class="pc-leg-name" [title]="e.name">{{ e.name }}</span>
              <span class="pc-leg-size">{{ formatBytes(e.size) }}</span>
              <span class="pc-leg-pct">{{ pctStr(e) }}</span>
            </button>

            <!-- Other row -->
            <button
              *ngIf="otherEntries.length > 0"
              class="pc-legend-row pc-legend-row--other"
              [class.pc-legend-row--hovered]="hoveredIndex === -1"
              (click)="drillIntoOther()"
              (mouseenter)="hoveredIndex = -1; cdr.markForCheck()"
              (mouseleave)="hoveredIndex = null; cdr.markForCheck()">
              <span class="pc-swatch pc-swatch--other"></span>
              <span class="pc-leg-name">Other <span class="pc-other-n">({{ otherEntries.length }})</span></span>
              <span class="pc-leg-size">{{ formatBytes(otherSize) }}</span>
              <span class="pc-leg-pct">{{ otherPct }}</span>
              <span class="pc-drill-arr">›</span>
            </button>
          </div>

        </div><!-- /pc-right -->
      </div>
    </ng-container>

    <ng-template #noData>
      <div class="pc-empty">No data to display</div>
    </ng-template>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      gap: 10px;
      min-width: 0;
      font-size: var(--vsc-font-size, 13px);
      font-family: var(--vsc-font-family, system-ui, sans-serif);
      color: var(--vsc-foreground, #cccccc);
    }

    /* ── Header ───────────────────────────────────────────── */

    .pc-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      flex-shrink: 0;
    }

    .pc-header-left {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
    }

    /* ‹ Back button — compact, borderless, accent on hover */
    .pc-back-btn {
      appearance: none;
      border: 1px solid var(--vsc-border-subtle, #3a3a3a);
      border-radius: var(--vsc-border-radius, 3px);
      background: transparent;
      font-family: inherit;
      font-size: 11px;
      color: var(--vsc-foreground-dim, #888888);
      cursor: pointer;
      padding: 2px 8px;
      white-space: nowrap;
      flex-shrink: 0;
      transition: color .12s, border-color .12s, background .12s;
    }
    .pc-back-btn:hover {
      color: var(--vsc-foreground, #cccccc);
      border-color: var(--vsc-foreground-dim, #888888);
      background: var(--vsc-list-hover-background, #2a2d2e);
    }

    .pc-breadcrumbs {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
    }

    .pc-crumb {
      appearance: none;
      border: none;
      background: none;
      font-family: inherit;
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: .12em;
      color: var(--vsc-foreground-dim, #888888);
      cursor: pointer;
      padding: 0;
      transition: color .12s;
    }
    .pc-crumb:hover:not(:disabled) {
      color: var(--vsc-foreground, #cccccc);
      text-decoration: underline;
    }
    .pc-crumb--active { color: var(--vsc-foreground, #cccccc); cursor: default; }
    .pc-crumb:not(:last-child)::after {
      content: ' › ';
      color: var(--vsc-foreground-dim, #888888);
      margin: 0 4px;
    }

    .pc-hint {
      font-size: 11px;
      color: var(--vsc-foreground-dim, #888888);
      white-space: nowrap;
    }

    /* ── Body layout ──────────────────────────────────────── */

    .pc-body {
      display: flex;
      gap: 14px;
      align-items: flex-start;
      min-height: 0;
    }

    .pc-svg {
      flex-shrink: 0;
      width: 200px;
      height: 200px;
      cursor: pointer;
      overflow: visible;
    }

    .pc-right {
      flex: 1 1 0%;
      min-width: 0;
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    /* ── Slices ───────────────────────────────────────────── */

    .pc-slice-g {
      --lift: 7px;
      transition: transform .18s cubic-bezier(.4,0,.2,1);
      cursor: pointer;
    }
    .pc-slice-g--hovered {
      transform: translateX(calc(var(--tx) * var(--lift)))
                 translateY(calc(var(--ty) * var(--lift)));
    }
    .pc-slice-g--selected {
      transform: translateX(calc(var(--tx) * 5px))
                 translateY(calc(var(--ty) * 5px));
    }
    .pc-slice-g--selected.pc-slice-g--hovered {
      transform: translateX(calc(var(--tx) * var(--lift)))
                 translateY(calc(var(--ty) * var(--lift)));
    }
    .pc-slice-g--other { cursor: zoom-in; }

    .pc-path {
      stroke: var(--vsc-panel-background, #252526);
      stroke-width: 2;
      stroke-linejoin: round;
      transition: stroke-width .15s, stroke .15s;
    }
    .pc-path--other    { stroke-dasharray: 3 2; }
    .pc-slice-g--selected .pc-path {
      stroke: var(--vsc-focus-border, #007acc);
      stroke-width: 2.5;
    }

    /* ── Hole + center text ───────────────────────────────── */

    .pc-hole { fill: var(--vsc-panel-background, #252526); }
    .pc-center { pointer-events: none; }

    .pc-center-name  { font-size: 10px; fill: var(--vsc-foreground-dim, #888888); }
    .pc-center-pct   { font-size: 15px; font-weight: 700; fill: var(--vsc-foreground, #cccccc); }
    .pc-center-other1 { font-size: 11px; fill: var(--vsc-foreground-dim, #888888); }
    .pc-center-other2 { font-size: 9px;  fill: var(--vsc-foreground-dim, #666666); }

    /* ── Tooltip ──────────────────────────────────────────── */

    .pc-tip-bg {
      fill: var(--vsc-editor-background, #1e1e1e);
      stroke: var(--vsc-border, #454545);
      stroke-width: .75;
    }
    .pc-tip-name { font-size: 11px; font-weight: 600; fill: var(--vsc-foreground, #cccccc); }
    .pc-tip-meta { font-size: 10px; fill: var(--vsc-foreground-dim, #888888); }

    /* ── Spotlight card ───────────────────────────────────── */

    .pc-spotlight {
      display: flex;
      flex-direction: column;
      gap: 6px;
      padding: 10px 12px;
      border-radius: var(--vsc-border-radius-lg, 4px);
      border: 1px solid var(--vsc-border-subtle, #3a3a3a);
      background: var(--vsc-editor-background, #1e1e1e);
      flex-shrink: 0;
    }

    .pc-spot-top {
      display: flex;
      align-items: center;
      gap: 7px;
      min-width: 0;
    }

    .pc-spot-icon {
      font-size: 11px;
      flex-shrink: 0;
      color: var(--vsc-foreground-dim, #888888);
    }

    .pc-spot-name {
      flex: 1;
      min-width: 0;
      font-weight: 600;
      font-size: 13px;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .pc-pill {
      font-size: 9px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: .08em;
      padding: 2px 6px;
      border-radius: 999px;
      background: rgba(157,220,252,.15);
      color: #9ddcfc;
      flex-shrink: 0;
    }
    .pc-pill--dir { background: rgba(184,209,143,.15); color: #b8d18f; }

    .pc-spot-size {
      font-size: 22px;
      font-weight: 700;
      letter-spacing: -.04em;
      font-variant-numeric: tabular-nums;
      line-height: 1;
    }

    .pc-spot-share {
      font-size: 11px;
      color: var(--vsc-foreground-dim, #888888);
    }

    .pc-bar-track {
      height: 4px;
      border-radius: 999px;
      background: var(--vsc-input-background, #3c3c3c);
      border: 1px solid var(--vsc-border-subtle, #3a3a3a);
      overflow: hidden;
    }

    .pc-bar-fill {
      height: 100%;
      border-radius: inherit;
      width: 0%;
    }

    .pc-spot-stats {
      display: flex;
      gap: 10px;
    }

    .pc-stat {
      display: flex;
      align-items: baseline;
      gap: 4px;
    }

    .pc-stat-val {
      font-size: 12px;
      font-weight: 600;
      font-variant-numeric: tabular-nums;
    }

    .pc-stat-lbl {
      font-size: 10px;
      color: var(--vsc-foreground-dim, #888888);
    }

    .pc-no-selection {
      font-size: 11px;
      color: var(--vsc-foreground-dim, #888888);
      padding: 8px 12px;
      text-align: center;
      border: 1px dashed var(--vsc-border-subtle, #3a3a3a);
      border-radius: var(--vsc-border-radius-lg, 4px);
    }

    /* ── Legend ───────────────────────────────────────────── */

    .pc-legend {
      display: flex;
      flex-direction: column;
      gap: 1px;
      overflow-y: auto;
      min-width: 0;
    }

    .pc-legend-row {
      appearance: none;
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius, 3px);
      padding: 4px 7px;
      display: flex;
      align-items: center;
      gap: 7px;
      cursor: pointer;
      background: transparent;
      color: inherit;
      font-family: inherit;
      font-size: 11px;
      text-align: left;
      transition: background .1s;
      width: 100%;
      min-width: 0;
    }
    .pc-legend-row:hover,
    .pc-legend-row--hovered { background: var(--vsc-list-hover-background, #2a2d2e); }
    .pc-legend-row--active {
      background: color-mix(in srgb,
        var(--vsc-focus-border, #007acc) 10%,
        var(--vsc-list-hover-background, #2a2d2e) 90%) !important;
      border-color: color-mix(in srgb, var(--vsc-focus-border, #007acc) 40%, transparent);
    }
    .pc-legend-row--other {
      border-style: dashed;
      border-color: color-mix(in srgb, var(--vsc-foreground-dim, #888888) 30%, transparent);
    }
    .pc-legend-row--other:hover,
    .pc-legend-row--other.pc-legend-row--hovered {
      border-color: color-mix(in srgb, var(--vsc-foreground-dim, #888888) 60%, transparent);
    }

    .pc-swatch {
      width: 8px; height: 8px;
      border-radius: 2px;
      flex-shrink: 0;
    }
    .pc-swatch--other {
      border: 1px dashed var(--vsc-foreground-dim, #888888);
    }

    .pc-leg-name {
      flex: 1;
      min-width: 0;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      font-weight: 500;
    }

    .pc-other-n {
      font-weight: 400;
      color: var(--vsc-foreground-dim, #888888);
    }

    .pc-leg-size {
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
      color: var(--vsc-foreground-dim, #888888);
    }

    .pc-leg-pct {
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
      color: var(--vsc-foreground-dim, #888888);
      min-width: 38px;
      text-align: right;
    }

    .pc-drill-arr {
      font-size: 13px;
      color: var(--vsc-foreground-dim, #888888);
      transition: transform .12s, color .12s;
    }
    .pc-legend-row--other:hover .pc-drill-arr,
    .pc-legend-row--other.pc-legend-row--hovered .pc-drill-arr {
      transform: translateX(2px);
      color: var(--vsc-foreground, #cccccc);
    }

    /* ── Empty ────────────────────────────────────────────── */

    .pc-empty {
      display: flex; align-items: center; justify-content: center;
      min-height: 120px;
      color: var(--vsc-foreground-dim, #888888);
    }

    /* Light theme */
    :host-context(.vscode-light) .pc-hole { fill: var(--vsc-panel-background, #f3f3f3); }
    :host-context(.vscode-light) .pc-path { stroke: var(--vsc-panel-background, #f3f3f3); }
    :host-context(.vscode-light) .pc-pill { background: rgba(10,93,143,.12); color: #0a5d8f; }
    :host-context(.vscode-light) .pc-pill--dir { background: rgba(90,110,0,.12); color: #5a6e00; }
  `],
})
export class DiskPieChartComponent implements OnChanges, OnDestroy {
  @Input({ required: true }) entries: DiskUsageEntry[] = [];
  @Input({ required: true }) totalSize = 0;
  @Input() innerR = 42;
  @Input() outerR = 100;
  @Input() minPct = 10;

  @Output() select    = new EventEmitter<DiskUsageEntry>();
  @Output() drillDown = new EventEmitter<DiskUsageEntry>();

  // ── Nav stack ─────────────────────────────────────────────────────────────
  navStack: NavFrame[] = [];

  // ── Derived state ─────────────────────────────────────────────────────────
  displayEntries: DiskUsageEntry[] = [];
  otherEntries:   DiskUsageEntry[] = [];
  otherSize = 0;
  otherPct  = '';
  readonly otherColor = '#555566';

  slices: PieSlice[]   = [];
  selectedIndex: number | null = null;
  hoveredIndex:  number | null = null;

  tooltip: TooltipState = { visible: false, x: 0, y: 0, name: '', size: '', pct: '', side: 1, isOther: false };
  tooltipWidth = 120;

  animatedBarPct = 0;
  barTransition  = 'none';

  readonly palette = [
    '#3794ff', '#4ec9b0', '#ce9178', '#c586c0',
    '#dcdcaa', '#569cd6', '#b5cea8', '#d7ba7d',
  ];

  private destroyed = false;

  constructor(readonly cdr: ChangeDetectorRef) {}

  // ── Lifecycle ─────────────────────────────────────────────────────────────

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['entries'] || changes['totalSize']) {
      this.navStack = [{ label: 'Distribution', entries: this.entries, total: this.totalSize }];
      this.applyCurrentFrame();
    }
  }

  ngOnDestroy(): void { this.destroyed = true; }

  // ── Navigation ────────────────────────────────────────────────────────────

  drillIntoOther(): void {
    if (!this.otherEntries.length) return;
    const depth = this.navStack.length; // 1 = first drill, 2 = second, …
    const label = depth === 1 ? 'Other' : `Other (${depth})`;
    this.navStack = [...this.navStack,
      { label, entries: this.otherEntries, total: this.currentFrame.total }];
    this.reset();
    this.applyCurrentFrame();
  }

  goToFrame(index: number): void {
    if (index >= this.navStack.length - 1) return;
    this.navStack = this.navStack.slice(0, index + 1);
    this.reset();
    this.applyCurrentFrame();
  }

  // ── Computed ──────────────────────────────────────────────────────────────

  get selectedEntry(): DiskUsageEntry | null {
    return this.selectedIndex !== null ? (this.displayEntries[this.selectedIndex] ?? null) : null;
  }

  get centerName(): string {
    if (this.selectedIndex === null) return '';
    const e = this.displayEntries[this.selectedIndex];
    return e ? (e.name.length > 10 ? e.name.slice(0, 9) + '…' : e.name) : '';
  }

  get centerPct(): string {
    return this.selectedIndex !== null ? this.pctStr(this.displayEntries[this.selectedIndex]) : '';
  }

  get tooltipTransform(): string {
    const PAD = 8;
    const x = this.tooltip.side === 1
      ? this.tooltip.x + PAD
      : this.tooltip.x - this.tooltipWidth - PAD;
    return `translate(${x.toFixed(1)},${(this.tooltip.y - 16).toFixed(1)})`;
  }

  // ── Helpers ───────────────────────────────────────────────────────────────

  getColor(i: number): string { return this.palette[i % this.palette.length]; }

  pctStr(e: DiskUsageEntry): string {
    const total = this.currentFrame?.total ?? this.totalSize;
    if (!total) return '—';
    const p = (e.size / total) * 100;
    return p < 0.1 ? '<0.1%' : p.toFixed(1) + '%';
  }

  formatBytes(bytes: number): string {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), 4);
    const v = bytes / Math.pow(1024, i);
    return (i === 0 ? v : v.toFixed(1)) + ' ' + units[i];
  }

  trackSlice(_: number, s: PieSlice): number { return s.index; }

  // ── Slice path ────────────────────────────────────────────────────────────

  getSlicePath(s: PieSlice): string {
    const o = this.outerR, ir = this.innerR;
    const p = (a: number, r: number) =>
      ({ x: +(r * Math.cos(a)).toFixed(3), y: +(r * Math.sin(a)).toFixed(3) });
    const os = p(s.startAngle, o), oe = p(s.endAngle, o);
    const is_ = p(s.startAngle, ir), ie = p(s.endAngle, ir);
    const large = s.endAngle - s.startAngle > Math.PI ? 1 : 0;
    return [
      `M ${os.x} ${os.y}`,
      `A ${o} ${o} 0 ${large} 1 ${oe.x} ${oe.y}`,
      `L ${ie.x} ${ie.y}`,
      `A ${ir} ${ir} 0 ${large} 0 ${is_.x} ${is_.y}`,
      'Z',
    ].join(' ');
  }

  // ── SVG events ────────────────────────────────────────────────────────────

  onSvgMouseMove(event: MouseEvent): void {
    const svgP = this.toSvgCoords(event);
    if (!svgP) return;
    const idx = this.hitTest(svgP.x, svgP.y);
    if (idx !== this.hoveredIndex) this.hoveredIndex = idx;

    if (idx !== null) {
      const isOther = idx === -1;
      const name  = isOther ? `Other (${this.otherEntries.length})` : (this.displayEntries[idx]?.name ?? '');
      const size  = isOther ? this.formatBytes(this.otherSize) : this.formatBytes(this.displayEntries[idx]?.size ?? 0);
      const pct   = isOther ? this.otherPct : this.pctStr(this.displayEntries[idx]);
      this.tooltipWidth = Math.ceil(Math.max(name.length * 6.5 + 12, (size.length + pct.length + 3) * 6 + 12, 80));
      this.tooltip = { visible: true, x: +svgP.x.toFixed(1), y: +svgP.y.toFixed(1), name, size, pct, side: svgP.x < 0 ? 1 : -1, isOther };
    } else {
      this.tooltip = { ...this.tooltip, visible: false };
    }
    this.cdr.markForCheck();
  }

  onSvgMouseLeave(): void {
    this.hoveredIndex = null;
    this.tooltip = { ...this.tooltip, visible: false };
    this.cdr.markForCheck();
  }

  onSvgClick(event: MouseEvent): void {
    const svgP = this.toSvgCoords(event);
    if (!svgP) return;
    const idx = this.hitTest(svgP.x, svgP.y);
    if (idx === -1)             { this.drillIntoOther(); return; }
    if (idx !== null && idx >= 0) this.selectSlice(idx);
  }

  onSvgDblClick(event: MouseEvent): void {
    const svgP = this.toSvgCoords(event);
    if (!svgP) return;
    const idx = this.hitTest(svgP.x, svgP.y);
    if (idx !== null && idx >= 0) {
      const e = this.displayEntries[idx];
      if (e?.isDir) this.drillDown.emit(e);
    }
  }

  selectSlice(i: number): void {
    this.selectedIndex = i;
    this.triggerBarAnimation();
    const e = this.displayEntries[i];
    if (e) this.select.emit(e);
    this.cdr.markForCheck();
  }

  onLegendDblClick(e: DiskUsageEntry): void {
    if (e.isDir) this.drillDown.emit(e);
  }

  // ── Private ───────────────────────────────────────────────────────────────

  private get currentFrame(): NavFrame { return this.navStack[this.navStack.length - 1]; }

  private reset(): void {
    this.selectedIndex  = null;
    this.hoveredIndex   = null;
    this.tooltip        = { ...this.tooltip, visible: false };
    this.animatedBarPct = 0;
    this.barTransition  = 'none';
  }

  private applyCurrentFrame(): void {
    const frame = this.currentFrame;

    if (!frame.total || !frame.entries.length) {
      this.displayEntries = []; this.otherEntries = [];
      this.otherSize = 0; this.otherPct = ''; this.slices = [];
      this.cdr.markForCheck(); return;
    }

    // Apply minPct bucketing at every level — makes Other drill-down
    // recursive: each sub-chart re-buckets its own entries the same way.
    const threshold = this.minPct / 100;
    const main: DiskUsageEntry[] = [], rest: DiskUsageEntry[] = [];
    for (const e of frame.entries) {
      (e.size / frame.total >= threshold ? main : rest).push(e);
    }
    // Cap named slices at palette length; overflow also goes to Other
    this.displayEntries = main.slice(0, this.palette.length);
    const overflow      = main.slice(this.palette.length);
    this.otherEntries   = [...overflow, ...rest];

    this.otherSize = this.otherEntries.reduce((s, e) => s + e.size, 0);
    this.otherPct  = this.otherSize
      ? ((this.otherSize / frame.total) * 100).toFixed(1) + '%' : '';

    this.buildSlices();
    this.cdr.markForCheck();
  }

  private buildSlices(): void {
    this.slices = [];
    const total = this.currentFrame?.total;
    if (!total) return;
    const drawnTotal = this.displayEntries.reduce((s, e) => s + e.size, 0) + this.otherSize;
    if (!drawnTotal) return;

    let angle = -Math.PI / 2;
    const push = (index: number, size: number, color: string) => {
      const sweep = (size / drawnTotal) * Math.PI * 2;
      const start = angle, end = angle + sweep, mid = start + sweep / 2;
      angle = end;
      this.slices.push({ index, startAngle: start, endAngle: end, midAngle: mid,
        tx: +Math.cos(mid).toFixed(4), ty: +Math.sin(mid).toFixed(4), color,
        pctNum: (size / total) * 100 });
    };

    this.displayEntries.forEach((e, i) => push(i, e.size, this.getColor(i)));
    if (this.otherSize > 0) push(-1, this.otherSize, this.otherColor);
  }

  private hitTest(x: number, y: number): number | null {
    const dist = Math.sqrt(x * x + y * y);
    if (dist < this.innerR || dist > this.outerR + 8) return null;
    let a = Math.atan2(y, x);
    if (a < -Math.PI / 2) a += Math.PI * 2;
    for (const s of this.slices) {
      let sa = s.startAngle, ea = s.endAngle;
      if (sa < -Math.PI / 2) sa += Math.PI * 2;
      if (ea < -Math.PI / 2) ea += Math.PI * 2;
      if (ea < sa) ea += Math.PI * 2;
      const norm = a < sa ? a + Math.PI * 2 : a;
      if (norm >= sa && norm <= ea) return s.index;
    }
    return null;
  }

  private toSvgCoords(event: MouseEvent): { x: number; y: number } | null {
    const svg = event.currentTarget as SVGSVGElement;
    const pt  = svg.createSVGPoint();
    pt.x = event.clientX; pt.y = event.clientY;
    const inv = svg.getScreenCTM()?.inverse();
    if (!inv) return null;
    const r = pt.matrixTransform(inv);
    return { x: r.x, y: r.y };
  }

  private triggerBarAnimation(): void {
    this.barTransition = 'none'; this.animatedBarPct = 0;
    this.cdr.markForCheck();
    requestAnimationFrame(() => {
      if (this.destroyed) return;
      this.barTransition = 'width .5s cubic-bezier(.4,0,.2,1)';
      if (this.selectedIndex !== null) {
        const total = this.currentFrame?.total ?? this.totalSize;
        this.animatedBarPct = (this.displayEntries[this.selectedIndex].size / total) * 100;
      }
      this.cdr.markForCheck();
    });
  }
}

// ─────────────────────────────────────────────────────────────────────────────
// SizeManagerSurfaceComponent
// ─────────────────────────────────────────────────────────────────────────────

@Component({
  selector: 'app-size-manager-surface',
  standalone: true,
  imports: [CommonModule, DiskPieChartComponent],
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

      <!--
        ── Merged spotlight + pie panel ──────────────────────
        Replaces the old two-column sm-cols (spotlight + ranking list).
        The pie chart component owns the donut on the left;
        spotlight details appear inline on the right via the pie's
        internal selectedEntry state.
        Selecting a row in the breakdown table also selects the slice
        via [selectedEntry] binding — and vice-versa.
      -->
      <div class="sm-panel sm-pie-panel">
        <app-disk-pie-chart
          [entries]="entries"
          [totalSize]="result.totalSize"
          [minPct]="10"
          (select)="onEntryClick($event)"
          (drillDown)="onEntryDblClick($event)">
        </app-disk-pie-chart>
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
      --_bg:         #ffffff;
      --_surface:    #f3f3f3;
      --_surface2:   #e4e6f1;
      --_input-bg:   #ffffff;
      --_text:       #3b3b3b;
      --_text-dim:   #737373;
      --_border:     #c4c4c4;
      --_border-sub: #e5e5e5;
      --_focus:      #007acc;
      --_btn-bg:     #007acc;
      --_btn-hover:  #005ba1;
      --_hover-bg:   #e4e6f1;
      --_active-bg:  #cce0f5;
      --_error:      #c7372f;
      --_blue:       #007acc;
      --_accent-a:   #0fa89e;
      --_accent-b:   #1a6fd4;
      --_icon-file:  #1a6fd4;
      --_icon-dir:   #a0700a;
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
      transition: background var(--vsc-transition-fast, .1s ease),
                  border-color var(--vsc-transition-fast, .1s ease);
      white-space: nowrap;
    }
    .sm-btn:hover:not(:disabled) { background: var(--_hover-bg); }
    .sm-btn:disabled { opacity: .35; cursor: default; }
    .sm-btn--primary { background: var(--_btn-bg); color: #fff; border-color: transparent; }
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
      width: 14px; height: 14px;
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

    /* ── Body scroll ─────────────────────────────────────── */

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

    /* ── Metrics ─────────────────────────────────────────── */

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
    }

    .sm-metric-lbl {
      margin-top: 3px;
      font-size: 11px;
      color: var(--_text-dim);
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
      flex-shrink: 0;
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

    .sm-panel-hint { font-size: 11px; color: var(--_text-dim); }

    /* Pie panel — let it size naturally, no fixed height */
    .sm-pie-panel { padding: 12px 14px; }

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

    .sm-row-icon { flex-shrink: 0; font-size: 11px; color: var(--_icon-file); }
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

    @media (max-width: 640px) {
      .sm-table-head { display: none; }
      .sm-row { grid-template-columns: minmax(0, 1fr) auto; }
      .sm-row-meta { display: none; }
    }
  `],
})
export class SizeManagerSurfaceComponent implements OnChanges, OnDestroy {
  @Input({ required: true }) ctx!: SizeManagerSurfaceContext;

  loading = false;
  error: string | null = null;
  result: DiskUsageResult | null = null;
  entries: DiskUsageEntry[] = [];
  selectedEntry: DiskUsageEntry | null = null;
  currentPath = '';
  parentPath: string | null = null;

  private destroyed = false;

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['ctx']) {
      const path = this.ctx?.sizeVm?.analysisPath;
      if (path && path !== this.currentPath) this.analyse(path);
    }
  }

  ngOnDestroy(): void { this.destroyed = true; }

  // ── Actions ──────────────────────────────────────────────────────────────

  close():   void { this.ctx?.sizeVm?.onClose(); }
  goUp():    void { if (this.parentPath) this.analyse(this.parentPath); }
  refresh(): void { if (this.currentPath) this.analyse(this.currentPath); }

  /** Called by pie (select output) and breakdown table rows */
  onEntryClick(e: DiskUsageEntry): void {
    this.selectedEntry = e;
    this.cdr.markForCheck();
  }

  /** Called by pie (drillDown output) and breakdown table dblclick */
  onEntryDblClick(e: DiskUsageEntry): void {
    if (e.isDir) this.analyse(e.path);
  }

  trackEntry(_: number, e: DiskUsageEntry): string { return e.path; }

  // ── View helpers ─────────────────────────────────────────────────────────

  get currentLabel(): string {
    return this.labelFromPath(this.currentPath) || this.currentPath || 'Disk Usage';
  }

  get totalItems(): number {
    return this.result ? this.result.totalFiles + this.result.totalDirs : 0;
  }

  get averageEntrySizeLabel(): string {
    if (!this.entries.length || !this.result) return '0 B';
    return this.formatBytes(this.result.totalSize / this.entries.length);
  }

  get directoryMixLabel(): string {
    const total = this.entries.length;
    if (!total) return 'No children';
    const share = (this.entries.filter(e => e.isDir).length / total) * 100;
    return share < 0.1 ? 'Mostly files' : `${share.toFixed(0)}% folders`;
  }

  barPercent(e: DiskUsageEntry): number {
    if (!this.result || !this.result.totalSize) return 0;
    return Math.max(0.3, (e.size / this.result.totalSize) * 100);
  }

  pct(e: DiskUsageEntry): string {
    if (!this.result || !this.result.totalSize) return '—';
    const p = (e.size / this.result.totalSize) * 100;
    return p < 0.1 ? '<0.1%' : p.toFixed(1) + '%';
  }

  formatBytes(bytes: number): string {
    if (!bytes) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const k = 1024;
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), units.length - 1);
    const val = bytes / Math.pow(k, i);
    return (i === 0 ? val : val.toFixed(1)) + ' ' + units[i];
  }

  entryComposition(entry: DiskUsageEntry): string {
    return entry.isDir ? `${entry.files} files · ${entry.dirs} dirs` : 'File';
  }

  // ── Core analysis ─────────────────────────────────────────────────────────

  async analyse(path: string): Promise<void> {
    this.ctx?.sizeVm?.onAnalyse(path);
    this.currentPath = path;
    this.parentPath  = this.getParent(path);
    this.loading     = true;
    this.error       = null;
    this.cdr.markForCheck();

    try {
      const result = await this.api.analyzeDiskUsage(path);
      if (this.destroyed || this.currentPath !== path) return;
      this.result  = result;
      this.entries = result.entries;
      this.selectedEntry = result.entries[0] ?? null;
    } catch (err: any) {
      if (this.destroyed || this.currentPath !== path) return;
      this.error   = err?.message || String(err);
      this.result  = null;
      this.entries = [];
      this.selectedEntry = null;
    } finally {
      this.loading = false;
      this.cdr.markForCheck();
    }
  }

  private getParent(path: string): string | null {
    const n = path.replace(/\\/g, '/').replace(/\/+$/, '');
    if (/^[a-zA-Z]:?\/?$/.test(n) || !n || n === '/') return null;
    const i = n.lastIndexOf('/');
    if (i <= 0) return n.charAt(0) === '/' ? '/' : null;
    const p = n.substring(0, i) || '/';
    return /^[a-zA-Z]:$/.test(p) ? p + '/' : p;
  }

  private labelFromPath(path: string): string {
    const n = path.replace(/\\/g, '/').replace(/\/+$/, '');
    if (!n) return '';
    if (/^[a-zA-Z]:$/.test(n)) return n + '/';
    const segs = n.split('/');
    return segs[segs.length - 1] || n;
  }
}