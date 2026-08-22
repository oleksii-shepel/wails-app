import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, Input, OnChanges, OnDestroy } from '@angular/core';
import { VscodeScrollbarDirective } from '../../directives/vscode-scrollbar.directive';
import { ApiService } from '../../services/api.service';
import { FileInfo, FileProbe, FileType } from '../../shared/protocol';
import { ViewerArchiveContext } from '../viewer-archive-context';
import {
  VIEWER_PRODUCTION_CHROME_STYLES,
  VIEWER_BASE_STYLES,
  baseName,
  buildDisplayName,
  extractExt,
  formatSize,
  makeSyntheticInfo,
} from './builtin-viewer.shared';

type FontViewerKind = 'empty' | 'loading' | 'ready' | 'directory' | 'error';
type SampleMode = 'sentence' | 'alphabet' | 'symbols';

const PREVIEW_SIZES = [40, 72, 120] as const;
const FONT_EXTENSIONS = new Set(['ttf', 'ttc', 'otf', 'otc', 'woff', 'woff2', 'eot']);
const GLYPH_ROWS = [
  'ABCDEFGHIJKLMNOPQRSTUVWXYZ',
  'abcdefghijklmnopqrstuvwxyz',
  '0123456789',
  '!@#$%^&*()_+-=[]{};:\",.<>/?\\|',
  'АБВГҐДЕЄЖЗИІЇЙКЛМНОПРСТУФХЦЧШЩЬЮЯ',
  'абвгґдеєжзиіїйклмнопрстуфхцчшщьюя',
];

