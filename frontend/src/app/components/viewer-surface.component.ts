import { CommonModule } from '@angular/common';
import { Component, Input, OnChanges } from '@angular/core';
import { ApiService } from '../services/api.service';
import { FileInfo, FileType } from '../shared/protocol';

@Component({
  selector: 'app-viewer-surface',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="viewer-surface">
      <div class="viewer-header">
        <div class="title">{{ (info?.name || 'Preview') }}</div>
        <div class="path" [title]="targetPath">{{ targetPath || 'Select a file to preview' }}</div>
      </div>

      <div class="viewer-body">
        <div *ngIf="!targetPath" class="placeholder">
          No file selected.
        </div>

        <div *ngIf="targetPath && error" class="error">
          {{ error }}
        </div>

        <div *ngIf="targetPath && info && info.type === FileType.DIRECTORY" class="placeholder">
          Directories are not previewed.
        </div>

        <div *ngIf="targetPath && info && info.type !== FileType.DIRECTORY && !error" class="content">
          <img class="image" *ngIf="imageSrc" [src]="imageSrc" [alt]="info.name || 'Image preview'">
          <pre class="text" *ngIf="!imageSrc && textContent !== null; else nonText">{{ textContent }}</pre>
          <ng-template #nonText>
            <div class="placeholder">
              No built-in viewer matched. Add a viewer plugin for this file type.
            </div>
          </ng-template>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    .viewer-surface {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
      background: var(--vsc-editor-background, #1e1e1e);
      color: var(--vsc-foreground, #cccccc);
    }

    .viewer-header {
      display: flex;
      flex-direction: column;
      gap: 2px;
      padding: 10px 12px;
      border-bottom: 1px solid var(--vsc-border, #454545);
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 92%, black);
    }

    .title {
      font-size: 12px;
      font-weight: 700;
      letter-spacing: 0.4px;
      text-transform: uppercase;
      opacity: 0.85;
    }

    .path {
      font-family: var(--vsc-font-family-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
      font-size: 12px;
      opacity: 0.9;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .viewer-body {
      flex: 1 1 0%;
      min-height: 0;
      overflow: auto;
      padding: 10px 12px;
    }

    .placeholder {
      opacity: 0.75;
      font-size: 13px;
    }

    .error {
      color: #ff6b6b;
      white-space: pre-wrap;
      font-size: 13px;
    }

    .text {
      margin: 0;
      white-space: pre;
      font-size: 12px;
      line-height: 1.45;
      font-family: var(--vsc-font-family-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace);
    }

    .image {
      display: block;
      max-width: 100%;
      max-height: 100%;
      object-fit: contain;
      background: color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 80%, black);
      border: 1px solid color-mix(in srgb, var(--vsc-border, #454545) 60%, transparent);
      border-radius: 6px;
    }
  `]
})
export class ViewerSurfaceComponent implements OnChanges {
  @Input() targetPath = '';

  info: FileInfo | null = null;
  textContent: string | null = null;
  imageSrc: string | null = null;
  error: string | null = null;

  FileType = FileType;

  // Keep this conservative to avoid hanging the UI on giant files.
  private readonly MAX_TEXT_BYTES = 512 * 1024;
  private readonly MAX_IMAGE_BYTES = 20 * 1024 * 1024;

  constructor(private api: ApiService) {}

  async ngOnChanges(): Promise<void> {
    const path = (this.targetPath ?? '').trim();
    this.error = null;
    this.info = null;
    this.textContent = null;
    this.imageSrc = null;

    if (!path) return;

    try {
      const info = await this.api.getFileInfo(path);
      this.info = info;

      if (info.type === FileType.DIRECTORY) return;
      const mime = this.inferImageMime(path);
      const isImage = !!mime;

      if (isImage) {
        if (info.size > this.MAX_IMAGE_BYTES) return;
        const content = await this.api.readFile(path, 'base64');
        const raw = content?.content ?? '';
        if (!raw) return;
        this.imageSrc = `data:${mime};base64,${raw}`;
        return;
      }

      if (info.size > this.MAX_TEXT_BYTES) return;

      // First built-in viewer: plain text (utf8). Future viewers will be resolved via a registry.
      const content = await this.api.readFile(path, 'utf8');
      this.textContent = content?.content ?? '';
    } catch (err: any) {
      this.error = err?.message || String(err);
    }
  }

  private inferImageMime(path: string): string | null {
    const idx = path.lastIndexOf('.');
    const ext = (idx >= 0 ? path.slice(idx + 1) : '').toLowerCase();
    switch (ext) {
      case 'png': return 'image/png';
      case 'jpg':
      case 'jpeg': return 'image/jpeg';
      case 'gif': return 'image/gif';
      case 'webp': return 'image/webp';
      case 'bmp': return 'image/bmp';
      case 'ico': return 'image/x-icon';
      case 'svg': return 'image/svg+xml';
      case 'tif':
      case 'tiff': return 'image/tiff';
      default: return null;
    }
  }
}
