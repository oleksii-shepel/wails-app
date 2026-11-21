import { writeClipboardText } from '../../utils/clipboard-text';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  Input,
  OnChanges,
  OnDestroy,
  ViewChild,
} from '@angular/core';
import { ApiService } from '../../services/api.service';
import { FileInfo, FileProbe, FileType } from '../../shared/protocol';
import { ViewerArchiveContext } from '../viewer-archive-context';
import { VIEWER_BASE_STYLES, VIEWER_PRODUCTION_CHROME_STYLES } from './builtin-viewer.shared';

type BinaryViewerKind = 'empty' | 'loading' | 'grid' | 'directory' | 'error';
type OffsetFormat = 'hex' | 'dec';

interface HexCell { hex: string; ascii: string; zero: boolean; printable: boolean; }
interface BinaryRow { offsetStr: string; cells: HexCell[]; ascii: string; }

// px height of each .grid-row вЂ” must match the CSS .grid-row { height } value
const ROW_HEIGHT = 28;
const OVERSCAN_ROWS = 40;
const CHUNK_BYTES = 16 * 1024;             // 16 KB fetch granularity
const MEMORY_THRESHOLD = 8 * 1024 * 1024;  // files в‰¤ 8 MB loaded fully into memory

@Component({
  selector: 'app-binary-viewer',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <div class="viewer-toolbar" *ngIf="kind === 'grid'">
        <button class="tool-btn" type="button" (click)="copyPath()" [class.active]="copiedPath">
          {{ copiedPath ? 'Copied' : 'Copy path' }}
        </button>
        <button class="tool-btn" type="button" *ngIf="!archiveContext && targetPath" (click)="openExternally()">
          Open externally
        </button>

        <span class="toolbar-divider"></span>

        <span class="toolbar-label" id="hex-width-label">Width</span>
        <div class="seg-group" role="group" aria-labelledby="hex-width-label">
          <button class="seg-btn" type="button" [class.active]="bytesPerRow === 8" [attr.aria-pressed]="bytesPerRow === 8"  (click)="setBytesPerRow(8)">8</button>
          <button class="seg-btn" type="button" [class.active]="bytesPerRow === 16" [attr.aria-pressed]="bytesPerRow === 16" (click)="setBytesPerRow(16)">16</button>
          <button class="seg-btn" type="button" [class.active]="bytesPerRow === 32" [attr.aria-pressed]="bytesPerRow === 32" (click)="setBytesPerRow(32)">32</button>
        </div>

        <span class="toolbar-divider"></span>

        <span class="toolbar-label" id="hex-offset-label">Offset</span>
        <div class="seg-group" role="group" aria-labelledby="hex-offset-label">
          <button class="seg-btn" type="button" [class.active]="offsetFormat === 'hex'" [attr.aria-pressed]="offsetFormat === 'hex'" (click)="setOffsetFormat('hex')">HEX</button>
          <button class="seg-btn" type="button" [class.active]="offsetFormat === 'dec'" [attr.aria-pressed]="offsetFormat === 'dec'" (click)="setOffsetFormat('dec')">DEC</button>
        </div>

        <span class="toolbar-divider"></span>

        <label class="toolbar-label" for="hex-goto-input">Go to</label>
        <input class="goto-input" id="hex-goto-input" type="text" [(ngModel)]="gotoValue"
          placeholder="{{ offsetFormat === 'hex' ? '0x0000' : '0' }}"
          [attr.aria-invalid]="!!gotoError" [attr.aria-describedby]="gotoError ? 'hex-goto-error' : null"
          (ngModelChange)="gotoError = ''"
          (keydown.enter)="gotoOffset(); $event.preventDefault(); $event.stopPropagation()" />
        <button class="tool-btn" type="button" (click)="gotoOffset()" title="Jump to offset" aria-label="Jump to offset">&#x2192;</button>
        <span *ngIf="gotoError" class="error-detail" id="hex-goto-error" role="status">{{ gotoError }}</span>

        <span class="toolbar-divider"></span>

        <button class="tool-btn" type="button" (click)="copyHexDump()" [disabled]="!renderedRows.length" [class.active]="copiedHex">
          {{ copiedHex ? 'Copied!' : 'Copy hex' }}
        </button>
      </div>

      <!-- в”Ђв”Ђ body в”Ђв”Ђ -->
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
        <div *ngIf="kind === 'error'" class="state-container state-error" role="alert">
          <div class="state-icon">&#x26A0;</div>
          <div class="state-label">Failed to load binary preview</div>
          <div class="error-detail">{{ error }}</div>
        </div>

        <!-- virtual hex grid вЂ” only mounted when kind === 'grid' -->
        <div *ngIf="kind === 'grid'" class="grid-shell" [style.--cols]="bytesPerRow">
          <!-- sticky column header -->
          <div class="grid-header">
            <div class="offset-col">{{ offsetFormat === 'hex' ? 'Offset h' : 'Offset d' }}</div>
            <div class="hex-col"><span *ngFor="let lbl of headerLabels">{{ lbl }}</span></div>
            <div class="ascii-col">ASCII</div>
          </div>
          <!-- scrollable body -->
          <div class="grid-body" #gridBody>
            <!-- spacer sets total scroll height -->
            <div class="grid-spacer" [style.height.px]="totalRows * ROW_HEIGHT">
              <!-- rendered window вЂ” translated to its position in the scroll space -->
              <div class="grid-viewport" [style.transform]="'translateY(' + (renderedStartRow * ROW_HEIGHT) + 'px)'">
                <div class="grid-row" *ngFor="let row of renderedRows; trackBy: trackRow">
                  <div class="offset-col">{{ row.offsetStr }}</div>
                  <div class="hex-col">
                    <span class="hex-cell"
                      *ngFor="let cell of row.cells"
                      [class.zero]="cell.zero"
                      [class.print]="cell.printable">{{ cell.hex }}</span>
                  </div>
                  <div class="ascii-col ascii-text">{{ row.ascii }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [VIEWER_BASE_STYLES, `
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
      background: var(--vsc-code-background, var(--vsc-editor-background, var(--vsc-editor-background)));
      color: var(--vsc-foreground, var(--vsc-foreground));
    }
    .viewer-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
      padding: 8px 12px;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-border));
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 92%, black);
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
      color: var(--vsc-accent-orange, var(--vsc-warning));
      background: color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-accent-yellow)) 16%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-accent-yellow)) 36%, transparent);
      flex-shrink: 0;
    }
    .header-text { min-width: 0; flex: 1; }
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
    .header-meta { display: flex; align-items: center; gap: 4px; flex-shrink: 0; }
    .badge {
      font-size: 10px;
      font-weight: 600;
      padding: 2px 6px;
      border-radius: 3px;
      white-space: nowrap;
      letter-spacing: 0.3px;
    }
    .archive-badge {
      background: color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-accent-orange)) 20%, transparent);
      color: var(--vsc-accent-orange, var(--vsc-warning));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-accent-orange)) 40%, transparent);
    }
    .type-badge {
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 40%, transparent);
    }
    .size-badge {
      background: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 8%, transparent);
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 60%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 50%, transparent);
    }

    /* toolbar */
    .viewer-toolbar {
      display: flex;
      align-items: center;
      gap: 4px;
      padding: 4px 8px;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-border));
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 96%, black);
      flex-shrink: 0;
      flex-wrap: wrap;
    }
    .tool-btn {
      background: transparent;
      border: 1px solid transparent;
      color: var(--vsc-foreground, var(--vsc-foreground));
      font: inherit;
      font-size: 12px;
      padding: 2px 8px;
      border-radius: 3px;
      cursor: pointer;
      line-height: 1.4;
      white-space: nowrap;
      transition: background 0.1s, border-color 0.1s;
    }
    .tool-btn:hover {
      background: var(--vsc-list-hover-background, var(--vsc-neutral-overlay-subtle));
      border-color: var(--vsc-border, var(--vsc-border));
    }
    .tool-btn.active {
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent);
      border-color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 50%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
    }
    .toolbar-divider {
      width: 1px;
      height: 16px;
      background: var(--vsc-border, var(--vsc-border));
      margin: 0 2px;
      flex-shrink: 0;
    }
    .toolbar-label {
      font-size: 11px;
      opacity: 0.55;
      white-space: nowrap;
    }
    /* segmented button group */
    .seg-group {
      display: flex;
      border: 1px solid var(--vsc-border, var(--vsc-border));
      border-radius: 4px;
      overflow: hidden;
    }
    .seg-btn {
      background: transparent;
      border: none;
      border-right: 1px solid var(--vsc-border, var(--vsc-border));
      color: var(--vsc-foreground, var(--vsc-foreground));
      font: inherit;
      font-size: 11px;
      padding: 2px 7px;
      cursor: pointer;
      white-space: nowrap;
      transition: background 0.1s;
    }
    .seg-btn:last-child { border-right: none; }
    .seg-btn:hover { background: var(--vsc-list-hover-background, var(--vsc-neutral-overlay-subtle)); }
    .seg-btn.active {
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 25%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
    }
    /* go-to input */
    .goto-input {
      background: var(--vsc-input-background, var(--vsc-control-background));
      border: 1px solid var(--vsc-input-border, var(--vsc-border));
      color: var(--vsc-foreground, var(--vsc-foreground));
      font: inherit;
      font-size: 12px;
      font-family: var(--vsc-font-family-mono, monospace);
      padding: 2px 7px;
      border-radius: 3px;
      width: 90px;
      outline: none;
    }
    .goto-input:focus { border-color: var(--vsc-input-focus-border, var(--vsc-accent-blue)); }

    /* body */
    .viewer-body {
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
      display: flex;
      overflow: hidden;
    }

    /* hex grid */
    .grid-shell {
      display: flex;
      flex-direction: column;
      flex: 1 1 auto;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      font-family: var(--vsc-font-family-mono, ui-monospace, 'Cascadia Code', 'SFMono-Regular', Consolas, monospace);
      font-size: 12px;
    }
    .grid-header,
    .grid-row {
      display: grid;
      grid-template-columns: 88px minmax(0, 1fr) calc(var(--cols, 16) * 1ch + 2ch);
      align-items: center;
      gap: 12px;
    }
    .grid-header {
      padding: 6px 12px;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-border));
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 94%, black);
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 65%, transparent);
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
      position: relative;
    }
    .grid-spacer { width: 100%; }
    .grid-viewport { position: absolute; top: 0; left: 0; right: 0; will-change: transform; }
    .grid-row {
      height: 28px;
      padding: 0 12px;
      border-bottom: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 30%, transparent);
      box-sizing: border-box;
    }
    .grid-row:hover {
      background: color-mix(in srgb, var(--vsc-list-hover-background, var(--vsc-neutral-overlay-subtle)) 85%, transparent);
    }
    .offset-col {
      color: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-ring)) 72%, var(--vsc-foreground, var(--vsc-foreground)));
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
    }
    .hex-col {
      display: grid;
      grid-template-columns: repeat(var(--cols, 16), minmax(0, 1fr));
      gap: 4px;
      min-width: 0;
      font-variant-numeric: tabular-nums;
    }
    .hex-cell { text-align: center; color: var(--vsc-foreground, var(--vsc-foreground)); }
    .hex-cell.zero { color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 28%, transparent); }
    .hex-cell.print { color: color-mix(in srgb, var(--vsc-accent-green, var(--vsc-success)) 80%, var(--vsc-foreground, var(--vsc-foreground))); }
    .ascii-col {
      min-width: 0;
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 55%, transparent);
      white-space: pre;
      overflow: hidden;
      text-overflow: clip;
    }
    .hex-cell.print ~ .ascii-col,
    .ascii-text {
      letter-spacing: 0.04em;
    }

    /* states */
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
    .state-icon { font-size: 36px; line-height: 1; opacity: 0.55; }
    .state-label { font-size: 13px; opacity: 0.65; }
    .state-error .state-icon { opacity: 1; }
    .state-error .state-label { color: var(--vsc-error); opacity: 1; }
    .error-detail {
      font-size: 11px;
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      color: color-mix(in srgb, var(--vsc-error-bright) 70%, var(--vsc-foreground, var(--vsc-foreground)));
      background: var(--vsc-error-surface);
      border: 1px solid var(--vsc-error-border);
      border-radius: 4px;
      padding: 8px 12px;
      max-width: 560px;
      white-space: pre-wrap;
      word-break: break-word;
    }
    .spinner {
      width: 26px;
      height: 26px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 25%, transparent);
      border-top-color: var(--vsc-accent-blue, var(--vsc-accent-blue));
      border-radius: 50%;
      animation: spin 0.65s linear infinite;
    }
    @keyframes spin { to { transform: rotate(360deg); } }
  `, VIEWER_PRODUCTION_CHROME_STYLES],
})
export class BinaryViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
  @Input() probe: FileProbe | null = null;

  @ViewChild('gridBody')
  set gridBodyRef(value: ElementRef<HTMLDivElement> | undefined) {
    if (this._scrollUnlisten) {
      this._scrollUnlisten();
      this._scrollUnlisten = null;
    }
    this._gridBody = value?.nativeElement ?? null;
    if (this._gridBody) {
      const el = this._gridBody;
      const handler = () => this.scheduleUpdate();
      el.addEventListener('scroll', handler, { passive: true });
      this._scrollUnlisten = () => el.removeEventListener('scroll', handler);
      this.scheduleUpdate();
    }
  }

  kind: BinaryViewerKind = 'empty';
  info: FileInfo | null = null;
  error = '';
  copiedPath = false;
  copiedHex = false;

  // viewport state
  readonly ROW_HEIGHT = ROW_HEIGHT;
  totalRows = 0;
  totalFileSize = 0;
  renderedStartRow = 0;
  renderedRows: BinaryRow[] = [];
  bytesPerRow: 8 | 16 | 32 = 16;
  offsetFormat: OffsetFormat = 'hex';
  gotoValue = '';
  gotoError = '';

  get headerLabels(): string[] {
    return Array.from({ length: this.bytesPerRow }, (_, i) => i.toString(16).toUpperCase().padStart(2, '0'));
  }

  // internals
  private _gridBody: HTMLDivElement | null = null;
  private _scrollUnlisten: (() => void) | null = null;
  private _rafId: number | null = null;

  private memBytes: Uint8Array | null = null;   // small files / archives: full bytes in memory
  private isRangeMode = false;                   // large disk files: fetch by range
  private streamUrl = '';                        // HTTP stream URL for disk files (range mode)
  private chunkCache = new Map<number, Uint8Array>();   // chunkIndex в†’ bytes
  private pendingChunks = new Set<number>();

  private loadId = 0;
  private copyResetTimer: number | null = null;
  private copyHexResetTimer: number | null = null;

  constructor(
    private readonly api: ApiService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  // в”Ђв”Ђ getters в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  get displayName(): string {
    if (this.info?.name) return this.info.name;
    const source = this.archiveContext?.entryPath ?? this.targetPath;
    return source ? this.baseName(source) : '';
  }

  get displayPath(): string {
    return this.archiveContext
      ? `${this.archiveContext.fsPath} :: ${this.archiveContext.entryPath}`
      : this.targetPath;
  }

  get typeBadge(): string {
    const mime = (this.probe?.mimeType ?? '').trim();
    if (mime) return mime;
    const ext = this.extractExt(this.archiveContext?.entryPath ?? this.targetPath);
    return ext ? ext.toUpperCase() : 'BINARY';
  }

  // в”Ђв”Ђ lifecycle в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  ngOnChanges(): void { void this.reload(); }

  ngOnDestroy(): void {
    if (this._scrollUnlisten) this._scrollUnlisten();
    if (this._rafId !== null) cancelAnimationFrame(this._rafId);
    if (this.copyResetTimer !== null) clearTimeout(this.copyResetTimer);
    if (this.copyHexResetTimer !== null) clearTimeout(this.copyHexResetTimer);
  }

  // в”Ђв”Ђ toolbar actions в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  setBytesPerRow(n: 8 | 16 | 32): void {
    if (this.bytesPerRow === n) return;
    this.bytesPerRow = n;
    this.totalRows = Math.ceil(this.totalFileSize / n);
    this.renderedRows = [];
    this.cdr.markForCheck();
    this.scheduleUpdate();
  }

  setOffsetFormat(fmt: OffsetFormat): void {
    if (this.offsetFormat === fmt) return;
    this.offsetFormat = fmt;
    this.scheduleUpdate();
  }

  gotoOffset(): void {
    const raw = this.gotoValue.trim();
    if (!raw || !this._gridBody) return;
    const hex = /^0x/i.test(raw) || this.offsetFormat === 'hex';
    const digits = raw.replace(/^0x/i, '');
    if (!(hex ? /^[0-9a-f]+$/i : /^\d+$/).test(digits)) {
      this.gotoError = hex ? 'Enter a hexadecimal offset.' : 'Enter a decimal or 0x-prefixed offset.';
      return;
    }
    const offset = Number.parseInt(digits, hex ? 16 : 10);
    if (!Number.isSafeInteger(offset) || offset >= this.totalFileSize) {
      this.gotoError = 'The offset is beyond the end of this file.';
      return;
    }
    this.gotoError = '';
    const row = Math.max(0, Math.min(Math.floor(offset / this.bytesPerRow), this.totalRows - 1));
    this._gridBody.scrollTop = row * ROW_HEIGHT;
  }

  async copyPath(): Promise<void> {
    const path = this.displayPath?.trim();
    if (!path) return;
    const copied = await writeClipboardText(path).then(() => true, () => false);
    if (!copied) return;
    this.copiedPath = true;
    if (this.copyResetTimer !== null) clearTimeout(this.copyResetTimer);
    this.copyResetTimer = window.setTimeout(() => {
      this.copiedPath = false; this.copyResetTimer = null; this.cdr.markForCheck();
    }, 1200);
    this.cdr.markForCheck();
  }

  async copyHexDump(): Promise<void> {
    if (!this.renderedRows.length) return;
    const lines = this.renderedRows.map(row => {
      const hex = row.cells.map(c => c.hex).join(' ');
      return `${row.offsetStr}  ${hex}  ${row.ascii}`;
    });
    const copied = await writeClipboardText(lines.join('\n')).then(() => true, () => false);
    if (!copied) return;
    this.copiedHex = true;
    if (this.copyHexResetTimer !== null) clearTimeout(this.copyHexResetTimer);
    this.copyHexResetTimer = window.setTimeout(() => {
      this.copiedHex = false; this.copyHexResetTimer = null; this.cdr.markForCheck();
    }, 1200);
    this.cdr.markForCheck();
  }

  async openExternally(): Promise<void> {
    if (!this.targetPath || this.archiveContext) return;
    await this.api.openFile(this.targetPath);
  }

  formatSize(bytes: number): string {
    if (!bytes || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB'];
    const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / Math.pow(1024, i);
    return `${i === 0 ? value : value.toFixed(1)} ${units[i]}`;
  }

  trackRow(_: number, row: BinaryRow): string { return row.offsetStr; }

  // в”Ђв”Ђ virtual scroll в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  private scheduleUpdate(): void {
    if (this._rafId !== null) return;
    this._rafId = requestAnimationFrame(() => {
      this._rafId = null;
      const el = this._gridBody;
      if (el && el.clientHeight === 0) { this.scheduleUpdate(); return; } // layout not settled
      void this.updateViewport();
    });
  }

  private async updateViewport(): Promise<void> {
    const body = this._gridBody;
    if (!body || this.kind !== 'grid') return;

    const scrollTop = body.scrollTop;
    const height = body.clientHeight || 600;
    const firstVisible = Math.floor(scrollTop / ROW_HEIGHT);
    const visibleCount = Math.ceil(height / ROW_HEIGHT);
    const renderStart = Math.max(0, firstVisible - OVERSCAN_ROWS);
    const renderEnd = Math.min(this.totalRows, firstVisible + visibleCount + OVERSCAN_ROWS + 1);
    if (renderEnd <= renderStart) return;

    // в”Ђв”Ђ memory mode в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
    if (!this.isRangeMode) {
      if (!this.memBytes) return;
      const byteStart = renderStart * this.bytesPerRow;
      const byteEnd = Math.min(renderEnd * this.bytesPerRow, this.totalFileSize);
      this.renderedStartRow = renderStart;
      this.renderedRows = this.buildRows(this.memBytes.subarray(byteStart, byteEnd), byteStart);
      this.cdr.markForCheck();
      return;
    }

    // в”Ђв”Ђ range mode в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ
    const byteStart = renderStart * this.bytesPerRow;
    const byteEnd = Math.min(renderEnd * this.bytesPerRow, this.totalFileSize);
    const chunkStart = Math.floor(byteStart / CHUNK_BYTES);
    const chunkEnd = Math.floor((byteEnd - 1) / CHUNK_BYTES);
    const currentLoad = this.loadId;

    const missing: number[] = [];
    for (let ci = chunkStart; ci <= chunkEnd; ci++) {
      if (!this.chunkCache.has(ci) && !this.pendingChunks.has(ci)) missing.push(ci);
    }

    // Render immediately with whatever we have (missing chunks show as 00)
    this.renderedStartRow = renderStart;
    this.renderedRows = this.assembleRows(renderStart, renderEnd);
    this.cdr.markForCheck();

    if (missing.length === 0) return;

    // Fetch missing chunks (max 8 at a time)
    await Promise.all(missing.slice(0, 8).map(ci => this.fetchChunk(ci, currentLoad)));
    if (this.loadId !== currentLoad) return;

    this.renderedRows = this.assembleRows(renderStart, renderEnd);
    this.cdr.markForCheck();
  }

  private async fetchChunk(chunkIndex: number, loadId: number): Promise<void> {
    if (this.chunkCache.has(chunkIndex) || this.pendingChunks.has(chunkIndex)) return;
    this.pendingChunks.add(chunkIndex);
    try {
      const offset = chunkIndex * CHUNK_BYTES;
      const length = Math.min(CHUNK_BYTES, this.totalFileSize - offset);
      if (length <= 0) return;
      const resp = await fetch(this.streamUrl, {
        headers: { Range: `bytes=${offset}-${offset + length - 1}` },
      });
      if (!resp.ok && resp.status !== 206) throw new Error(`HTTP ${resp.status}`);
      const buf = await resp.arrayBuffer();
      if (this.loadId !== loadId) return;
      this.chunkCache.set(chunkIndex, new Uint8Array(buf));
    } finally {
      this.pendingChunks.delete(chunkIndex);
    }
  }

  /** Build rows from the chunk cache for a row range [startRow, endRow). */
  private assembleRows(startRow: number, endRow: number): BinaryRow[] {
    const dataStart = startRow * this.bytesPerRow;
    const dataEnd = Math.min(endRow * this.bytesPerRow, this.totalFileSize);
    const totalBytes = dataEnd - dataStart;
    if (totalBytes <= 0) return [];

    const combined = new Uint8Array(totalBytes); // zero-filled = '00' placeholder
    const firstChunk = Math.floor(dataStart / CHUNK_BYTES);
    const lastChunk = Math.floor((dataEnd - 1) / CHUNK_BYTES);

    for (let ci = firstChunk; ci <= lastChunk; ci++) {
      const chunk = this.chunkCache.get(ci);
      if (!chunk) continue;
      const chunkBase = ci * CHUNK_BYTES;
      const srcStart = Math.max(0, dataStart - chunkBase);
      const dstStart = Math.max(0, chunkBase - dataStart);
      const copyEnd = Math.min(chunkBase + chunk.length, dataEnd);
      const copyLen = copyEnd - Math.max(chunkBase, dataStart);
      if (copyLen > 0) combined.set(chunk.subarray(srcStart, srcStart + copyLen), dstStart);
    }

    return this.buildRows(combined, dataStart);
  }

  // в”Ђв”Ђ loading в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  private async reload(): Promise<void> {
    const path = (this.targetPath ?? '').trim();
    const currentLoad = ++this.loadId; // invalidate in-flight work

    this.error = '';
    this.info = null;
    this.memBytes = null;
    this.isRangeMode = false;
    this.streamUrl = '';
    this.chunkCache.clear();
    this.pendingChunks.clear();
    this.totalRows = 0;
    this.totalFileSize = 0;
    this.renderedStartRow = 0;
    this.renderedRows = [];

    if (!path && !this.archiveContext) {
      this.kind = 'empty';
      this.cdr.markForCheck();
      return;
    }

    this.kind = 'loading';
    this.cdr.markForCheck();

    try {
      if (this.archiveContext) {
        await this.loadFromArchive(this.archiveContext, currentLoad);
      } else {
        await this.loadFromDisk(path, currentLoad);
      }
    } catch (err) {
      if (currentLoad !== this.loadId) return;
      this.kind = 'error';
      this.error = err instanceof Error ? err.message : String(err);
      this.cdr.markForCheck();
    }
  }

  private async loadFromDisk(path: string, loadId: number): Promise<void> {
    const info = await this.api.getFileInfo(path);
    if (loadId !== this.loadId) return;
    this.info = info;

    if (info.type === FileType.DIRECTORY) {
      this.kind = 'directory';
      this.cdr.markForCheck();
      return;
    }

    this.totalFileSize = info.size;
    this.totalRows = Math.ceil(info.size / this.bytesPerRow);

    const url = await this.api.getStreamURL(path);
    if (loadId !== this.loadId) return;

    if (info.size <= MEMORY_THRESHOLD) {
      const resp = await fetch(url);
      if (!resp.ok) throw new Error(`HTTP ${resp.status}: ${resp.statusText}`);
      const buf = await resp.arrayBuffer();
      if (loadId !== this.loadId) return;
      this.memBytes = new Uint8Array(buf);
      this.isRangeMode = false;
    } else {
      // Large file — use on-demand HTTP Range reads
      this.isRangeMode = true;
      this.streamUrl = url;
      // Pre-fetch the first chunk so the initial view renders immediately
      await this.fetchChunk(0, loadId);
      if (loadId !== this.loadId) return;
    }

    this.kind = 'grid';
    this.cdr.markForCheck();
    this.scheduleUpdate();
  }

  private async loadFromArchive(ctx: ViewerArchiveContext, loadId: number): Promise<void> {
    const streamURL = await this.api.getArchiveStreamURL(ctx.fsPath, ctx.nestedChain, ctx.entryPath);
    const bytes = await this.api.fetchBytesFromURL(streamURL);
    if (loadId !== this.loadId) return;
    const size = bytes.byteLength;
    this.info = {
      name: this.baseName(ctx.entryPath), path: ctx.entryPath,
      type: FileType.FILE, size,
      created: 0, modified: 0, accessed: 0, permissions: '', isHidden: false,
    };
    this.totalFileSize = size;
    this.totalRows = Math.ceil(size / this.bytesPerRow);
    this.memBytes = bytes;
    this.isRangeMode = false;
    this.kind = 'grid';
    this.cdr.markForCheck();
    this.scheduleUpdate();
  }

  // в”Ђв”Ђ helpers в”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђв”Ђ

  private buildRows(bytes: Uint8Array, baseOffset: number): BinaryRow[] {
    const bpr = this.bytesPerRow;
    const rows: BinaryRow[] = [];
    for (let i = 0; i < bytes.length; i += bpr) {
      const offset = baseOffset + i;
      const slice = bytes.subarray(i, i + bpr);
      const cells: HexCell[] = Array.from(slice, v => ({
        hex: v.toString(16).toUpperCase().padStart(2, '0'),
        ascii: v >= 32 && v <= 126 ? String.fromCharCode(v) : '.',
        zero: v === 0,
        printable: v >= 32 && v <= 126,
      }));
      const offsetStr = this.offsetFormat === 'hex'
        ? offset.toString(16).toUpperCase().padStart(8, '0')
        : offset.toString(10).padStart(10, ' ');
      rows.push({ offsetStr, cells, ascii: cells.map(c => c.ascii).join('') });
    }
    return rows;
  }

  private baseName(path: string): string {
    return path.replace(/\\/g, '/').split('/').pop() || path;
  }

  private extractExt(path: string): string {
    const base = path.replace(/\\/g, '/').split('/').pop() || path;
    const idx = base.lastIndexOf('.');
    return idx >= 0 ? base.slice(idx + 1).toLowerCase() : '';
  }
}