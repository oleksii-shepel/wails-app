import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, Input, OnChanges } from '@angular/core';
import { ApiService } from '../../services/api.service';
import { FileInfo, FileProbe, FileType } from '../../shared/protocol';
import { ViewerArchiveContext } from '../viewer-archive-context';

type BinaryViewerKind = 'empty' | 'loading' | 'grid' | 'directory' | 'toobig' | 'error';

interface BinaryRow {
  offsetHex: string;
  cells: Array<{ hex: string; ascii: string }>;
  ascii: string;
}

const MAX_BINARY_PREVIEW_BYTES = 256 * 1024;
const BYTES_PER_ROW = 16;

@Component({
  selector: 'app-binary-viewer',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <div class="viewer-header">
        <div class="header-main">
          <span class="file-icon">01</span>
          <div class="header-text">
            <div class="title" [title]="displayName">{{ displayName || 'Binary Preview' }}</div>
            <div class="path" [title]="displayPath">{{ displayPath || 'Select a file to preview' }}</div>
          </div>
        </div>
        <div class="header-meta" *ngIf="kind === 'grid' || kind === 'directory' || kind === 'toobig'">
          <span class="badge archive-badge" *ngIf="archiveContext" title="Loaded from archive">
            &#x1F4E6; archive
          </span>
          <span class="badge type-badge">{{ typeBadge }}</span>
          <span class="badge size-badge" *ngIf="info?.size">{{ formatSize(info?.size ?? 0) }}</span>
          <span class="badge bytes-badge" *ngIf="kind === 'grid'">{{ byteLength }} bytes</span>
        </div>
      </div>

      <div class="viewer-toolbar" *ngIf="kind === 'grid'">
        <button class="tool-btn" type="button" (click)="copyPath()" [class.active]="copiedPath">
          {{ copiedPath ? 'Copied' : 'Copy path' }}
        </button>
        <button class="tool-btn" type="button" *ngIf="!archiveContext && targetPath" (click)="openExternally()">
          Open externally
        </button>
        <span class="toolbar-divider"></span>
        
      </div>

      <div class="viewer-body">
        <div *ngIf="kind === 'loading'" class="state-container">
          <div class="spinner"></div>
          <div class="state-label">Loading binary preview…</div>
        </div>

        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon">&#x1F50D;</div>
          <div class="state-label">Select a file to preview</div>
        </div>

        <div *ngIf="kind === 'directory'" class="state-container">
          <div class="state-icon">&#x1F4C1;</div>
          <div class="state-label">Directory — no preview available</div>
        </div>

        <div *ngIf="kind === 'toobig'" class="state-container">
          <div class="state-icon">&#x1F4E6;</div>
          <div class="state-label">Binary preview is limited to {{ formatSize(maxPreviewBytes) }}</div>
          <div class="state-sub">{{ formatSize(info?.size ?? 0) }}</div>
        </div>

        <div *ngIf="kind === 'error'" class="state-container state-error">
          <div class="state-icon">&#x26A0;</div>
          <div class="state-label">Failed to load binary preview</div>
          <div class="error-detail">{{ error }}</div>
        </div>

        <div *ngIf="kind === 'grid'" class="grid-shell">
          <div class="grid-header">
            <div class="offset-col">Offset</div>
            <div class="hex-col">
              <span *ngFor="let label of headerLabels">{{ label }}</span>
            </div>
            <div class="ascii-col">ASCII</div>
          </div>

          <div class="grid-body">
            <div class="grid-row" *ngFor="let row of rows">
              <div class="offset-col">{{ row.offsetHex }}</div>
              <div class="hex-col">
                <span
                  class="hex-cell"
                  *ngFor="let cell of row.cells"
                  [class.zero]="cell.hex === '00'">
                  {{ cell.hex }}
                </span>
              </div>
              <div class="ascii-col ascii-text">{{ row.ascii }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }
    .viewer-surface {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-code-background, var(--vsc-editor-background, #1e1e1e));
      color: var(--vsc-foreground, #cccccc);
    }
    .viewer-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      padding: 8px 12px;
      border-bottom: 1px solid var(--vsc-border, #454545);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 92%, black);
      flex-shrink: 0;
      min-width: 0;
    }
    .header-main {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
      flex: 1;
    }
    .file-icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 28px;
      height: 28px;
      border-radius: 6px;
      font-size: 11px;
      font-weight: 700;
      letter-spacing: 0.12em;
      color: var(--vsc-accent-orange, #e8c040);
      background: color-mix(in srgb, var(--vsc-accent-orange, #f6b73c) 16%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, #f6b73c) 36%, transparent);
      flex-shrink: 0;
    }
    .header-text {
      min-width: 0;
      flex: 1;
    }
    .title {
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 0.3px;
      text-transform: uppercase;
      opacity: 0.9;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .path {
      font-family: var(--vsc-font-family-mono, ui-monospace, 'Courier New', monospace);
      font-size: 11px;
      opacity: 0.6;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      margin-top: 1px;
    }
    .header-meta {
      display: flex;
      align-items: center;
      gap: 4px;
      flex-shrink: 0;
    }
    .badge {
      font-size: 10px;
      font-weight: 600;
      padding: 2px 6px;
      border-radius: 3px;
      white-space: nowrap;
      letter-spacing: 0.3px;
    }
    .archive-badge {
      background: color-mix(in srgb, var(--vsc-accent-orange, #f90) 20%, transparent);
      color: var(--vsc-accent-orange, #e8c040);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, #f90) 40%, transparent);
    }
    .type-badge {
      background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent);
      color: var(--vsc-accent-blue, #4da8ff);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 40%, transparent);
    }
    .size-badge,
    .bytes-badge {
      background: color-mix(in srgb, var(--vsc-foreground, #ccc) 8%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 50%, transparent);
    }
    .viewer-toolbar {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 4px 8px;
      border-bottom: 1px solid var(--vsc-border, #454545);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 96%, black);
      flex-shrink: 0;
    }
    .tool-btn {
      background: transparent;
      border: 1px solid transparent;
      color: var(--vsc-foreground, #ccc);
      font-size: 12px;
      padding: 2px 8px;
      border-radius: 3px;
      cursor: pointer;
      line-height: 1.4;
      transition: background 0.1s, border-color 0.1s;
    }
    .tool-btn:hover {
      background: var(--vsc-list-hover-background, rgba(255,255,255,0.05));
      border-color: var(--vsc-border, #454545);
    }
    .tool-btn.active {
      background: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 20%, transparent);
      border-color: color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 50%, transparent);
      color: var(--vsc-accent-blue, #4da8ff);
    }
    .toolbar-divider {
      width: 1px;
      height: 16px;
      background: var(--vsc-border, #454545);
      margin: 0 4px;
    }
    .toolbar-note {
      font-size: 11px;
      opacity: 0.55;
    }
    .viewer-body {
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
      display: flex;
      overflow: hidden;
    }
    .grid-shell {
      display: flex;
      flex-direction: column;
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      font-family: var(--vsc-font-family-mono, ui-monospace, 'Cascadia Code', 'SFMono-Regular', Consolas, monospace);
      font-size: 12px;
      line-height: 1.6;
    }
    .grid-header,
    .grid-row {
      display: grid;
      grid-template-columns: 88px minmax(0, 1fr) 180px;
      align-items: center;
      gap: 12px;
      min-width: 0;
    }
    .grid-header {
      padding: 8px 12px;
      border-bottom: 1px solid var(--vsc-border, #454545);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 94%, black);
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 70%, transparent);
      text-transform: uppercase;
      font-size: 10px;
      letter-spacing: 0.08em;
      flex-shrink: 0;
    }
    .grid-body {
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
      overflow: auto;
    }
    .grid-row {
      padding: 4px 12px;
      border-bottom: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 35%, transparent);
    }
    .grid-row:hover {
      background: color-mix(in srgb, var(--vsc-list-hover-background, rgba(255,255,255,0.04)) 85%, transparent);
    }
    .offset-col {
      color: color-mix(in srgb, var(--vsc-accent-blue, #4da8ff) 72%, var(--vsc-foreground, #ccc));
      font-variant-numeric: tabular-nums;
    }
    .hex-col {
      display: grid;
      grid-template-columns: repeat(16, minmax(0, 1fr));
      gap: 6px;
      min-width: 0;
      font-variant-numeric: tabular-nums;
    }
    .hex-cell {
      text-align: center;
      color: var(--vsc-foreground, #ccc);
    }
    .hex-cell.zero {
      color: color-mix(in srgb, var(--vsc-foreground, #ccc) 35%, transparent);
    }
    .ascii-col {
      min-width: 0;
      color: color-mix(in srgb, var(--vsc-accent-green, #7ad97a) 72%, var(--vsc-foreground, #ccc));
      white-space: pre;
      overflow: hidden;
      text-overflow: clip;
    }
    .ascii-text {
      letter-spacing: 0.04em;
    }
    .state-container {
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      flex: 1 1 auto;
      height: 100%;
      min-height: 160px;
      gap: 8px;
      padding: 24px;
      text-align: center;
    }
    .state-icon {
      font-size: 36px;
      line-height: 1;
      opacity: 0.55;
    }
    .state-label {
      font-size: 13px;
      opacity: 0.65;
    }
    .state-sub {
      font-size: 11px;
      opacity: 0.4;
    }
    .state-error .state-icon {
      opacity: 1;
    }
    .state-error .state-label {
      color: #ff7070;
      opacity: 1;
    }
    .error-detail {
      font-size: 11px;
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      color: color-mix(in srgb, #ff7070 70%, var(--vsc-foreground, #ccc));
      background: rgba(255, 0, 0, 0.07);
      border: 1px solid rgba(255, 112, 112, 0.2);
      border-radius: 4px;
      padding: 8px 12px;
      max-width: 560px;
      white-space: pre-wrap;
      word-break: break-word;
    }
    .spinner {
      width: 26px;
      height: 26px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, #0078d4) 25%, transparent);
      border-top-color: var(--vsc-accent-blue, #0078d4);
      border-radius: 50%;
      animation: spin 0.65s linear infinite;
    }
    @keyframes spin {
      to { transform: rotate(360deg); }
    }
  `],
})
export class BinaryViewerComponent implements OnChanges {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
  @Input() probe: FileProbe | null = null;

  kind: BinaryViewerKind = 'empty';
  info: FileInfo | null = null;
  error = '';
  rows: BinaryRow[] = [];
  copiedPath = false;

  readonly headerLabels = Array.from({ length: BYTES_PER_ROW }, (_, index) => index.toString(16).toUpperCase().padStart(2, '0'));
  readonly maxPreviewBytes = MAX_BINARY_PREVIEW_BYTES;

  private loadId = 0;
  private copyResetTimer: number | null = null;

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
  ) {}

  get displayName(): string {
    if (this.info?.name) return this.info.name;
    const source = this.archiveContext?.entryPath ?? this.targetPath;
    return source ? this.baseName(source) : '';
  }

  get displayPath(): string {
    if (this.archiveContext) {
      return `${this.archiveContext.fsPath} :: ${this.archiveContext.entryPath}`;
    }
    return this.targetPath;
  }

  get typeBadge(): string {
    const mime = (this.probe?.mimeType ?? '').trim();
    if (mime) return mime;
    const ext = this.extractExt(this.archiveContext?.entryPath ?? this.targetPath);
    return ext ? ext.toUpperCase() : 'BINARY';
  }

  get byteLength(): number {
    return this.rows.reduce((total, row) => total + row.cells.length, 0);
  }

  ngOnChanges(): void {
    void this.reload();
  }

  async openExternally(): Promise<void> {
    if (!this.targetPath || this.archiveContext) return;
    await this.api.openFile(this.targetPath);
  }

  async copyPath(): Promise<void> {
    const path = this.displayPath?.trim();
    if (!path) return;

    await navigator.clipboard.writeText(path).catch(() => {});
    this.copiedPath = true;
    if (this.copyResetTimer !== null) {
      window.clearTimeout(this.copyResetTimer);
    }
    this.copyResetTimer = window.setTimeout(() => {
      this.copiedPath = false;
      this.copyResetTimer = null;
      this.cdr.markForCheck();
    }, 1200);
    this.cdr.markForCheck();
  }

  formatSize(bytes: number): string {
    if (!bytes || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / Math.pow(1024, i);
    return `${i === 0 ? value : value.toFixed(1)} ${units[i]}`;
  }

  private async reload(): Promise<void> {
    const path = (this.targetPath ?? '').trim();
    const currentLoad = ++this.loadId;

    this.error = '';
    this.info = null;
    this.rows = [];

    if (!path && !this.archiveContext) {
      this.kind = 'empty';
      this.cdr.markForCheck();
      return;
    }

    this.kind = 'loading';
    this.cdr.markForCheck();

    try {
      const result = this.archiveContext
        ? await this.loadFromArchive(this.archiveContext)
        : await this.loadFromDisk(path);

      if (currentLoad !== this.loadId) return;

      if (result.kind === 'directory') {
        this.kind = 'directory';
        this.cdr.markForCheck();
        return;
      }

      if (result.size > MAX_BINARY_PREVIEW_BYTES) {
        this.kind = 'toobig';
        this.cdr.markForCheck();
        return;
      }

      this.rows = this.buildRows(this.decodeBase64(result.content));
      this.kind = 'grid';
      this.cdr.markForCheck();
    } catch (error) {
      if (currentLoad !== this.loadId) return;
      this.kind = 'error';
      this.error = error instanceof Error ? error.message : String(error);
      this.cdr.markForCheck();
    }
  }

  private async loadFromDisk(path: string): Promise<{ kind: 'grid' | 'directory'; content: string; size: number }> {
    const info = await this.api.getFileInfo(path);
    this.info = info;

    if (info.type === FileType.DIRECTORY) {
      return { kind: 'directory', content: '', size: 0 };
    }

    if (info.size > MAX_BINARY_PREVIEW_BYTES) {
      return { kind: 'grid', content: '', size: info.size };
    }

    const file = await this.api.readFile(path, 'base64');
    return {
      kind: 'grid',
      content: file.content,
      size: file.size ?? info.size,
    };
  }

  private async loadFromArchive(ctx: ViewerArchiveContext): Promise<{ kind: 'grid'; content: string; size: number }> {
    const response = await this.api.archiveRead(ctx.fsPath, ctx.nestedChain, ctx.entryPath, 'base64');
    const entryName = this.baseName(ctx.entryPath);
    this.info = {
      name: entryName,
      path: ctx.entryPath,
      type: FileType.FILE,
      size: response.size ?? 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
    };

    return {
      kind: 'grid',
      content: response.content,
      size: response.size ?? 0,
    };
  }

  private decodeBase64(content: string): Uint8Array {
    const decoded = atob(content ?? '');
    const bytes = new Uint8Array(decoded.length);
    for (let index = 0; index < decoded.length; index++) {
      bytes[index] = decoded.charCodeAt(index);
    }
    return bytes;
  }

  private buildRows(bytes: Uint8Array): BinaryRow[] {
    const rows: BinaryRow[] = [];
    for (let offset = 0; offset < bytes.length; offset += BYTES_PER_ROW) {
      const slice = bytes.subarray(offset, offset + BYTES_PER_ROW);
      const cells = Array.from(slice, (value) => ({
        hex: value.toString(16).toUpperCase().padStart(2, '0'),
        ascii: value >= 32 && value <= 126 ? String.fromCharCode(value) : '.',
      }));
      rows.push({
        offsetHex: offset.toString(16).toUpperCase().padStart(8, '0'),
        cells,
        ascii: cells.map((cell) => cell.ascii).join(''),
      });
    }
    return rows;
  }

  private baseName(path: string): string {
    return path.replace(/\\/g, '/').split('/').pop() || path;
  }

  private extractExt(path: string): string {
    const normalized = path.replace(/\\/g, '/');
    const base = normalized.split('/').pop() || normalized;
    const index = base.lastIndexOf('.');
    return index >= 0 ? base.slice(index + 1).toLowerCase() : '';
  }
}