@Component({
  selector: 'app-font-viewer',
  standalone: true,
  imports: [CommonModule, VscodeScrollbarDirective],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <div class="viewer-header">
        <div class="header-main">
          <span class="file-icon" aria-hidden="true">🔤</span>
          <div class="header-text">
            <div class="title" [title]="displayName">{{ displayName || 'Font Preview' }}</div>
            <div class="path" [title]="displayPath">{{ displayPath || 'Select a font to preview' }}</div>
          </div>
        </div>

        <div class="header-meta" *ngIf="info && kind !== 'empty' && kind !== 'loading'">
          <span class="badge archive-badge" *ngIf="archiveContext" title="Loaded from archive">📦 archive</span>
          <span class="badge type-badge">{{ typeBadge }}</span>
          <span class="badge size-badge" *ngIf="info.size > 0">{{ formatSize(info.size) }}</span>
          <span class="badge meta-badge" *ngIf="mimeLabel">{{ mimeLabel }}</span>
        </div>
      </div>

      <div class="viewer-toolbar" *ngIf="kind === 'ready'">
        <button
          *ngFor="let size of previewSizes"
          class="tool-btn"
          type="button"
          [class.active]="previewSize === size"
          (click)="setPreviewSize(size)">
          {{ size }}px
        </button>

        <span class="toolbar-divider"></span>

        <button class="tool-btn" type="button" [class.active]="sampleMode === 'sentence'" (click)="setSampleMode('sentence')">
          Sentence
        </button>
        <button class="tool-btn" type="button" [class.active]="sampleMode === 'alphabet'" (click)="setSampleMode('alphabet')">
          Alphabet
        </button>
        <button class="tool-btn" type="button" [class.active]="sampleMode === 'symbols'" (click)="setSampleMode('symbols')">
          Symbols
        </button>
      </div>

      <div class="viewer-body" appVscodeScrollbar>
        <div *ngIf="kind === 'loading'" class="state-container">
          <div class="spinner"></div>
          <div class="state-label">Loading font preview...</div>
        </div>

        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon">🔎</div>
          <div class="state-label">Select a font to preview</div>
        </div>

        <div *ngIf="kind === 'directory'" class="state-container">
          <div class="state-icon">📁</div>
          <div class="state-label">Directory - no preview available</div>
        </div>

        <div *ngIf="kind === 'error'" class="state-container state-error">
          <div class="state-icon">⚠</div>
          <div class="state-label">Font preview failed</div>
          <div class="state-sub">The browser could not decode this font file.</div>
          <div class="error-detail">{{ error }}</div>
        </div>

        <div *ngIf="kind === 'ready'" class="font-layout">
          <section class="font-hero panel" [style.font-family]="fontStack">
            <div class="hero-kicker">Font Preview</div>
            <div class="hero-sample" [style.font-size.px]="previewSize">{{ activeSample }}</div>
            <div class="hero-caption">{{ subtitleSample }}</div>
          </section>

          <section class="font-sheet panel" [style.font-family]="fontStack">
            <div class="sheet-header">
              <div class="sheet-title">Character Sheet</div>
              <div class="sheet-sub">Use this to quickly check coverage and rendering quality.</div>
            </div>

            <div class="glyph-grid">
              <div class="glyph-row" *ngFor="let row of glyphRows">
                <span class="glyph-line" [style.font-size.px]="glyphSize">{{ row }}</span>
              </div>
            </div>
          </section>

          <section class="font-meta panel">
            <div class="sheet-title">Details</div>
            <div class="meta-list">
              <div class="meta-row">
                <span class="meta-label">Name</span>
                <span class="meta-value">{{ displayName || 'Unknown' }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-label">Format</span>
                <span class="meta-value">{{ typeBadge }}</span>
              </div>
              <div class="meta-row" *ngIf="mimeLabel">
                <span class="meta-label">MIME</span>
                <span class="meta-value meta-value--mono">{{ mimeLabel }}</span>
              </div>
              <div class="meta-row" *ngIf="info?.size">
                <span class="meta-label">Size</span>
                <span class="meta-value">{{ formatSize(info.size) }}</span>
              </div>
              <div class="meta-row">
                <span class="meta-label">Source</span>
                <span class="meta-value">{{ archiveContext ? 'Archive entry' : 'Filesystem file' }}</span>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  `,
  styles: [`
    ${VIEWER_BASE_STYLES}

    .viewer-body {
      background:
        radial-gradient(circle at top right, color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 11%, transparent), transparent 34%),
        linear-gradient(180deg, color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 96%, black), var(--vsc-editor-background, var(--vsc-editor-background)));
    }

    .font-layout {
      display: grid;
      grid-template-columns: minmax(0, 1.6fr) minmax(280px, 0.8fr);
      gap: 16px;
      padding: 18px;
      min-height: 100%;
      box-sizing: border-box;
      align-content: start;
    }

    .panel {
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 72%, transparent);
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 88%, white 2%);
      border-radius: 12px;
      box-shadow: 0 18px 48px var(--vsc-shadow-soft);
      overflow: hidden;
    }

    .font-hero {
      grid-column: 1 / -1;
      padding: 20px 22px 18px;
      display: flex;
      flex-direction: column;
      gap: 14px;
    }

    .hero-kicker,
    .sheet-title {
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.16em;
      text-transform: uppercase;
      color: var(--vsc-foreground, var(--vsc-foreground));
      opacity: 0.72;
    }

    .hero-sample {
      line-height: 1.08;
      letter-spacing: -0.03em;
      word-break: break-word;
      color: var(--vsc-foreground, var(--vsc-foreground));
    }

    .hero-caption,
    .sheet-sub {
      font-size: 12px;
      line-height: 1.5;
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 70%, transparent);
    }

    .font-sheet {
      padding: 18px;
      min-width: 0;
    }

    .sheet-header {
      margin-bottom: 14px;
    }

    .glyph-grid {
      display: grid;
      gap: 10px;
    }

    .glyph-row {
      border: 1px solid color-mix(in srgb, var(--vsc-border, var(--vsc-border)) 55%, transparent);
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 78%, white 3%);
      border-radius: 10px;
      padding: 12px 14px;
      overflow: hidden;
    }

    .glyph-line {
      display: block;
      line-height: 1.25;
      white-space: normal;
      overflow-wrap: anywhere;
      color: var(--vsc-foreground, var(--vsc-foreground));
    }

    .font-meta {
      padding: 18px;
      align-self: start;
    }

    .meta-list {
      display: grid;
      gap: 10px;
      margin-top: 14px;
    }

    .meta-row {
      display: grid;
      grid-template-columns: 84px minmax(0, 1fr);
      gap: 12px;
      align-items: start;
      font-size: 12px;
    }

    .meta-label {
      color: color-mix(in srgb, var(--vsc-foreground, var(--vsc-foreground)) 60%, transparent);
      text-transform: uppercase;
      letter-spacing: 0.08em;
      font-size: 10px;
      font-weight: 700;
      padding-top: 3px;
    }

    .meta-value {
      color: var(--vsc-foreground, var(--vsc-foreground));
      word-break: break-word;
      line-height: 1.45;
    }

    .meta-value--mono {
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
    }

    @media (max-width: 900px) {
      .font-layout {
        grid-template-columns: 1fr;
      }

      .font-hero {
        grid-column: auto;
      }
    }
  `, VIEWER_PRODUCTION_CHROME_STYLES],
})
export class FontViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
  @Input() probe: FileProbe | null = null;

  readonly previewSizes = PREVIEW_SIZES;
  readonly glyphRows = GLYPH_ROWS;

  kind: FontViewerKind = 'empty';
  info: FileInfo | null = null;
  error = '';
  previewSize = 72;
  sampleMode: SampleMode = 'sentence';
  fontStack = "var(--vsc-font-family, 'Segoe UI', sans-serif)";
  mimeLabel = '';

  private loadId = 0;
  private destroyed = false;
  private activeFontFace: FontFace | null = null;

  constructor(
    private readonly api: ApiService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get displayName(): string {
    return buildDisplayName(this.info, this.targetPath, this.archiveContext);
  }

  get displayPath(): string {
    if (this.archiveContext) {
      return `${this.archiveContext.fsPath} :: ${this.archiveContext.entryPath}`;
    }
    return this.targetPath;
  }

  get typeBadge(): string {
    const ext = extractExt(this.archiveContext?.entryPath ?? this.targetPath);
    if (ext) {
      return ext.toUpperCase();
    }
    const mime = (this.probe?.mimeType ?? '').split(';', 1)[0].trim().toLowerCase();
    if (mime.startsWith('font/')) {
      return mime.slice(5).toUpperCase();
    }
    return 'FONT';
  }

  get activeSample(): string {
    switch (this.sampleMode) {
      case 'alphabet':
        return 'ABCDEFGHIJKLMNOPQRSTUVWXYZ abcdefghijklmnopqrstuvwxyz';
      case 'symbols':
        return '0123456789 !@#$%^&*() []{} <>?/\\ +-=_';
      default:
        return 'The quick brown fox jumps over the lazy dog';
    }
  }

  get subtitleSample(): string {
    switch (this.sampleMode) {
      case 'alphabet':
        return 'Latin alphabet sample';
      case 'symbols':
        return 'Digits and punctuation sample';
      default:
        return 'Слава Україні • Ґґ Єє Її Іі • 1234567890';
    }
  }

  get glyphSize(): number {
    return Math.max(22, Math.round(this.previewSize * 0.42));
  }

  ngOnChanges(): void {
    void this.reload();
  }

  ngOnDestroy(): void {
    this.destroyed = true;
    this.disposeFontFace();
  }

  setPreviewSize(size: number): void {
    this.previewSize = size;
  }

  setSampleMode(mode: SampleMode): void {
    this.sampleMode = mode;
  }

  formatSize(bytes: number): string {
    return formatSize(bytes);
  }

  private async reload(): Promise<void> {
    const path = (this.targetPath ?? '').trim();
    const currentLoad = ++this.loadId;

    this.disposeFontFace();
    this.error = '';
    this.info = null;
    this.mimeLabel = '';

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
    } catch (error) {
      if (this.destroyed || currentLoad !== this.loadId) {
        return;
      }
      this.kind = 'error';
      this.error = error instanceof Error ? error.message : String(error);
      this.cdr.markForCheck();
    }
  }

  private async loadFromDisk(path: string, loadId: number): Promise<void> {
    const info = await this.api.getFileInfo(path);
    if (this.destroyed || loadId !== this.loadId) {
      return;
    }

    this.info = info;
    if (info.type === FileType.DIRECTORY) {
      this.kind = 'directory';
      this.cdr.markForCheck();
      return;
    }

    const probe = this.probe ?? await this.api.probeFile(path).catch(() => null);
    if (this.destroyed || loadId !== this.loadId) {
      return;
    }

    this.mimeLabel = this.normalizeMime(probe?.mimeType);
    const streamURL = await this.api.getStreamURL(path);
    const bytes = await this.api.fetchBytesFromURL(streamURL);
    await this.installFont(bytes, baseName(path), loadId);
  }

  private async loadFromArchive(ctx: ViewerArchiveContext, loadId: number): Promise<void> {
    const probe = this.probe ?? await this.api.probeArchiveFile(ctx.fsPath, ctx.nestedChain, ctx.entryPath).catch(() => null);
    if (this.destroyed || loadId !== this.loadId) {
      return;
    }

    const streamURL = await this.api.getArchiveStreamURL(ctx.fsPath, ctx.nestedChain, ctx.entryPath);
    const bytes = await this.api.fetchBytesFromURL(streamURL);
    if (this.destroyed || loadId !== this.loadId) {
      return;
    }

    this.info = makeSyntheticInfo(baseName(ctx.entryPath), ctx.entryPath, bytes.byteLength);
    this.mimeLabel = this.normalizeMime(probe?.mimeType);
    await this.installFont(bytes, baseName(ctx.entryPath), loadId);
  }

  private async installFont(bytes: Uint8Array, sourceName: string, loadId: number): Promise<void> {
    if (!bytes.byteLength) {
      throw new Error('The selected font file is empty.');
    }

    const ext = extractExt(sourceName);
    if (ext && !FONT_EXTENSIONS.has(ext) && !this.mimeLabel.startsWith('font/')) {
      throw new Error('This file does not look like a supported font format.');
    }

    const family = `preview-font-${loadId}-${Math.random().toString(36).slice(2, 8)}`;
    const fontData = new Uint8Array(bytes.byteLength);
    fontData.set(bytes);
    const fontFace = new FontFace(family, fontData);
    await fontFace.load();

    if (this.destroyed || loadId !== this.loadId) {
      return;
    }

    this.getDocumentFonts().add(fontFace);
    this.activeFontFace = fontFace;
    this.fontStack = `'${family}', var(--vsc-font-family, 'Segoe UI', sans-serif)`;
    this.kind = 'ready';
    this.cdr.markForCheck();
  }

  private disposeFontFace(): void {
    if (this.activeFontFace) {
      this.getDocumentFonts().delete(this.activeFontFace);
      this.activeFontFace = null;
    }
    this.fontStack = "var(--vsc-font-family, 'Segoe UI', sans-serif)";
  }

  private getDocumentFonts(): { add(face: FontFace): void; delete(face: FontFace): void } {
    return document.fonts as FontFaceSet & { add(face: FontFace): void; delete(face: FontFace): void };
  }

  private normalizeMime(mime: string | null | undefined): string {
    return (mime ?? '').split(';', 1)[0].trim().toLowerCase();
  }
}
