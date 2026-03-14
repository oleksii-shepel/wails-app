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
    <div class="sm-root">
      <div class="sm-hero">
        <div class="sm-hero-main">
          <div class="sm-kicker">Storage Dashboard</div>
          <h1 class="sm-title">{{ currentLabel }}</h1>
          <div class="sm-path" [title]="currentPath">{{ currentPath }}</div>
          <p class="sm-subtitle">
            Summary-first disk usage analysis with ranked hotspots, footprint metrics, and drill-down into heavy folders.
          </p>
        </div>

        <div class="sm-hero-actions">
          <button class="sm-btn sm-btn--ghost" (click)="close()" title="Close analyzer">Close</button>
          <button
            class="sm-btn sm-btn--ghost"
            [disabled]="!parentPath"
            (click)="goUp()"
            title="Go to parent folder">Parent</button>
          <button class="sm-btn sm-btn--primary" (click)="refresh()">Refresh</button>
        </div>
      </div>

      <div class="sm-loading" *ngIf="loading">
        <div class="sm-spinner"></div>
        <div>
          <div class="sm-loading-title">Scanning footprint</div>
          <div class="sm-loading-copy">Collecting size, file count, and folder density for {{ currentLabel }}.</div>
        </div>
      </div>

      <div class="sm-error" *ngIf="error && !loading">
        <div>
          <div class="sm-error-title">Analysis failed</div>
          <div class="sm-error-copy">{{ error }}</div>
        </div>
        <button class="sm-btn sm-btn--primary" (click)="refresh()">Retry</button>
      </div>

      <div class="sm-dashboard" *ngIf="result && !loading">
        <section class="sm-summary-grid">
          <article class="sm-card sm-metric-card">
            <div class="sm-card-label">Total footprint</div>
            <div class="sm-card-value">{{ formatBytes(result.totalSize) }}</div>
            <div class="sm-card-meta">{{ entries.length | number }} direct children analyzed</div>
          </article>

          <article class="sm-card sm-metric-card">
            <div class="sm-card-label">Items scanned</div>
            <div class="sm-card-value">{{ totalItems | number }}</div>
            <div class="sm-card-meta">{{ result.totalFiles | number }} files · {{ result.totalDirs | number }} folders</div>
          </article>

          <article class="sm-card sm-metric-card">
            <div class="sm-card-label">Average node size</div>
            <div class="sm-card-value">{{ averageEntrySizeLabel }}</div>
            <div class="sm-card-meta">Across visible children in this location</div>
          </article>

          <article class="sm-card sm-metric-card">
            <div class="sm-card-label">Scan time</div>
            <div class="sm-card-value">{{ result.elapsedMs }} ms</div>
            <div class="sm-card-meta">{{ directoryMixLabel }}</div>
          </article>
        </section>

        <section class="sm-main-grid">
          <article class="sm-card sm-spotlight-card">
            <div class="sm-card-header">
              <div>
                <div class="sm-card-label">Spotlight</div>
                <div class="sm-card-title">{{ selectedEntry?.name || 'No dominant entry' }}</div>
              </div>
              <span class="sm-pill" [class.sm-pill--dir]="selectedEntry?.isDir">
                {{ selectedEntry?.isDir ? 'Folder' : 'File' }}
              </span>
            </div>

            <div class="sm-spotlight-value">{{ selectedEntry ? formatBytes(selectedEntry.size) : '0 B' }}</div>
            <div class="sm-spotlight-share">{{ selectedEntry ? pct(selectedEntry) : '—' }} of current location</div>

            <div class="sm-spotlight-bar" *ngIf="selectedEntry">
              <div class="sm-spotlight-bar-fill" [style.width.%]="barPercent(selectedEntry)"></div>
            </div>

            <div class="sm-insight-grid" *ngIf="selectedEntry">
              <div class="sm-insight-box">
                <span class="sm-insight-label">Contained files</span>
                <strong>{{ selectedEntry.files | number }}</strong>
              </div>
              <div class="sm-insight-box">
                <span class="sm-insight-label">Contained folders</span>
                <strong>{{ selectedEntry.dirs | number }}</strong>
              </div>
              <div class="sm-insight-box">
                <span class="sm-insight-label">Path</span>
                <strong class="sm-insight-path" [title]="selectedEntry.path">{{ selectedEntry.path }}</strong>
              </div>
            </div>

            <div class="sm-empty-state" *ngIf="!selectedEntry">
              No entries available for this location.
            </div>
          </article>

          <article class="sm-card sm-ranking-card">
            <div class="sm-card-header">
              <div>
                <div class="sm-card-label">Top consumers</div>
                <div class="sm-card-title">Largest children</div>
              </div>
            </div>

            <div class="sm-ranking-list" *ngIf="topEntries.length > 0; else noRanking">
              <button
                type="button"
                class="sm-ranking-item"
                *ngFor="let e of topEntries; let i = index; trackBy: trackEntry"
                [class.sm-ranking-item--active]="selectedEntry?.path === e.path"
                (click)="onEntryClick(e)"
                (dblclick)="onEntryDblClick(e)">
                <div class="sm-ranking-rank">{{ i + 1 }}</div>
                <div class="sm-ranking-copy">
                  <div class="sm-ranking-name" [title]="e.path">{{ e.name }}</div>
                  <div class="sm-ranking-meta">{{ e.isDir ? 'Folder' : 'File' }} · {{ pct(e) }}</div>
                </div>
                <div class="sm-ranking-size">{{ formatBytes(e.size) }}</div>
              </button>
            </div>

            <ng-template #noRanking>
              <div class="sm-empty-state">Nothing to rank in this location.</div>
            </ng-template>
          </article>
        </section>

        <section class="sm-card sm-breakdown-card">
          <div class="sm-card-header">
            <div>
              <div class="sm-card-label">Breakdown</div>
              <div class="sm-card-title">Distribution by child entry</div>
            </div>
            <div class="sm-card-caption">Single-click highlights. Double-click drills into folders.</div>
          </div>

          <div class="sm-breakdown-head" *ngIf="entries.length > 0">
            <span>Name</span>
            <span>Composition</span>
            <span>Size</span>
            <span>Share</span>
          </div>

          <div class="sm-breakdown-list" *ngIf="entries.length > 0; else emptyBreakdown">
            <button
              type="button"
              class="sm-entry"
              *ngFor="let e of entries; trackBy: trackEntry"
              [class.sm-entry--dir]="e.isDir"
              [class.sm-entry--active]="selectedEntry?.path === e.path"
              (click)="onEntryClick(e)"
              (dblclick)="onEntryDblClick(e)">

              <div class="sm-entry-main">
                <span class="sm-entry-icon">{{ e.isDir ? '▣' : '•' }}</span>
                <span class="sm-entry-name" [title]="e.path">{{ e.name }}</span>
              </div>

              <span class="sm-entry-meta">{{ entryComposition(e) }}</span>
              <span class="sm-entry-size">{{ formatBytes(e.size) }}</span>
              <span class="sm-entry-pct">{{ pct(e) }}</span>

              <div class="sm-entry-bar-track">
                <div class="sm-entry-bar" [style.width.%]="barPercent(e)"></div>
              </div>
            </button>
          </div>

          <ng-template #emptyBreakdown>
            <div class="sm-empty-state">Folder is empty.</div>
          </ng-template>
        </section>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: hidden;
    }

    .sm-root {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      overflow: auto;
      padding: 20px;
      gap: 18px;
      font-size: 13px;
      color: var(--vsc-foreground, #d4d4d4);
      background:
        radial-gradient(circle at top right, color-mix(in srgb, var(--vsc-charts-blue, #4da3ff) 16%, transparent) 0%, transparent 34%),
        radial-gradient(circle at left center, color-mix(in srgb, var(--vsc-charts-green, #7ad87a) 12%, transparent) 0%, transparent 30%),
        linear-gradient(180deg, color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 94%, #081018 6%) 0%, var(--vsc-editor-background, #1e1e1e) 100%);
    }

    .sm-hero {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 18px;
      padding: 24px;
      border: 1px solid color-mix(in srgb, var(--vsc-panel-border, #3c3c3c) 80%, transparent);
      border-radius: 24px;
      background:
        linear-gradient(135deg, color-mix(in srgb, var(--vsc-editorWidget-background, #252526) 78%, #10232f 22%) 0%, color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 82%, #142018 18%) 100%);
      box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
      flex-shrink: 0;
    }

    .sm-hero-main {
      display: flex;
      flex-direction: column;
      gap: 8px;
      min-width: 0;
    }

    .sm-kicker {
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.18em;
      color: color-mix(in srgb, var(--vsc-descriptionForeground, #9aa0a6) 92%, white 8%);
    }

    .sm-title {
      margin: 0;
      font-size: clamp(28px, 4vw, 42px);
      line-height: 1;
      font-weight: 700;
      letter-spacing: -0.04em;
    }

    .sm-path {
      font-size: 13px;
      color: var(--vsc-descriptionForeground, #9aa0a6);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      min-width: 0;
    }

    .sm-subtitle {
      margin: 0;
      max-width: 72ch;
      color: color-mix(in srgb, var(--vsc-descriptionForeground, #9aa0a6) 88%, white 12%);
      line-height: 1.5;
    }

    .sm-hero-actions {
      display: flex;
      align-items: flex-start;
      justify-content: flex-end;
      gap: 10px;
      flex-wrap: wrap;
    }

    .sm-btn {
      appearance: none;
      border: 1px solid color-mix(in srgb, var(--vsc-button-border, #3c3c3c) 84%, transparent);
      color: inherit;
      border-radius: 999px;
      padding: 10px 16px;
      cursor: pointer;
      font-size: 12px;
      font-weight: 600;
      letter-spacing: 0.02em;
      white-space: nowrap;
      background: color-mix(in srgb, var(--vsc-editorWidget-background, #252526) 86%, transparent);
      transition: transform 0.18s ease, border-color 0.18s ease, background 0.18s ease;
    }
    .sm-btn:hover {
      transform: translateY(-1px);
      border-color: color-mix(in srgb, var(--vsc-focusBorder, #007fd4) 72%, transparent);
    }
    .sm-btn--ghost:hover {
      background: color-mix(in srgb, var(--vsc-list-hover-background, #2a2d2e) 88%, transparent);
    }
    .sm-btn--primary {
      color: white;
      border-color: transparent;
      background: linear-gradient(135deg, #0f8a9d 0%, #2c5fa8 100%);
      box-shadow: 0 12px 28px rgba(28, 96, 166, 0.28);
    }
    .sm-btn:disabled { opacity: 0.35; cursor: default; }

    .sm-loading {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 14px;
      min-height: 260px;
      padding: 24px;
      border-radius: 24px;
      border: 1px solid color-mix(in srgb, var(--vsc-panel-border, #3c3c3c) 80%, transparent);
      background: color-mix(in srgb, var(--vsc-editorWidget-background, #252526) 74%, transparent);
    }
    .sm-spinner {
      width: 22px;
      height: 22px;
      border: 2px solid transparent;
      border-top-color: var(--vsc-accent-blue, #007acc);
      border-radius: 50%;
      animation: spin .7s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }

    .sm-loading-title,
    .sm-error-title {
      font-size: 16px;
      font-weight: 600;
      margin-bottom: 4px;
    }

    .sm-loading-copy,
    .sm-error-copy {
      color: var(--vsc-descriptionForeground, #9aa0a6);
    }

    .sm-error {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 16px;
      min-height: 180px;
      padding: 24px;
      border-radius: 24px;
      border: 1px solid color-mix(in srgb, #ff6b6b 38%, transparent);
      background: linear-gradient(135deg, rgba(129, 28, 40, 0.25) 0%, rgba(54, 12, 18, 0.18) 100%);
      color: var(--vsc-inputValidation-errorForeground, #f48771);
    }

    .sm-dashboard {
      display: flex;
      flex-direction: column;
      gap: 18px;
      min-height: 0;
    }

    .sm-summary-grid {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
      gap: 14px;
    }

    .sm-main-grid {
      display: grid;
      grid-template-columns: minmax(340px, 1.4fr) minmax(280px, 0.9fr);
      gap: 18px;
    }

    .sm-card {
      display: flex;
      flex-direction: column;
      gap: 14px;
      padding: 20px;
      border-radius: 22px;
      border: 1px solid color-mix(in srgb, var(--vsc-panel-border, #3c3c3c) 80%, transparent);
      background: linear-gradient(180deg, color-mix(in srgb, var(--vsc-editorWidget-background, #252526) 92%, transparent) 0%, color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 96%, transparent) 100%);
      box-shadow: 0 12px 40px rgba(0, 0, 0, 0.18);
      min-width: 0;
    }

    .sm-card-header {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 12px;
    }

    .sm-card-label {
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.16em;
      color: var(--vsc-descriptionForeground, #9aa0a6);
    }

    .sm-card-title {
      font-size: 20px;
      font-weight: 600;
      letter-spacing: -0.02em;
    }

    .sm-card-caption,
    .sm-card-meta {
      color: var(--vsc-descriptionForeground, #9aa0a6);
    }

    .sm-card-value {
      font-size: clamp(22px, 2.5vw, 34px);
      font-weight: 700;
      letter-spacing: -0.04em;
      font-variant-numeric: tabular-nums;
    }

    .sm-pill {
      align-self: flex-start;
      padding: 6px 10px;
      border-radius: 999px;
      font-size: 11px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.12em;
      color: #9bdcff;
      background: rgba(19, 108, 145, 0.18);
    }

    .sm-pill--dir {
      color: #b8d18f;
      background: rgba(106, 142, 35, 0.18);
    }

    .sm-spotlight-card {
      min-height: 0;
    }

    .sm-spotlight-value {
      font-size: clamp(34px, 5vw, 56px);
      line-height: 0.95;
      font-weight: 700;
      letter-spacing: -0.06em;
      font-variant-numeric: tabular-nums;
    }

    .sm-spotlight-share {
      color: var(--vsc-descriptionForeground, #9aa0a6);
    }

    .sm-spotlight-bar,
    .sm-entry-bar-track {
      position: relative;
      overflow: hidden;
      width: 100%;
      height: 10px;
      border-radius: 999px;
      background: color-mix(in srgb, var(--vsc-input-background, #2d2d30) 82%, transparent);
    }

    .sm-spotlight-bar-fill,
    .sm-entry-bar {
      height: 100%;
      border-radius: inherit;
      background: linear-gradient(90deg, #2ec4b6 0%, #3a86ff 100%);
    }

    .sm-insight-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 12px;
    }

    .sm-insight-box {
      display: flex;
      flex-direction: column;
      gap: 6px;
      padding: 14px;
      border-radius: 18px;
      background: color-mix(in srgb, var(--vsc-list-hover-background, #2a2d2e) 55%, transparent);
      min-width: 0;
    }

    .sm-insight-label {
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.14em;
      color: var(--vsc-descriptionForeground, #9aa0a6);
    }

    .sm-insight-path {
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .sm-ranking-list {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    .sm-ranking-item,
    .sm-entry {
      appearance: none;
      width: 100%;
      border: 0;
      color: inherit;
      background: transparent;
      padding: 0;
      text-align: left;
    }

    .sm-ranking-item {
      display: flex;
      align-items: center;
      gap: 14px;
      padding: 14px;
      border-radius: 18px;
      background: color-mix(in srgb, var(--vsc-list-hover-background, #2a2d2e) 40%, transparent);
      cursor: pointer;
      transition: transform 0.18s ease, background 0.18s ease;
    }

    .sm-ranking-item:hover,
    .sm-entry:hover {
      transform: translateY(-1px);
    }

    .sm-ranking-item--active,
    .sm-entry--active {
      background: color-mix(in srgb, var(--vsc-focusBorder, #007fd4) 18%, var(--vsc-list-hover-background, #2a2d2e) 82%);
      box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--vsc-focusBorder, #007fd4) 44%, transparent);
    }

    .sm-ranking-rank {
      width: 32px;
      height: 32px;
      flex-shrink: 0;
      display: grid;
      place-items: center;
      border-radius: 50%;
      font-weight: 700;
      font-variant-numeric: tabular-nums;
      background: rgba(58, 134, 255, 0.14);
    }

    .sm-ranking-copy {
      display: flex;
      flex-direction: column;
      gap: 4px;
      flex: 1 1 auto;
      min-width: 0;
    }

    .sm-ranking-name {
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .sm-ranking-meta {
      font-size: 12px;
      color: var(--vsc-descriptionForeground, #9aa0a6);
    }

    .sm-ranking-size {
      font-weight: 600;
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
    }

    .sm-breakdown-card {
      min-height: 0;
    }

    .sm-breakdown-head {
      display: grid;
      grid-template-columns: minmax(0, 2fr) minmax(140px, 1fr) 110px 90px;
      gap: 14px;
      padding: 0 10px;
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.14em;
      color: var(--vsc-descriptionForeground, #9aa0a6);
    }

    .sm-breakdown-list {
      display: flex;
      flex-direction: column;
      gap: 8px;
    }

    .sm-entry {
      position: relative;
      display: grid;
      grid-template-columns: minmax(0, 2fr) minmax(140px, 1fr) 110px 90px;
      gap: 14px;
      align-items: center;
      padding: 12px 14px;
      border-radius: 18px;
      cursor: pointer;
      background: color-mix(in srgb, var(--vsc-list-hover-background, #2a2d2e) 34%, transparent);
      overflow: hidden;
    }

    .sm-entry-main {
      position: relative;
      z-index: 1;
      display: flex;
      align-items: center;
      gap: 10px;
      min-width: 0;
    }

    .sm-entry-icon {
      flex-shrink: 0;
      width: 18px;
      text-align: center;
      font-size: 16px;
      color: #8fd3ff;
    }

    .sm-entry--dir .sm-entry-icon {
      color: #b8d18f;
    }

    .sm-entry-name {
      position: relative;
      z-index: 1;
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .sm-entry-meta {
      position: relative;
      z-index: 1;
      color: var(--vsc-descriptionForeground, #9aa0a6);
    }
    .sm-entry-size {
      position: relative;
      z-index: 1;
      text-align: right;
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
    }
    .sm-entry-pct {
      position: relative;
      z-index: 1;
      text-align: right;
      color: var(--vsc-descriptionForeground, #9aa0a6);
      font-variant-numeric: tabular-nums;
    }

    .sm-empty-state {
      display: flex;
      align-items: center;
      justify-content: center;
      min-height: 120px;
      color: var(--vsc-descriptionForeground, #9aa0a6);
      border-radius: 18px;
      background: color-mix(in srgb, var(--vsc-list-hover-background, #2a2d2e) 28%, transparent);
    }

    @media (max-width: 1080px) {
      .sm-main-grid {
        grid-template-columns: 1fr;
      }

      .sm-breakdown-head,
      .sm-entry {
        grid-template-columns: minmax(0, 1.8fr) minmax(120px, 1fr) 100px 70px;
      }
    }

    @media (max-width: 720px) {
      .sm-root {
        padding: 14px;
      }

      .sm-hero {
        flex-direction: column;
        padding: 18px;
      }

      .sm-hero-actions {
        width: 100%;
        justify-content: flex-start;
      }

      .sm-insight-grid {
        grid-template-columns: 1fr;
      }

      .sm-breakdown-head {
        display: none;
      }

      .sm-entry {
        grid-template-columns: 1fr;
      }

      .sm-entry-meta,
      .sm-entry-size,
      .sm-entry-pct {
        text-align: left;
      }
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

  close(): void {
    this.ctx?.sizeVm?.onClose();
  }

  goUp(): void {
    if (this.parentPath) {
      this.analyse(this.parentPath);
    }
  }

  refresh(): void {
    if (this.currentPath) {
      this.analyse(this.currentPath);
    }
  }

  onEntryClick(e: DiskUsageEntry): void {
    this.selectedEntry = e;
  }

  onEntryDblClick(e: DiskUsageEntry): void {
    if (e.isDir) {
      this.analyse(e.path);
    }
  }

  trackEntry(_: number, e: DiskUsageEntry): string {
    return e.path;
  }

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
    if (total === 0) return 'No children to categorize';
    const folders = this.entries.filter((entry) => entry.isDir).length;
    const share = (folders / total) * 100;
    return share < 0.1 ? 'Mostly files' : `${share.toFixed(0)}% folders in current view`;
  }

  barPercent(e: DiskUsageEntry): number {
    if (!this.result || this.result.totalSize === 0) return 0;
    return Math.max(0.3, (e.size / this.result.totalSize) * 100);
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
    return entry.isDir
      ? `${entry.files} files · ${entry.dirs} folders`
      : 'File';
  }

  // ── Core analysis ────────────────────────────────────────────────────────

  private async analyse(path: string): Promise<void> {
    this.ctx?.sizeVm?.onAnalyse(path);
    this.currentPath = path;
    this.parentPath = this.getParent(path);
    this.loading = true;
    this.error = null;
    this.cdr.markForCheck();

    try {
      const result = await this.api.analyzeDiskUsage(path);
      if (this.destroyed || this.currentPath !== path) return;
      this.result = result;
      this.entries = result.entries;
      this.topEntries = result.entries.slice(0, 5);
      this.selectedEntry = this.topEntries[0] ?? null;
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
    // Drive root (e.g. "C:" or "/")
    if (/^[a-zA-Z]:?\/?$/.test(normalized) || normalized === '' || normalized === '/') {
      return null;
    }
    const i = normalized.lastIndexOf('/');
    if (i <= 0) return normalized.charAt(0) === '/' ? '/' : null;
    // Keep "C:/" for Windows drive roots
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
