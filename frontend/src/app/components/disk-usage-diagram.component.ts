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
import { CommonModule } from '@angular/common';
import type { DiskUsageEntry } from '../shared/protocol-enhanced';

// ─── Types ────────────────────────────────────────────────────────────────────

interface PieSlice {
  /** -1 = "Other" bucket */
  index: number;
  startAngle: number;
  endAngle: number;
  midAngle: number;
  /** unit vector components for radial hover translate */
  tx: number;
  ty: number;
  color: string;
}

interface TooltipState {
  visible: boolean;
  x: number; // SVG-space
  y: number;
  name: string;
  size: string;
  pct: string;
  side: number; // 1 = open right, -1 = open left
}

// ─────────────────────────────────────────────────────────────────────────────

/**
 * Pure display component — renders a donut chart for a flat list of entries.
 *
 * Navigation / drill logic is intentionally NOT here.
 * The parent (SizeManagerSurfaceComponent) owns the nav stack and passes
 * the current level's entries + totalSize down as inputs.
 *
 * Outputs:
 *   (select)     — user clicked a named slice
 *   (otherDrill) — user clicked the "Other" slice/row; parent should push a
 *                  new nav frame containing the bucketed entries
 *   (drillDown)  — user double-clicked a folder slice; parent navigates into it
 */
