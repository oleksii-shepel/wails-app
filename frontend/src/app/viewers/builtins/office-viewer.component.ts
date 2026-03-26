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
import { ViewerArchiveContext } from '../viewer-archive-context';
import { PdfViewerComponent } from './pdf-viewer.component';

type OfficeViewerKind = 'empty' | 'loading' | 'pdf' | 'error' | 'unavailable' | 'unsupported';

@Component({
  selector: 'app-office-viewer',
  standalone: true,
  imports: [CommonModule, PdfViewerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-surface">
      <div class="viewer-header">
        <div class="header-main">
          <span class="file-icon">📄</span>
          <div class="header-text">
            <div class="title" [title]="displayName">{{ displayName || 'Preview' }}</div>
            <div class="path" [title]="targetPath">{{ targetPath || 'Select a file to preview' }}</div>
          </div>
        </div>
        <div class="header-meta" *ngIf="kind !== 'empty'">
          <span class="badge archive-badge" *ngIf="archiveContext" title="Extracted from archive into memory">
            📦 archive
          </span>
          <span class="badge type-badge">{{ fileTypeLabel }}</span>
          <span class="badge format-badge" *ngIf="kind === 'pdf'">PDF preview</span>
        </div>
      </div>

      <div class="conversion-status" *ngIf="kind === 'loading'">
        <span class="conversion-status-activity" aria-hidden="true">
          <span class="conversion-status-activity-dot"></span>
        </span>
        <span class="conversion-status-text">Converting to PDF...</span>
      </div>

      <div class="viewer-body">
        <div *ngIf="kind === 'empty'" class="state-container">
          <div class="state-icon">🔍</div>
          <div class="state-label">Select a file to preview</div>
        </div>

        <div *ngIf="kind === 'loading'" class="state-container">
          <div class="spinner"></div>
          <div class="state-label">Converting document to PDF...</div>
          <div class="state-sub">This may take a moment</div>
        </div>

        <div *ngIf="kind === 'pdf' && pdfPath" class="pdf-viewer-host">
          <app-pdf-viewer
            [targetPath]="pdfPath"
            [archiveContext]="archiveContext"
            [hideHeader]="true"
            [viewerId]="viewerId">
          </app-pdf-viewer>
        </div>

        <div *ngIf="kind === 'unsupported'" class="state-container state-error">
          <div class="state-icon">⛔</div>
          <div class="state-label">Unsupported document format</div>
          <div class="error-detail" *ngIf="fileExt">.{{ fileExt }}</div>
          <div class="state-sub">Supported formats: .doc, .docx, .xls, .xlsx, .ppt, .pptx, .odt, .ods, .odp, .rtf</div>
        </div>

        <div *ngIf="kind === 'unavailable'" class="state-container state-error">
          <div class="state-icon">⚠️</div>
          <div class="state-label">LibreOffice not installed</div>
          <div class="error-detail">{{ error }}</div>
          <div class="state-sub">
            Install LibreOffice to convert Office documents to PDF:
            <ul>
              <li>Windows: <a href="https://www.libreoffice.org/download/" target="_blank">https://www.libreoffice.org/download/</a></li>
              <li>macOS: <code>brew install libreoffice</code></li>
              <li>Linux: <code>sudo apt install libreoffice</code></li>
            </ul>
          </div>
        </div>

        <div *ngIf="kind === 'error'" class="state-container state-error">
          <div class="state-icon">❌</div>
          <div class="state-label">Failed to convert document</div>
          <div class="error-detail">{{ error }}</div>
          <div class="state-sub">Check that the file is a valid Office document and is not corrupted</div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host { display: flex; flex-direction: column; flex: 1 1 0%; min-height: 0; min-width: 0; overflow: hidden; }
    .viewer-surface { display: flex; flex-direction: column; flex: 1 1 0%; min-height: 0; min-width: 0; overflow: hidden; }
    .viewer-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 10px;
      padding: 10px 12px;
      border-bottom: 1px solid var(--vsc-border, #333);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 92%, #000);
    }
    .header-main { display: flex; align-items: center; gap: 10px; min-width: 0; }
    .file-icon { font-size: 18px; opacity: 0.9; }
    .header-text { min-width: 0; }
    .title { font-weight: 600; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .path { font-size: 11px; opacity: 0.75; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
    .header-meta { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; justify-content: flex-end; }
    .badge { font-size: 10px; padding: 2px 6px; border: 1px solid var(--vsc-border, #444); border-radius: 999px; opacity: 0.9; }
    .archive-badge { border-color: color-mix(in srgb, var(--vsc-accent-orange, #f90) 60%, var(--vsc-border, #444)); }
    .type-badge { border-color: var(--vsc-accent-blue, #0078d4); }
    .format-badge { color: var(--vsc-accent-green, #107c10); border-color: var(--vsc-accent-green, #107c10); }
    .conversion-status {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 12px;
      border-bottom: 1px solid var(--vsc-border, #333);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 96%, #000);
      color: var(--vsc-foreground, #ccc);
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
      background: color-mix(in srgb, #2bb24c 14%, transparent);
      border: 1px solid color-mix(in srgb, #2bb24c 35%, var(--vsc-border, #444));
    }
    .conversion-status-activity-dot {
      width: 8px;
      height: 8px;
      border-radius: 999px;
      background: #2bb24c;
      border: 1px solid rgba(255, 255, 255, 0.35);
      box-shadow: 0 0 0 0 rgba(43, 178, 76, 0.45);
      animation: office-pulse-green 1.3s ease-out infinite;
    }
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
      color: var(--vsc-foreground, #ccc);
      padding: 20px;
      text-align: center;
    }
    .state-icon { font-size: 48px; opacity: 0.8; }
    .state-label { font-size: 14px; font-weight: 500; }
    .state-sub { font-size: 12px; opacity: 0.7; margin-top: 5px; line-height: 1.5; }
    .state-sub ul { list-style: inside; margin-top: 10px; font-size: 11px; }
    .state-sub li { margin: 5px 0; }
    .state-sub a { color: var(--vsc-accent-blue, #0078d4); text-decoration: none; }
    .state-sub a:hover { text-decoration: underline; }
    .state-sub code { background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 80%, #fff); padding: 2px 4px; border-radius: 3px; font-family: monospace; font-size: 10px; }
    .error-detail { color: var(--vsc-errorForeground, #f48771); font-size: 12px; margin-top: 5px; }
    .state-error .state-label { color: var(--vsc-errorForeground, #f48771); }
    .spinner {
      width: 32px;
      height: 32px;
      border: 3px solid var(--vsc-border, #444);
      border-top: 3px solid var(--vsc-accent-blue, #0078d4);
      border-radius: 50%;
      animation: spin 0.8s linear infinite;
    }
    @keyframes office-pulse-green {
      0% { box-shadow: 0 0 0 0 rgba(43, 178, 76, 0.45); }
      70% { box-shadow: 0 0 0 8px rgba(43, 178, 76, 0); }
      100% { box-shadow: 0 0 0 0 rgba(43, 178, 76, 0); }
    }
    @keyframes spin { to { transform: rotate(360deg); } }
  `],
})
export class OfficeViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;

  kind: OfficeViewerKind = 'empty';
  displayName = '';
  fileExt = '';
  fileTypeLabel = '';
  pdfPath = '';
  error = '';

  private abortController: AbortController | null = null;

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
    if (this.abortController) {
      this.abortController.abort();
    }
  }

  private loadDocument(): void {
    if (!this.targetPath) {
      this.kind = 'empty';
      this.cdr.markForCheck();
      return;
    }

    this.displayName = this.targetPath.split('/').pop() || 'Document';
    this.fileExt = this.displayName.split('.').pop()?.toLowerCase() || '';
    this.fileTypeLabel = this.getFileTypeLabel(this.fileExt);

    this.kind = 'loading';
    this.error = '';
    this.cdr.markForCheck();

    // Check file extension first for quick validation
    const supportedExts = ['doc', 'docx', 'docm', 'xls', 'xlsx', 'xlsm', 'ppt', 'pptx', 'pptm', 'odt', 'ods', 'odp', 'rtf'];
    if (!supportedExts.includes(this.fileExt)) {
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
        if (result.success) {
          this.pdfPath = result.pdfPath;
          this.kind = 'pdf';
        } else {
          if (result.availability === false) {
            this.kind = 'unavailable';
            this.error = result.instructions || 'LibreOffice is not available';
          } else {
            this.kind = 'error';
            this.error = result.error || 'Unknown conversion error';
          }
        }
        this.cdr.markForCheck();
      })
      .catch((err: any) => {
        this.kind = 'error';
        this.error = err?.error?.message || err?.message || 'Failed to convert document';
        this.cdr.markForCheck();
      });
  }

  private getFileTypeLabel(ext: string): string {
    const typeMap: Record<string, string> = {
      doc: 'Word', docx: 'Word', docm: 'Word',
      xls: 'Excel', xlsx: 'Excel', xlsm: 'Excel',
      ppt: 'PowerPoint', pptx: 'PowerPoint', pptm: 'PowerPoint',
      odt: 'OpenDoc Text', ods: 'OpenDoc Spreadsheet', odp: 'OpenDoc Presentation',
      rtf: 'Rich Text',
    };
    return typeMap[ext] || 'Office Document';
  }
}
