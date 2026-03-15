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
  tx: number;
  ty: number;
  color: string;
  pctNum: number;
}

interface TooltipState {
  visible: boolean;
  x: number;
  y: number;
  name: string;
  size: string;
  pct: string;
  /** 1 = open right, -1 = open left */
  side: number;
  isOther: boolean;
}

/**
 * One level in the internal navigation stack.
 * `label`   – breadcrumb display name
 * `entries` – the DiskUsageEntry array shown at this level
 * `total`   – denominator used for percentage calculations
 */
interface NavFrame {
  label: string;
  entries: DiskUsageEntry[];
  total: number;
}

// ─────────────────────────────────────────────────────────────────────────────

@Component({
  selector: 'app-disk-pie-chart',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="pie-panel">

      <!-- ── Header ──────────────────────────────────────────── -->
      <div class="pie-header">
        <div class="pie-breadcrumbs">
          <button
            *ngFor="let frame of navStack; let i = index; let last = last"
            class="pie-crumb"
            [class.pie-crumb--active]="last"
            [disabled]="last"
            (click)="goToFrame(i)">
            {{ frame.label }}
          </button>
        </div>
        <span class="pie-hint">Click to select · double-click folder to open</span>
      </div>

      <!-- ── Chart + legend ──────────────────────────────────── -->
      <ng-container *ngIf="displayEntries.length > 0; else noData">
        <div class="pie-container">

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
              <path
                class="pie-path"
                [class.pie-path--other]="s.index === -1"
                [attr.d]="getSlicePath(s)"
                [attr.fill]="s.color"/>
            </g>

            <!-- Donut hole -->
            <circle cx="0" cy="0" [attr.r]="innerR"
                    class="pie-hole" pointer-events="none"/>

            <!-- Center: selected entry -->
            <g *ngIf="selectedIndex !== null && selectedIndex >= 0 && hoveredIndex !== -1"
               class="pie-center" pointer-events="none">
              <text y="-7" text-anchor="middle" class="pie-center-name">{{ centerName }}</text>
              <text y="10" text-anchor="middle" class="pie-center-pct">{{ centerPct }}</text>
            </g>

            <!-- Center: Other hover hint -->
            <g *ngIf="hoveredIndex === -1" class="pie-center" pointer-events="none">
              <text y="-4" text-anchor="middle" class="pie-center-drill-line1">Other</text>
              <text y="11" text-anchor="middle" class="pie-center-drill-line2">click to expand</text>
            </g>

            <!-- SVG tooltip -->
            <g *ngIf="tooltip.visible" class="pie-tip"
               pointer-events="none"
               [attr.transform]="tooltipTransform">
              <rect class="pie-tip-bg" x="-2" y="-14"
                    [attr.width]="tooltipWidth" height="32" rx="3"/>
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

            <!-- Other row — clickable drill-in -->
            <button
              *ngIf="otherEntries.length > 0"
              class="pie-legend-item pie-legend-item--other"
              [class.pie-legend-item--hovered]="hoveredIndex === -1"
              (click)="drillIntoOther()"
              (mouseenter)="hoveredIndex = -1; cdr.markForCheck()"
              (mouseleave)="hoveredIndex = null; cdr.markForCheck()">
              <span class="pie-swatch pie-swatch--other"></span>
              <span class="pie-legend-name">
                Other
                <span class="pie-other-count">({{ otherEntries.length }})</span>
              </span>
              <span class="pie-legend-size">{{ formatBytes(otherSize) }}</span>
              <span class="pie-legend-pct">{{ otherPct }}</span>
              <span class="pie-drill-arrow">›</span>
            </button>
          </div>
        </div>

        <!-- Details strip -->
        <div class="pie-details" *ngIf="selectedEntry">
          <div class="pie-details-row">
            <span class="pie-details-type-icon">{{ selectedEntry.isDir ? '▣' : '•' }}</span>
            <span class="pie-details-name" [title]="selectedEntry.path">{{ selectedEntry.name }}</span>
            <span class="pie-pill" [class.pie-pill--dir]="selectedEntry.isDir">
              {{ selectedEntry.isDir ? 'Folder' : 'File' }}
            </span>
            <span class="pie-details-size">{{ formatBytes(selectedEntry.size) }}</span>
          </div>
          <div class="pie-bar-track">
            <div
              class="pie-bar-fill"
              [style.width.%]="animatedBarPct"
              [style.background]="selectedIndex !== null ? getColor(selectedIndex) : ''"
              [style.transition]="barTransition">
            </div>
          </div>
          <div class="pie-details-meta" *ngIf="selectedEntry.isDir">
            {{ selectedEntry.files | number }} files · {{ selectedEntry.dirs | number }} folders
          </div>
        </div>

      </ng-container>

      <ng-template #noData>
        <div class="pie-empty">No data to display</div>
      </ng-template>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
      font-size: var(--vsc-font-size, 13px);
      font-family: var(--vsc-font-family, system-ui, sans-serif);
      color: var(--vsc-foreground, #cccccc);
    }

    .pie-panel {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      padding: 12px 14px;
      gap: 12px;
      background: var(--vsc-panel-background, #252526);
      border: 1px solid var(--vsc-border, #454545);
      border-radius: var(--vsc-border-radius-lg, 4px);
    }

    /* ── Header / breadcrumbs ─────────────────────────────── */

    .pie-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      flex-shrink: 0;
    }

    .pie-breadcrumbs {
      display: flex;
      align-items: center;
      flex-wrap: wrap;
      min-width: 0;
    }

    .pie-crumb {
      appearance: none;
      border: none;
      background: none;
      font-family: inherit;
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: 0.12em;
      color: var(--vsc-foreground-dim, #888888);
      cursor: pointer;
      padding: 0;
      transition: color 0.12s;
    }

    .pie-crumb:hover:not(:disabled) {
      color: var(--vsc-foreground, #cccccc);
      text-decoration: underline;
    }

    .pie-crumb--active {
      color: var(--vsc-foreground, #cccccc);
      cursor: default;
    }

    .pie-crumb:not(:last-child)::after {
      content: ' › ';
      color: var(--vsc-foreground-dim, #888888);
      margin: 0 4px;
      display: inline;
    }

    .pie-hint {
      font-size: 11px;
      color: var(--vsc-foreground-dim, #888888);
      white-space: nowrap;
    }

    /* ── Layout ───────────────────────────────────────────── */

    .pie-container {
      display: flex;
      gap: 16px;
      align-items: center;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
    }

    .pie-svg {
      flex-shrink: 0;
      width: 220px;
      height: 220px;
      cursor: pointer;
      overflow: visible;
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

    /* Other slice: dashed stroke + zoom cursor */
    .pie-slice-g--other { cursor: zoom-in; }
    .pie-path--other    { stroke-dasharray: 3 2; }

    .pie-path {
      stroke: var(--vsc-panel-background, #252526);
      stroke-width: 2;
      stroke-linejoin: round;
      transition: stroke-width 0.15s ease, stroke 0.15s ease;
    }

    .pie-slice-g--selected .pie-path {
      stroke: var(--vsc-focus-border, #007acc);
      stroke-width: 2.5;
    }

    /* ── Donut hole + center ──────────────────────────────── */

    .pie-hole { fill: var(--vsc-panel-background, #252526); }

    .pie-center { pointer-events: none; }

    .pie-center-name {
      font-size: 10px;
      fill: var(--vsc-foreground-dim, #888888);
    }

    .pie-center-pct {
      font-size: 15px;
      font-weight: 700;
      fill: var(--vsc-foreground, #cccccc);
    }

    .pie-center-drill-line1 {
      font-size: 11px;
      fill: var(--vsc-foreground-dim, #888888);
    }

    .pie-center-drill-line2 {
      font-size: 9px;
      fill: var(--vsc-foreground-dim, #666666);
    }

    /* ── SVG tooltip ──────────────────────────────────────── */

    .pie-tip-bg {
      fill: var(--vsc-editor-background, #1e1e1e);
      stroke: var(--vsc-border, #454545);
      stroke-width: 0.75;
    }

    .pie-tip-name {
      font-size: 11px;
      font-weight: 600;
      fill: var(--vsc-foreground, #cccccc);
    }

    .pie-tip-meta {
      font-size: 10px;
      fill: var(--vsc-foreground-dim, #888888);
    }

    /* ── Legend ───────────────────────────────────────────── */

    .pie-legend {
      flex: 1 1 0%;
      display: flex;
      flex-direction: column;
      gap: 2px;
      overflow-y: auto;
      overflow-x: hidden;
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
      transition: background 0.12s ease;
      min-width: 0;
      width: 100%;
    }

    .pie-legend-item:hover,
    .pie-legend-item--hovered {
      background: var(--vsc-list-hover-background, #2a2d2e);
    }

    .pie-legend-item--active {
      background: color-mix(in srgb,
        var(--vsc-focus-border, #007acc) 10%,
        var(--vsc-list-hover-background, #2a2d2e) 90%) !important;
      border-color: color-mix(in srgb,
        var(--vsc-focus-border, #007acc) 40%, transparent);
    }

    /* Other row */
    .pie-legend-item--other {
      border-style: dashed;
      border-color: color-mix(in srgb,
        var(--vsc-foreground-dim, #888888) 30%, transparent);
    }

    .pie-legend-item--other:hover,
    .pie-legend-item--other.pie-legend-item--hovered {
      border-color: color-mix(in srgb,
        var(--vsc-foreground-dim, #888888) 60%, transparent);
    }

    .pie-swatch {
      width: 9px;
      height: 9px;
      border-radius: 2px;
      flex-shrink: 0;
    }

    .pie-swatch--other {
      border: 1px dashed var(--vsc-foreground-dim, #888888);
    }

    .pie-legend-name {
      flex: 1 1 0%;
      min-width: 0;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      font-weight: 500;
    }

    .pie-other-count {
      font-weight: 400;
      color: var(--vsc-foreground-dim, #888888);
      margin-left: 2px;
    }

    .pie-legend-size {
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
      font-size: 11px;
      color: var(--vsc-foreground-dim, #888888);
    }

    .pie-legend-pct {
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
      font-size: 11px;
      color: var(--vsc-foreground-dim, #888888);
      min-width: 42px;
      text-align: right;
    }

    .pie-drill-arrow {
      font-size: 14px;
      color: var(--vsc-foreground-dim, #888888);
      flex-shrink: 0;
      transition: transform 0.12s ease, color 0.12s ease;
    }

    .pie-legend-item--other:hover .pie-drill-arrow,
    .pie-legend-item--other.pie-legend-item--hovered .pie-drill-arrow {
      transform: translateX(2px);
      color: var(--vsc-foreground, #cccccc);
    }

    /* ── Details strip ────────────────────────────────────── */

    .pie-details {
      display: flex;
      flex-direction: column;
      gap: 7px;
      padding: 10px 12px;
      background: var(--vsc-editor-background, #1e1e1e);
      border: 1px solid var(--vsc-border-subtle, #3a3a3a);
      border-radius: var(--vsc-border-radius-lg, 4px);
      flex-shrink: 0;
    }

    .pie-details-row {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
    }

    .pie-details-type-icon {
      font-size: 12px;
      flex-shrink: 0;
      color: var(--vsc-foreground-dim, #888888);
    }

    .pie-details-name {
      flex: 1 1 0%;
      min-width: 0;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      font-weight: 600;
      font-size: 13px;
    }

    .pie-pill {
      font-size: 9px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.08em;
      padding: 2px 6px;
      border-radius: 999px;
      background: rgba(157, 220, 252, 0.15);
      color: #9ddcfc;
      flex-shrink: 0;
    }

    .pie-pill--dir {
      background: rgba(184, 209, 143, 0.15);
      color: #b8d18f;
    }

    .pie-details-size {
      font-size: 12px;
      font-weight: 600;
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
      flex-shrink: 0;
    }

    .pie-bar-track {
      height: 4px;
      border-radius: 999px;
      background: var(--vsc-input-background, #3c3c3c);
      border: 1px solid var(--vsc-border-subtle, #3a3a3a);
      overflow: hidden;
    }

    .pie-bar-fill {
      height: 100%;
      border-radius: inherit;
      width: 0%;
    }

    .pie-details-meta {
      font-size: 11px;
      color: var(--vsc-foreground-dim, #888888);
    }

    .pie-empty {
      display: flex;
      align-items: center;
      justify-content: center;
      flex: 1;
      min-height: 180px;
      color: var(--vsc-foreground-dim, #888888);
    }

    :host-context(.vscode-light) .pie-pill {
      background: rgba(10, 93, 143, 0.12); color: #0a5d8f;
    }
    :host-context(.vscode-light) .pie-pill--dir {
      background: rgba(90, 110, 0, 0.12); color: #5a6e00;
    }
    :host-context(.vscode-light) .pie-hole {
      fill: var(--vsc-panel-background, #f3f3f3);
    }
    :host-context(.vscode-light) .pie-path {
      stroke: var(--vsc-panel-background, #f3f3f3);
    }

    @media (max-width: 480px) {
      .pie-container { flex-direction: column; align-items: center; }
      .pie-svg { width: 180px; height: 180px; }
    }
  `],
})
export class DiskPieChartComponent implements OnChanges, OnDestroy {
  @Input({ required: true }) entries: DiskUsageEntry[] = [];
  @Input({ required: true }) totalSize = 0;
  @Input() innerR = 42;
  @Input() outerR = 100;
  /** Slices whose share is below this % are grouped into "Other" */
  @Input() minPct = 10;

  @Output() select    = new EventEmitter<DiskUsageEntry>();
  @Output() drillDown = new EventEmitter<DiskUsageEntry>();

  // ── Nav stack ─────────────────────────────────────────────────────────────

  navStack: NavFrame[] = [];

  // ── Current-level state ───────────────────────────────────────────────────

  displayEntries: DiskUsageEntry[] = [];
  otherEntries:   DiskUsageEntry[] = [];
  otherSize  = 0;
  otherPct   = '';
  readonly otherColor = '#555566';

  slices: PieSlice[]   = [];
  selectedIndex: number | null = null;
  hoveredIndex:  number | null = null;

  tooltip: TooltipState = {
    visible: false, x: 0, y: 0, name: '', size: '', pct: '', side: 1, isOther: false,
  };
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
    this.navStack = [
      ...this.navStack,
      { label: 'Other', entries: this.otherEntries, total: this.currentFrame.total },
    ];
    this.reset();
    this.applyCurrentFrame();
  }

  goToFrame(index: number): void {
    if (index >= this.navStack.length - 1) return;
    this.navStack = this.navStack.slice(0, index + 1);
    this.reset();
    this.applyCurrentFrame();
  }

  // ── Computed getters ──────────────────────────────────────────────────────

  get selectedEntry(): DiskUsageEntry | null {
    return this.selectedIndex !== null
      ? (this.displayEntries[this.selectedIndex] ?? null)
      : null;
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
    return `translate(${x.toFixed(1)}, ${(this.tooltip.y - 16).toFixed(1)})`;
  }

  // ── Public helpers ────────────────────────────────────────────────────────

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

  // ── Slice geometry ────────────────────────────────────────────────────────

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
      const nameW = Math.max(name.length * 6.5 + 12, 80);
      const metaW = (size.length + pct.length + 3) * 6 + 12;
      this.tooltipWidth = Math.ceil(Math.max(nameW, metaW));
      this.tooltip = {
        visible: true,
        x: +svgP.x.toFixed(1), y: +svgP.y.toFixed(1),
        name, size, pct,
        side: svgP.x < 0 ? 1 : -1,
        isOther,
      };
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
    if (idx === -1)               { this.drillIntoOther(); return; }
    if (idx !== null && idx >= 0)   this.selectSlice(idx);
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

  private get currentFrame(): NavFrame {
    return this.navStack[this.navStack.length - 1];
  }

  private reset(): void {
    this.selectedIndex = null;
    this.hoveredIndex  = null;
    this.tooltip       = { ...this.tooltip, visible: false };
    this.animatedBarPct = 0;
    this.barTransition  = 'none';
  }

  private applyCurrentFrame(): void {
    const frame      = this.currentFrame;
    const isSubLevel = this.navStack.length > 1;

    if (!frame.total || !frame.entries.length) {
      this.displayEntries = [];
      this.otherEntries   = [];
      this.otherSize      = 0;
      this.otherPct       = '';
      this.slices         = [];
      this.cdr.markForCheck();
      return;
    }

    if (isSubLevel) {
      // Inside Other: show every entry, no further bucketing.
      // If there are more entries than palette colours, overflow into a second Other.
      const cap = this.palette.length * 2;
      this.displayEntries = frame.entries.slice(0, cap);
      this.otherEntries   = frame.entries.slice(cap);
    } else {
      // Root level: apply minPct bucketing
      const threshold = this.minPct / 100;
      const main: DiskUsageEntry[] = [];
      const rest: DiskUsageEntry[] = [];
      for (const e of frame.entries) {
        (e.size / frame.total >= threshold ? main : rest).push(e);
      }
      this.displayEntries = main.slice(0, this.palette.length);
      this.otherEntries   = rest;
    }

    this.otherSize = this.otherEntries.reduce((s, e) => s + e.size, 0);
    this.otherPct  = this.otherSize
      ? ((this.otherSize / frame.total) * 100).toFixed(1) + '%'
      : '';

    this.buildSlices();
    this.cdr.markForCheck();
  }

  private buildSlices(): void {
    this.slices = [];
    const total = this.currentFrame?.total;
    if (!total) return;

    const drawnTotal =
      this.displayEntries.reduce((s, e) => s + e.size, 0) + this.otherSize;
    if (drawnTotal === 0) return;

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
        pctNum: (size / total) * 100,
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
    this.barTransition  = 'none';
    this.animatedBarPct = 0;
    this.cdr.markForCheck();
    requestAnimationFrame(() => {
      if (this.destroyed) return;
      this.barTransition = 'width 0.5s cubic-bezier(0.4, 0, 0.2, 1)';
      if (this.selectedIndex !== null) {
        const total = this.currentFrame?.total ?? this.totalSize;
        this.animatedBarPct =
          (this.displayEntries[this.selectedIndex].size / total) * 100;
      }
      this.cdr.markForCheck();
    });
  }
}