@Component({
  selector: 'app-disk-pie-chart',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <!-- SVG donut -->
    <svg
      class="pie-svg"
      viewBox="-120 -120 240 240"
      (mousemove)="onSvgMouseMove($event)"
      (mouseleave)="onSvgMouseLeave()"
      (click)="onSvgClick($event)"
      (dblclick)="onSvgDblClick($event)">

      <g
        *ngFor="let s of slices; trackBy: trackSlice"
        class="pie-slice-g"
        [class.pie-slice-g--hovered]="hoveredIndex === s.index"
        [class.pie-slice-g--selected]="selectedIndex === s.index"
        [class.pie-slice-g--other]="s.index === -1"
        [style.--tx]="s.tx"
        [style.--ty]="s.ty">

        <!-- 100% full-circle fallback: arc command is degenerate at 360° -->
        <ng-container *ngIf="isFullCircle(s); else arcSlice">
          <circle class="pie-path" [class.pie-path--other]="s.index === -1"
                  cx="0" cy="0" [attr.r]="outerR" [attr.fill]="s.color"/>
          <circle class="pie-path pie-path--inner-mask"
                  cx="0" cy="0" [attr.r]="innerR"
                  fill="var(--vsc-panel-background, #252526)"/>
        </ng-container>
        <ng-template #arcSlice>
          <path class="pie-path" [class.pie-path--other]="s.index === -1"
                [attr.d]="getSlicePath(s)" [attr.fill]="s.color"/>
        </ng-template>

      </g>

      <!-- Donut hole -->
      <circle cx="0" cy="0" [attr.r]="innerR" class="pie-hole" pointer-events="none"/>

      <!-- Center: selected entry name + pct -->
      <g *ngIf="selectedIndex !== null && selectedIndex >= 0 && hoveredIndex !== -1"
         class="pie-center" pointer-events="none">
        <text y="-7"  text-anchor="middle" class="pie-center-name">{{ centerName }}</text>
        <text y="10" text-anchor="middle" class="pie-center-pct">{{ centerPct }}</text>
      </g>

      <!-- Center: Other hover hint -->
      <g *ngIf="hoveredIndex === -1" class="pie-center" pointer-events="none">
        <text y="-4" text-anchor="middle" class="pie-center-other1">Other</text>
        <text y="11" text-anchor="middle" class="pie-center-other2">click to expand</text>
      </g>

      <!-- SVG tooltip -->
      <g *ngIf="tooltip.visible" class="pie-tip" pointer-events="none"
         [attr.transform]="tooltipTransform">
        <rect class="pie-tip-bg" x="-2" y="-14" [attr.width]="tooltipWidth" height="32" rx="3"/>
        <text class="pie-tip-name" x="6" y="0">{{ tooltip.name }}</text>
        <text class="pie-tip-meta" x="6" y="14">{{ tooltip.size }} · {{ tooltip.pct }}</text>
      </g>

    </svg>

    <!-- Legend -->
    <div class="pie-legend">
      <button
        *ngFor="let e of displayEntries; let i = index"
        class="pie-legend-item"
        [class.pie-legend-item--active]="selectedIndex === i"
        [class.pie-legend-item--hovered]="hoveredIndex === i"
        (click)="selectSlice(i)"
        (dblclick)="onLegendDblClick(e)"
        (mouseenter)="hoveredIndex = i; cdr.markForCheck()"
        (mouseleave)="hoveredIndex = null; cdr.markForCheck()">
        <span class="pie-swatch" [style.background]="getColor(i)"></span>
        <span class="pie-legend-name" [title]="e.name">{{ e.name }}</span>
        <span class="pie-legend-size">{{ formatBytes(e.size) }}</span>
        <span class="pie-legend-pct">{{ pctStr(e) }}</span>
      </button>

      <!-- Other row -->
      <button
        *ngIf="otherEntries.length > 0"
        class="pie-legend-item pie-legend-item--other"
        [class.pie-legend-item--hovered]="hoveredIndex === -1"
        (click)="onOtherClick()"
        (mouseenter)="hoveredIndex = -1; cdr.markForCheck()"
        (mouseleave)="hoveredIndex = null; cdr.markForCheck()">
        <span class="pie-swatch pie-swatch--other"></span>
        <span class="pie-legend-name">
          Other <span class="pie-other-count">({{ otherEntries.length }})</span>
        </span>
        <span class="pie-legend-size">{{ formatBytes(otherSize) }}</span>
        <span class="pie-legend-pct">{{ otherPct }}</span>
        <span class="pie-drill-arrow">›</span>
      </button>
    </div>

    <!-- Details strip -->
    <div class="pie-details" *ngIf="selectedEntry">
      <div class="pie-details-row">
        <span class="pie-details-icon">{{ selectedEntry.isDir ? '▣' : '•' }}</span>
        <span class="pie-details-name" [title]="selectedEntry.path">{{ selectedEntry.name }}</span>
        <span class="pie-pill" [class.pie-pill--dir]="selectedEntry.isDir">
          {{ selectedEntry.isDir ? 'Folder' : 'File' }}
        </span>
        <span class="pie-details-size">{{ formatBytes(selectedEntry.size) }}</span>
      </div>
      <div class="pie-bar-track">
        <div class="pie-bar-fill"
             [style.width.%]="animatedBarPct"
             [style.background]="selectedIndex !== null ? getColor(selectedIndex) : ''"
             [style.transition]="barTransition">
        </div>
      </div>
      <div class="pie-details-meta" *ngIf="selectedEntry.isDir">
        {{ selectedEntry.files | number }} files · {{ selectedEntry.dirs | number }} folders
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      gap: 10px;
      font-size: var(--vsc-font-size, 13px);
      font-family: var(--vsc-font-family, system-ui, sans-serif);
      color: var(--vsc-foreground, #cccccc);
      min-width: 0;
    }

    /* ── Layout ───────────────────────────────────────────── */

    .pie-svg {
      flex-shrink: 0;
      width: 220px;
      height: 220px;
      cursor: pointer;
      overflow: visible;
      align-self: center;
    }

    /* ── Slice groups — radial hover translate ────────────── */

    .pie-slice-g {
      --lift: 7px;
      transition: transform 0.18s cubic-bezier(0.4, 0, 0.2, 1);
      cursor: pointer;
    }
    .pie-slice-g--hovered {
      transform: translateX(calc(var(--tx) * var(--lift)))
                 translateY(calc(var(--ty) * var(--lift)));
    }
    .pie-slice-g--selected {
      transform: translateX(calc(var(--tx) * 5px))
                 translateY(calc(var(--ty) * 5px));
    }
    .pie-slice-g--selected.pie-slice-g--hovered {
      transform: translateX(calc(var(--tx) * var(--lift)))
                 translateY(calc(var(--ty) * var(--lift)));
    }
    .pie-slice-g--other { cursor: zoom-in; }
    .pie-path--other    { stroke-dasharray: 3 2; }

    .pie-path {
      stroke: var(--vsc-panel-background, #252526);
      stroke-width: 2;
      stroke-linejoin: round;
      transition: stroke-width 0.15s ease, stroke 0.15s ease;
    }
    .pie-path--inner-mask { stroke: none !important; pointer-events: none; }
    .pie-slice-g--selected .pie-path:not(.pie-path--inner-mask) {
      stroke: var(--vsc-focus-border, #007acc);
      stroke-width: 2.5;
    }

    /* ── Donut hole + center ──────────────────────────────── */

    .pie-hole { fill: var(--vsc-panel-background, #252526); }
    .pie-center { pointer-events: none; }
    .pie-center-name  { font-size: 10px; fill: var(--vsc-foreground-dim, #888); }
    .pie-center-pct   { font-size: 15px; font-weight: 700; fill: var(--vsc-foreground, #ccc); }
    .pie-center-other1 { font-size: 11px; fill: var(--vsc-foreground-dim, #888); }
    .pie-center-other2 { font-size: 9px;  fill: var(--vsc-foreground-dim, #666); }

    /* ── SVG tooltip ──────────────────────────────────────── */

    .pie-tip-bg   { fill: var(--vsc-editor-background, #1e1e1e); stroke: var(--vsc-border, #454545); stroke-width: 0.75; }
    .pie-tip-name { font-size: 11px; font-weight: 600; fill: var(--vsc-foreground, #ccc); }
    .pie-tip-meta { font-size: 10px; fill: var(--vsc-foreground-dim, #888); }

    /* ── Legend ───────────────────────────────────────────── */

    .pie-legend {
      display: flex;
      flex-direction: column;
      gap: 2px;
      min-width: 0;
    }

    .pie-legend-item {
      appearance: none;
      border: 1px solid transparent;
      border-radius: var(--vsc-border-radius, 3px);
      padding: 5px 8px;
      display: flex;
      align-items: center;
      gap: 8px;
      cursor: pointer;
      background: transparent;
      color: inherit;
      font-family: inherit;
      font-size: 12px;
      text-align: left;
      transition: background 0.12s;
      min-width: 0;
      width: 100%;
    }
    .pie-legend-item:hover,
    .pie-legend-item--hovered { background: var(--vsc-list-hover-background, #2a2d2e); }
    .pie-legend-item--active {
      background: color-mix(in srgb,
        var(--vsc-focus-border, #007acc) 10%,
        var(--vsc-list-hover-background, #2a2d2e) 90%) !important;
      border-color: color-mix(in srgb, var(--vsc-focus-border, #007acc) 40%, transparent);
    }
    .pie-legend-item--other {
      border-style: dashed;
      border-color: color-mix(in srgb, var(--vsc-foreground-dim, #888) 30%, transparent);
    }
    .pie-legend-item--other:hover,
    .pie-legend-item--other.pie-legend-item--hovered {
      border-color: color-mix(in srgb, var(--vsc-foreground-dim, #888) 60%, transparent);
    }

    .pie-swatch {
      width: 9px; height: 9px;
      border-radius: 2px;
      flex-shrink: 0;
    }
    .pie-swatch--other {
      border: 1px dashed var(--vsc-foreground-dim, #888);
    }
    .pie-legend-name {
      flex: 1 1 0%;
      min-width: 0;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      font-weight: 500;
    }
    .pie-other-count { font-weight: 400; color: var(--vsc-foreground-dim, #888); margin-left: 2px; }
    .pie-legend-size { font-variant-numeric: tabular-nums; white-space: nowrap; font-size: 11px; color: var(--vsc-foreground-dim, #888); }
    .pie-legend-pct  { font-variant-numeric: tabular-nums; white-space: nowrap; font-size: 11px; color: var(--vsc-foreground-dim, #888); min-width: 42px; text-align: right; }
    .pie-drill-arrow { font-size: 14px; color: var(--vsc-foreground-dim, #888); flex-shrink: 0; transition: transform 0.12s, color 0.12s; }
    .pie-legend-item--other:hover .pie-drill-arrow,
    .pie-legend-item--other.pie-legend-item--hovered .pie-drill-arrow { transform: translateX(2px); color: var(--vsc-foreground, #ccc); }

    /* ── Details strip ────────────────────────────────────── */

    .pie-details {
      display: flex;
      flex-direction: column;
      gap: 7px;
      padding: 10px 12px;
      background: var(--vsc-editor-background, #1e1e1e);
      border: 1px solid var(--vsc-border-subtle, #3a3a3a);
      border-radius: var(--vsc-border-radius-lg, 4px);
    }
    .pie-details-row { display: flex; align-items: center; gap: 8px; min-width: 0; }
    .pie-details-icon { font-size: 11px; flex-shrink: 0; color: var(--vsc-foreground-dim, #888); }
    .pie-details-name { flex: 1; min-width: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; font-weight: 600; font-size: 13px; }
    .pie-pill { font-size: 9px; font-weight: 700; text-transform: uppercase; letter-spacing: .08em; padding: 2px 6px; border-radius: 999px; background: rgba(157,220,252,.15); color: #9ddcfc; flex-shrink: 0; }
    .pie-pill--dir { background: rgba(184,209,143,.15); color: #b8d18f; }
    .pie-details-size { font-size: 12px; font-weight: 600; font-variant-numeric: tabular-nums; white-space: nowrap; flex-shrink: 0; }
    .pie-bar-track { height: 4px; border-radius: 999px; background: var(--vsc-input-background, #3c3c3c); border: 1px solid var(--vsc-border-subtle, #3a3a3a); overflow: hidden; }
    .pie-bar-fill  { height: 100%; border-radius: inherit; width: 0%; }
    .pie-details-meta { font-size: 11px; color: var(--vsc-foreground-dim, #888); }

    /* Light theme */
    :host-context(.vscode-light) .pie-hole  { fill: var(--vsc-panel-background, #f3f3f3); }
    :host-context(.vscode-light) .pie-path  { stroke: var(--vsc-panel-background, #f3f3f3); }
    :host-context(.vscode-light) .pie-pill  { background: rgba(10,93,143,.12); color: #0a5d8f; }
    :host-context(.vscode-light) .pie-pill--dir { background: rgba(90,110,0,.12); color: #5a6e00; }
  `],
})
export class DiskPieChartComponent implements OnChanges, OnDestroy {
  /** Entries for the CURRENT level — parent controls which level this is. */
  @Input({ required: true }) entries: DiskUsageEntry[] = [];
  /**
   * Total size denominator for percentages.
   * At root level: the directory total.
   * At sub-levels: parent passes the sum of the Other bucket it drilled into.
   */
  @Input({ required: true }) totalSize = 0;
  @Input() innerR  = 42;
  @Input() outerR  = 100;
  /** Entries below this % of totalSize are grouped into the Other bucket. */
  @Input() minPct  = 10;

  @Output() select     = new EventEmitter<DiskUsageEntry>();
  /** Emits the bucketed "Other" entries — parent pushes a new nav frame. */
  @Output() otherDrill = new EventEmitter<DiskUsageEntry[]>();
  @Output() drillDown  = new EventEmitter<DiskUsageEntry>();

  // ── Derived state (rebuilt whenever inputs change) ────────────────────────

  displayEntries: DiskUsageEntry[] = [];
  otherEntries:   DiskUsageEntry[] = [];
  otherSize = 0;
  otherPct  = '';
  readonly otherColor = '#555566';

  slices: PieSlice[]   = [];
  selectedIndex: number | null = null;
  hoveredIndex:  number | null = null;

  tooltip: TooltipState = { visible: false, x: 0, y: 0, name: '', size: '', pct: '', side: 1 };
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
    if (changes['entries'] || changes['totalSize'] || changes['minPct']) {
      // Reset interaction state when the data changes (parent navigated)
      this.selectedIndex  = null;
      this.hoveredIndex   = null;
      this.tooltip        = { ...this.tooltip, visible: false };
      this.animatedBarPct = 0;
      this.barTransition  = 'none';
      this.partition();
      this.buildSlices();
    }
  }

  ngOnDestroy(): void { this.destroyed = true; }

  // ── Computed getters ──────────────────────────────────────────────────────

  get selectedEntry(): DiskUsageEntry | null {
    return this.selectedIndex !== null ? (this.displayEntries[this.selectedIndex] ?? null) : null;
  }

  get centerName(): string {
    if (this.selectedIndex === null) return '';
    const e = this.displayEntries[this.selectedIndex];
    return e ? (e.name.length > 10 ? e.name.slice(0, 9) + '…' : e.name) : '';
  }

  get centerPct(): string {
    if (this.selectedIndex === null) return '';
    return this.pctStr(this.displayEntries[this.selectedIndex]);
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
    if (!this.totalSize) return '—';
    const p = (e.size / this.totalSize) * 100;
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

  isFullCircle(s: PieSlice): boolean {
    return s.endAngle - s.startAngle >= Math.PI * 2 - 0.001;
  }

  // ── Slice path ────────────────────────────────────────────────────────────

  getSlicePath(s: PieSlice): string {
    if (this.isFullCircle(s)) return '';
    const o = this.outerR, ir = this.innerR;
    const pt = (a: number, r: number) =>
      ({ x: +(r * Math.cos(a)).toFixed(3), y: +(r * Math.sin(a)).toFixed(3) });
    const os = pt(s.startAngle, o), oe = pt(s.endAngle, o);
    const is_ = pt(s.startAngle, ir), ie = pt(s.endAngle, ir);
    const large = s.endAngle - s.startAngle > Math.PI ? 1 : 0;
    return [
      `M ${os.x} ${os.y}`,
      `A ${o} ${o} 0 ${large} 1 ${oe.x} ${oe.y}`,
      `L ${ie.x} ${ie.y}`,
      `A ${ir} ${ir} 0 ${large} 0 ${is_.x} ${is_.y}`,
      'Z',
    ].join(' ');
  }

  // ── Event handlers ────────────────────────────────────────────────────────

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
      this.tooltipWidth = Math.ceil(Math.max(name.length * 6.5 + 18, (size.length + pct.length + 3) * 6 + 18, 90));
      this.tooltip = { visible: true, x: +svgP.x.toFixed(1), y: +svgP.y.toFixed(1), name, size, pct, side: svgP.x < 0 ? 1 : -1 };
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
    if (idx === -1)             { this.onOtherClick(); return; }
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

  onOtherClick(): void {
    if (this.otherEntries.length) this.otherDrill.emit(this.otherEntries);
  }

  onLegendDblClick(e: DiskUsageEntry): void {
    if (e.isDir) this.drillDown.emit(e);
  }

  // ── Private ───────────────────────────────────────────────────────────────

  /**
   * Split entries into displayEntries (≥ minPct) and otherEntries (< minPct).
   * If nothing clears the threshold, show everything — chart is never empty.
   */
  private partition(): void {
    const total = this.totalSize;

    if (!this.entries.length || !total) {
      this.displayEntries = this.entries.slice(0, this.palette.length);
      this.otherEntries   = this.entries.slice(this.palette.length);
      this.otherSize      = this.otherEntries.reduce((s, e) => s + e.size, 0);
      this.otherPct       = '';
      return;
    }

    const threshold = this.minPct / 100;
    const main: DiskUsageEntry[] = [];
    const rest: DiskUsageEntry[] = [];
    for (const e of this.entries) {
      (e.size / total >= threshold ? main : rest).push(e);
    }

    // If nothing passes the threshold, show everything without bucketing.
    const visible = main.length > 0 ? main : this.entries;
    const leftover = main.length > 0 ? rest : [];

    this.displayEntries = visible.slice(0, this.palette.length);
    const overflow      = visible.slice(this.palette.length);
    this.otherEntries   = [...overflow, ...leftover];

    this.otherSize = this.otherEntries.reduce((s, e) => s + e.size, 0);
    this.otherPct  = this.otherSize
      ? ((this.otherSize / total) * 100).toFixed(1) + '%'
      : '';
  }

  private buildSlices(): void {
    this.slices = [];
    const drawnTotal = this.displayEntries.reduce((s, e) => s + e.size, 0) + this.otherSize;
    if (!drawnTotal) return;

    let angle = -Math.PI / 2;
    const push = (index: number, size: number, color: string) => {
      const sweep = (size / drawnTotal) * Math.PI * 2;
      const start = angle, end = angle + sweep, mid = start + sweep / 2;
      angle = end;
      this.slices.push({
        index, startAngle: start, endAngle: end, midAngle: mid,
        tx: +Math.cos(mid).toFixed(4),
        ty: +Math.sin(mid).toFixed(4),
        color,
      });
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
      this.barTransition = 'width 0.5s cubic-bezier(0.4, 0, 0.2, 1)';
      if (this.selectedIndex !== null && this.totalSize) {
        this.animatedBarPct = (this.displayEntries[this.selectedIndex].size / this.totalSize) * 100;
      }
      this.cdr.markForCheck();
    });
  }
}