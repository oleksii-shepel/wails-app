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
import type { FileProbe } from '../../shared/protocol';
import type { ViewerFileListItem } from '../../shared/protocol-enhanced';
import { ViewerArchiveContext } from '../viewer-archive-context';
import { VIEWER_BASE_STYLES, VIEWER_PRODUCTION_CHROME_STYLES } from './builtin-viewer.shared';
import { PdfViewerComponent } from './pdf-viewer.component';

type OfficeViewerKind = 'empty' | 'loading' | 'pdf' | 'error' | 'unavailable' | 'unsupported' | 'plugin-required';

// These formats are previewed by the Office Renderer plugin. Keep the legacy
// PDF conversion path out of this component so an explicitly selected legacy
// viewer cannot silently bypass the plugin.
const PLUGIN_RENDERED_EXTENSIONS = new Set([
  'doc', 'docx', 'docm', 'dotx', 'dotm',
  'xls', 'xlsx', 'xlsm', 'xltx', 'xltm', 'xlam',
  'ppt', 'pptx', 'pptm', 'ppsx', 'ppsm', 'potx', 'potm',
]);

@Component({
  selector: 'app-office-viewer',
  standalone: true,
  imports: [CommonModule, PdfViewerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <div class="conversion-status" *ngIf="kind === 'loading'" role="status">
        <span class="conversion-status-activity" aria-hidden="true">
          <span class="conversion-status-activity-dot"></span>
        </span>
        <span class="conversion-status-elapsed">{{ conversionElapsedLabel }}</span>
        <span class="conversion-status-sep">·</span>
        <span class="conversion-status-text">Converting to PDF...</span>
      </div>

      <div class="viewer-body">
        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon" aria-hidden="true">🔍</div>
          <div class="state-label">Select a file to preview</div>
        </div>

        <div *ngIf="kind === 'pdf' && pdfPath" class="pdf-viewer-host">
          <app-pdf-viewer
            [targetPath]="pdfPath"
            [fileList]="fileList"
            [archiveContext]="archiveContext"
            [hideHeader]="true"
            [viewerId]="viewerId">
          </app-pdf-viewer>
        </div>

        <div *ngIf="kind === 'unsupported'" class="state-container state-error" role="status">
          <div class="state-icon" aria-hidden="true">⛔</div>
          <div class="state-label">Unsupported document format</div>
          <div class="error-detail" *ngIf="fileExt">.{{ fileExt }}</div>
          <div class="state-sub">Supported formats: .pdf, .doc, .docx, .xls, .xlsx, .ppt, .pptx, .odt, .ods, .odp, .rtf, .epub, .djvu, .djv</div>
        </div>

        <div *ngIf="kind === 'unavailable'" class="state-container state-error" role="status">
          <div class="state-icon" aria-hidden="true">⚠️</div>
          <div class="state-label">Converter not available</div>
          <div class="error-detail">{{ error }}</div>
          <div class="state-sub">
            Install the required converter to generate a PDF preview:
            <ul>
              <li>Windows: <a href="https://www.libreoffice.org/download/" target="_blank">https://www.libreoffice.org/download/</a></li>
              <li>macOS: <code>brew install libreoffice</code></li>
              <li>Linux: <code>sudo apt install libreoffice</code></li>
            </ul>
          </div>
        </div>

        <div *ngIf="kind === 'plugin-required'" class="state-container state-error" role="status">
          <div class="state-icon" aria-hidden="true">⚠️</div>
          <div class="state-label">Office Renderer plugin required</div>
          <div class="error-detail">{{ error }}</div>
          <div class="state-sub">Enable the Office Document Renderer plugin in Settings, then reopen this document.</div>
        </div>

        <div *ngIf="kind === 'error'" class="state-container state-error" role="alert">
          <div class="state-icon" aria-hidden="true">❌</div>
          <div class="state-label">Failed to convert document</div>
          <div class="error-detail">{{ error }}</div>
          <div class="state-sub">Check that the file is a valid Office document and is not corrupted</div>
        </div>
      </div>
    </div>
  `,
  styles: [VIEWER_BASE_STYLES, `
    :host { display: flex; flex-direction: column; flex: 1 1 0%; min-height: 0; min-width: 0; overflow: hidden; }
    .viewer-surface { display: flex; flex-direction: column; flex: 1 1 0%; min-height: 0; min-width: 0; overflow: hidden; }
    .viewer-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      border-bottom: 1px solid var(--vsc-border, var(--vsc-activitybar-background));
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 92%, var(--vsc-media-canvas-background));
    }
    .header-main { display: flex; align-items: center; gap: 10px; min-width: 0; }
    .file-icon { font-size: 18px; opacity: 0.9; }
    .header-text { min-width: 0; }
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
    .path { font-family: var(--vsc-font-family-mono, ui-monospace, 'Courier New', monospace); font-size: 11px; opacity: 0.6; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; margin-top: 1px; }
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
      background: color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-warning)) 20%, transparent);
      color: var(--vsc-accent-orange, var(--vsc-warning));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-orange, var(--vsc-warning)) 40%, transparent);
    }
    .type-badge {
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 20%, transparent);
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-accent-blue)) 40%, transparent);
    }
    .format-badge {
      background: color-mix(in srgb, var(--vsc-accent-green, var(--vsc-success)) 18%, transparent);
      color: color-mix(in srgb, var(--vsc-accent-green, var(--vsc-success)) 78%, white);
      border: 1px solid color-mix(in srgb, var(--vsc-accent-green, var(--vsc-success)) 40%, transparent);
    }
    .conversion-status {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 12px;
      background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 96%, var(--vsc-media-canvas-background));
      color: var(--vsc-foreground, var(--vsc-foreground));
      flex-shrink: 0;
    }
    .conversion-status-activity {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 16px;
      height: 16px;
      flex-shrink: 0;
      border-radius: 999px;
      background: color-mix(in srgb, var(--vsc-success) 14%, transparent);
      border: 1px solid color-mix(in srgb, var(--vsc-success) 35%, var(--vsc-border, var(--vsc-border)));
    }
    .conversion-status-activity-dot {
      width: 8px;
      height: 8px;
      border-radius: 999px;
      background: var(--vsc-success);
      border: 1px solid var(--vsc-neutral-overlay-strong);
      box-shadow: 0 0 0 0 var(--vsc-success-border);
      animation: office-pulse-green 1.3s ease-out infinite;
    }
    .conversion-status-elapsed {
      font-size: 11px;
      font-weight: 600;
      font-variant-numeric: tabular-nums;
      white-space: nowrap;
      color: var(--vsc-success);
    }
    .conversion-status-sep { opacity: 0.5; }
    .conversion-status-text { font-size: 11px; font-weight: 600; letter-spacing: 0.02em; opacity: 0.9; }
    .viewer-body { flex: 1 1 auto; min-height: 0; position: relative; overflow: hidden; display: flex; flex-direction: column; }
    .pdf-viewer-host { flex: 1 1 0%; min-height: 0; min-width: 0; display: flex; overflow: hidden; }
    .viewer-body app-pdf-viewer { display: flex; flex: 1 1 0%; width: 100%; height: 100%; min-height: 0; min-width: 0; }
    .state-container {
      height: 100%;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      gap: 10px;
      color: var(--vsc-foreground, var(--vsc-foreground));
      padding: 20px;
      text-align: center;
    }
    .state-icon { font-size: 48px; opacity: 0.8; }
    .state-label { font-size: 14px; font-weight: 500; }
    .state-sub { font-size: 12px; opacity: 0.7; margin-top: 5px; line-height: 1.5; }
    .state-sub ul { list-style: inside; margin-top: 10px; font-size: 11px; }
    .state-sub li { margin: 5px 0; }
    .state-sub a { color: var(--vsc-accent-blue, var(--vsc-accent-blue)); text-decoration: none; }
    .state-sub a:hover { text-decoration: underline; }
    .state-sub code { background: color-mix(in srgb, var(--vsc-editor-background, var(--vsc-editor-background)) 80%, var(--vsc-on-accent)); padding: 2px 4px; border-radius: 3px; font-family: monospace; font-size: 10px; }
    .error-detail { color: var(--vsc-errorForeground, var(--vsc-error)); font-size: 12px; margin-top: 5px; }
    .state-error .state-label { color: var(--vsc-errorForeground, var(--vsc-error)); }
    .spinner {
      width: 32px;
      height: 32px;
      border: 3px solid var(--vsc-border, var(--vsc-border));
      border-top: 3px solid var(--vsc-accent-blue, var(--vsc-accent-blue));
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
    }
    @keyframes office-pulse-green {
      0% { box-shadow: 0 0 0 0 var(--vsc-success-border); }
      70% { box-shadow: 0 0 0 8px transparent; }
      100% { box-shadow: 0 0 0 0 transparent; }
    }
    @keyframes spin { to { transform: rotate(360deg); } }
  `, VIEWER_PRODUCTION_CHROME_STYLES],
})
export class OfficeViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() fileList: ViewerFileListItem[] = [];
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
  @Input() probe: FileProbe | null = null;

  kind: OfficeViewerKind = 'empty';
  displayName = '';
  fileExt = '';
  fileTypeLabel = '';
  pdfPath = '';
  error = '';
  conversionElapsedMs = 0;

  private abortController: AbortController | null = null;
  private conversionStartedAt = 0;
  private conversionElapsedTimer: number | null = null;

  constructor(
    private api: ApiService,
    private cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['targetPath'] && !changes['targetPath'].firstChange) {
      this.loadDocument();
    } else if (changes['targetPath'] && changes['targetPath'].firstChange && this.targetPath) {
      this.loadDocument();
    }
  }

  ngOnDestroy(): void {
    this.stopConversionTimer();
    if (this.abortController) {
      this.abortController.abort();
    }
  }

  get conversionElapsedLabel(): string {
    if (this.conversionElapsedMs < 1000) return `${this.conversionElapsedMs} ms`;
    return `${(this.conversionElapsedMs / 1000).toFixed(1)} s`;
  }

  private loadDocument(): void {
    this.stopConversionTimer();
    this.pdfPath = '';
    if (!this.targetPath) {
      this.kind = 'empty';
      this.conversionElapsedMs = 0;
      this.cdr.markForCheck();
      return;
    }

    this.displayName = this.targetPath.split('/').pop() || 'Document';
    this.fileExt = this.displayName.split('.').pop()?.toLowerCase() || '';
    this.fileTypeLabel = this.getFileTypeLabel(this.fileExt);

    if (this.fileExt === 'pdf') {
      this.kind = 'pdf';
      this.error = '';
      this.conversionElapsedMs = 0;
      this.pdfPath = this.targetPath;
      this.cdr.markForCheck();
      return;
    }

    if (PLUGIN_RENDERED_EXTENSIONS.has(this.fileExt)) {
      this.kind = 'plugin-required';
      this.error = `.${this.fileExt} previews use the Office Document Renderer plugin instead of LibreOffice conversion.`;
      this.conversionElapsedMs = 0;
      this.cdr.markForCheck();
      return;
    }

    this.kind = 'loading';
    this.error = '';
    this.conversionElapsedMs = 0;
    this.startConversionTimer();
    this.cdr.markForCheck();

    // Check file extension first for quick validation
    const supportedExts = ['pdf', 'doc', 'docx', 'docm', 'xls', 'xlsx', 'xlsm', 'ppt', 'pptx', 'pptm', 'odt', 'ods', 'odp', 'rtf', 'epub', 'djvu', 'djv'];
    if (!supportedExts.includes(this.fileExt)) {
      this.stopConversionTimer();
      this.kind = 'unsupported';
      this.cdr.markForCheck();
      return;
    }

    this.convertDocument();
  }

  private convertDocument(): void {
    this.abortController = new AbortController();

    this.api.ConvertOfficeDocument(this.targetPath)
      .then((result: any) => {
        this.stopConversionTimer();
        if (result.success) {
          this.pdfPath = result.pdfPath;
          this.kind = 'pdf';
        } else {
          if (result.availability === false) {
            this.kind = 'unavailable';
            this.error = result.instructions || 'Required converter is not available';
          } else {
            this.kind = 'error';
            this.error = result.error || 'Unknown conversion error';
          }
        }
        this.cdr.markForCheck();
      })
      .catch((err: any) => {
        this.stopConversionTimer();
        this.kind = 'error';
        this.error = err?.error?.message || err?.message || 'Failed to convert document';
        this.cdr.markForCheck();
      });
  }

  private startConversionTimer(): void {
    this.stopConversionTimer();
    this.conversionStartedAt = Date.now();
    this.conversionElapsedMs = 0;
    this.conversionElapsedTimer = window.setInterval(() => {
      this.conversionElapsedMs = Date.now() - this.conversionStartedAt;
      this.cdr.markForCheck();
    }, 100);
  }

  private stopConversionTimer(): void {
    if (this.conversionStartedAt) {
      this.conversionElapsedMs = Date.now() - this.conversionStartedAt;
    }
    this.conversionStartedAt = 0;
    if (this.conversionElapsedTimer !== null) {
      window.clearInterval(this.conversionElapsedTimer);
      this.conversionElapsedTimer = null;
    }
  }

  private getFileTypeLabel(ext: string): string {
    const typeMap: Record<string, string> = {
      doc: 'Word', docx: 'Word', docm: 'Word',
      xls: 'Excel', xlsx: 'Excel', xlsm: 'Excel',
      ppt: 'PowerPoint', pptx: 'PowerPoint', pptm: 'PowerPoint',
      odt: 'OpenDoc Text', ods: 'OpenDoc Spreadsheet', odp: 'OpenDoc Presentation',
      rtf: 'Rich Text',
      epub: 'EPUB',
      djvu: 'DjVu', djv: 'DjVu',
    };
    return typeMap[ext] || 'Document';
  }
}
