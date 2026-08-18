import { CommonModule, NgComponentOutlet } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, EventEmitter, Input, OnChanges, OnDestroy, Output, Type, ViewChild } from '@angular/core';
import { ApiService } from '../services/api.service';
import { ViewerRegistryService } from './viewer-registry.service';
import { ViewerArchiveContext } from './viewer-archive-context';
import type { FileInfo, FileProbe } from '../shared/protocol';
import type { ViewerFileListItem } from '../shared/protocol-enhanced';

function extLower(path: string): string {
  const p = (path ?? '').replace(/\\/g, '/');
  const i = p.lastIndexOf('.');
  if (i < 0) return '';
  return p.slice(i + 1).toLowerCase();
}

@Component({
  selector: 'app-viewer-host-surface',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-host">
      <header class="viewer-chrome" *ngIf="targetPath; else noTarget">
        <div class="viewer-identity">
          <span class="viewer-file-icon" aria-hidden="true">{{ fileIcon }}</span>
          <span class="viewer-context" *ngIf="autoFollow">Preview</span>
          <span class="viewer-title" [title]="displayName">{{ displayName }}</span>
          <span class="viewer-kind">{{ displayKind }}</span>
          <span class="viewer-state" *ngIf="resolving">Loading…</span>
          <span class="viewer-metadata" [title]="displayPath">
            <span *ngIf="probe?.mimeType">{{ probe.mimeType }}</span>
            <span *ngIf="fileInfo?.size">{{ formatSize(fileInfo.size || 0) }}</span>
            <span *ngIf="archiveContext">Archive</span>
            <span class="viewer-path">{{ displayPath }}</span>
          </span>
        </div>

        <div class="viewer-actions">
          <button
            type="button"
            *ngIf="sourceAvailable"
            class="viewer-action viewer-action--text"
            (click)="returnSource.emit()"
            title="Return to source folder">
            Folder
          </button>

          <span class="viewer-divider" *ngIf="sourceAvailable"></span>

          <button type="button" (click)="toggleFullscreen()" title="Fullscreen">⛶</button>
          <button
            type="button"
            [disabled]="!!archiveContext"
            (click)="external.emit()"
            title="Open with external application">
            ↗
          </button>
          <button
            type="button"
            [disabled]="isPinned || !isClosable"
            (click)="close.emit()"
            title="Close viewer">
            ×
          </button>
        </div>
      </header>

      <div class="viewer-stage" [class.resolving]="resolving" #viewerStage>
        <ng-container *ngIf="component; else empty">
          <div class="viewer-content" [attr.data-viewer-key]="resolvedViewerKey || 'viewer'">
            <ng-container *ngComponentOutlet="component; inputs: outletInputs"></ng-container>
          </div>
        </ng-container>

        <div class="viewer-transition-status" *ngIf="resolving && component" aria-live="polite">
          Loading preview…
        </div>
      </div>
    </div>

    <ng-template #noTarget>
      <div class="viewer-host-empty">
        <div class="title">Preview</div>
        <div class="sub">Select a file to preview</div>
      </div>
    </ng-template>

    <ng-template #empty>
      <div class="viewer-host-empty viewer-host-empty--stage">
        <div class="title">{{ resolving ? 'Loading preview…' : 'Preview unavailable' }}</div>
        <div class="sub" *ngIf="!resolving">{{ displayPath }}</div>
      </div>
    </ng-template>
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

    .viewer-host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
      background: var(--vsc-editor-background);
      color: var(--vsc-foreground);
    }

    .viewer-chrome {
      display: flex;
      align-items: center;
      gap: 10px;
      min-height: 32px;
      padding: 3px 6px 3px 8px;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-tabbar-background, var(--vsc-panel-background));
      flex: 0 0 auto;
      min-width: 0;
    }

    .viewer-identity {
      display: flex;
      align-items: center;
      gap: 6px;
      min-width: 0;
      flex: 1 1 auto;
      overflow: hidden;
    }

    .viewer-file-icon {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      width: 20px;
      height: 20px;
      flex: 0 0 20px;
      font-size: 14px;
      line-height: 1;
    }

    .viewer-context {
      flex: 0 0 auto;
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      font-size: 10px;
      font-weight: 600;
    }

    .viewer-title {
      min-width: 48px;
      max-width: min(260px, 28vw);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 12px;
      font-weight: 600;
      flex: 0 1 auto;
    }

    .viewer-kind,
    .viewer-state {
      flex: 0 0 auto;
      padding: 0 5px;
      border-radius: 3px;
      font-size: 9px;
      line-height: 16px;
      text-transform: uppercase;
      letter-spacing: .25px;
    }

    .viewer-kind {
      color: var(--vsc-accent-blue, var(--vsc-focus-ring));
      border: 1px solid color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-ring)) 35%, transparent);
      background: color-mix(in srgb, var(--vsc-accent-blue, var(--vsc-focus-ring)) 10%, transparent);
    }

    .viewer-state {
      color: var(--vsc-description-foreground);
      background: color-mix(in srgb, var(--vsc-foreground) 7%, transparent);
    }

    .viewer-metadata {
      display: flex;
      align-items: center;
      gap: 0;
      min-width: 0;
      flex: 1 1 auto;
      overflow: hidden;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 10px;
      line-height: 18px;
      white-space: nowrap;
    }

    .viewer-metadata > span:not(:first-child)::before {
      content: '·';
      margin: 0 6px;
      opacity: .55;
    }

    .viewer-path {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      opacity: .72;
    }

    .viewer-actions {
      display: flex;
      align-items: center;
      gap: 2px;
      flex: 0 0 auto;
    }

    .viewer-actions button {
      min-width: 24px;
      height: 24px;
      padding: 0 6px;
      border: 1px solid transparent;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      font: inherit;
      font-size: 12px;
      cursor: pointer;
    }

    .viewer-actions button:hover:not(:disabled) {
      background: var(--vsc-list-hover-background);
      border-color: var(--vsc-border-subtle, var(--vsc-border));
    }

    .viewer-actions button:disabled {
      opacity: .32;
      cursor: default;
    }

    .viewer-action--text {
      min-width: auto !important;
    }

    .viewer-divider {
      width: 1px;
      height: 18px;
      margin: 0 3px;
      background: var(--vsc-border, var(--vsc-border-subtle));
    }

    .viewer-stage {
      position: relative;
      display: flex;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }

    .viewer-content {
      display: flex;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      animation: viewer-enter 90ms ease-out;
    }

    /*
     * ViewerHostSurface is the only file-identity chrome. Built-in and plugin
     * viewers may keep content-specific toolbars, but not a second file header.
     */
    :host ::ng-deep .viewer-content .viewer-header {
      display: none !important;
    }

    :host ::ng-deep .viewer-content .viewer-toolbar {
      min-height: 30px;
      padding: 2px 8px;
      background: var(--vsc-editor-background);
      border-bottom-color: var(--vsc-border-subtle, var(--vsc-border));
    }

    .viewer-transition-status {
      position: absolute;
      top: 6px;
      right: 8px;
      z-index: 10;
      padding: 2px 6px;
      border-radius: 3px;
      background: color-mix(in srgb, var(--vsc-menu-background, var(--vsc-panel-background)) 88%, transparent);
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 10px;
      pointer-events: none;
    }

    .viewer-stage.resolving .viewer-content {
      opacity: .72;
      transition: opacity 80ms ease;
    }

    @keyframes viewer-enter {
      from { opacity: .72; transform: translateY(1px); }
      to { opacity: 1; transform: translateY(0); }
    }

    @media (prefers-reduced-motion: reduce) {
      .viewer-content { animation: none; }
      .viewer-stage.resolving .viewer-content { transition: none; }
    }

    .viewer-host-empty {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      align-items: center;
      justify-content: center;
      color: var(--vsc-foreground);
      opacity: .8;
      gap: 6px;
      user-select: none;
    }

    .viewer-host-empty--stage {
      width: 100%;
    }

    .viewer-host-empty .title { font-weight: 600; }
    .viewer-host-empty .sub {
      max-width: min(680px, 80vw);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      font-size: 11px;
      opacity: .75;
    }

    @media (max-width: 900px) {
      .viewer-path { display: none; }
    }

    @media (max-width: 640px) {
      .viewer-metadata { display: none; }
      .viewer-actions .viewer-action--text { display: none; }
      .viewer-title { max-width: 40vw; }
    }
  `],
})
export class ViewerHostSurfaceComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  resolving = false;
  @Input() fileList: ViewerFileListItem[] = [];
  @Input() archiveContext: ViewerArchiveContext | null = null;
  /** Optional explicit viewer id (from tab metadata) */
  @Input() viewerId: string | null = null;
  @Input() autoFollow = false;
  @Input() sourceAvailable = false;
  @Input() isPinned = false;
  @Input() isClosable = true;

  @Output() returnSource = new EventEmitter<void>();
  @Output() external = new EventEmitter<void>();
  @Output() close = new EventEmitter<void>();

  component: Type<unknown> | null = null;
  outletInputs: Record<string, unknown> = {};
  probe: FileProbe | null = null;
  fileInfo: FileInfo | null = null;
  error: string | null = null;

  @ViewChild(NgComponentOutlet) private outlet?: NgComponentOutlet;
  @ViewChild('viewerStage') private viewerStage?: import('@angular/core').ElementRef<HTMLElement>;

  private resolveSeq = 0;
  resolvedViewerKey: string | null = null;
  private renderedInputSignature = '';

  constructor(
    private registry: ViewerRegistryService,
    private api: ApiService,
    private cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(): void {
    void this.refresh();
  }

  private async refresh(): Promise<void> {
    const req = {
      targetPath: this.targetPath ?? '',
      fileList: this.fileList ?? [],
      archiveContext: this.archiveContext ?? null,
      viewerId: this.viewerId ?? null,
      probe: null as FileProbe | null,
    };
    const seq = ++this.resolveSeq;
    this.resolving = !!req.targetPath;
    this.error = null;
    this.probe = null;
    this.fileInfo = null;
    this.cdr.markForCheck();

    if (!req.targetPath) {
      this.resolvedViewerKey = null;
      this.renderedInputSignature = '';
      this.component = null;
      this.outletInputs = {};
      this.resolving = false;
      this.cdr.markForCheck();
      return;
    }

    const ext = extLower(req.targetPath);
    if (ext || req.targetPath) {
      const probePromise = req.archiveContext
        ? this.api.probeArchiveFile(
            req.archiveContext.fsPath,
            req.archiveContext.nestedChain,
            req.archiveContext.entryPath,
          )
        : this.api.probeFile(req.targetPath);

      const infoPromise = req.archiveContext
        ? Promise.resolve(null)
        : this.api.getFileInfo(req.targetPath).catch(() => null);

      try {
        const [probe, info] = await Promise.all([
          probePromise.catch(() => null),
          infoPromise,
        ]);
        if (seq !== this.resolveSeq) return;
        req.probe = probe;
        this.probe = probe;
        this.fileInfo = info;
      } catch {
        if (seq !== this.resolveSeq) return;
        req.probe = null;
        this.probe = null;
        this.fileInfo = null;
      }
    }

    if (seq !== this.resolveSeq) return;

    const inputSignature = JSON.stringify(req);

    this.outletInputs = {
      targetPath: req.targetPath,
      fileList: req.fileList,
      archiveContext: req.archiveContext,
      viewerId: req.viewerId,
      probe: req.probe,
    };

    const viewerKey = this.registry.resolveId(req);
    const preserveMountedViewer = this.shouldPreserveMountedViewer(viewerKey);
    if (this.component && this.resolvedViewerKey === viewerKey) {
      if (this.renderedInputSignature !== inputSignature) {
        this.renderedInputSignature = inputSignature;
        if (!preserveMountedViewer) {
          this.component = null;
          this.cdr.detectChanges();
        }
        this.cdr.markForCheck();
        if (preserveMountedViewer) {
          this.resolving = false;
          this.cdr.markForCheck();
          return;
        }
      }

      if (preserveMountedViewer) {
        this.resolving = false;
        this.cdr.markForCheck();
        return;
      }
    }

    this.resolvedViewerKey = viewerKey;
    this.renderedInputSignature = inputSignature;
    this.cdr.markForCheck();

    // Async resolve so we can lazy-load contributions.
    void this.registry.resolve(req).then(({ component }) => {
      if (seq !== this.resolveSeq) return;
      this.component = component;
      this.resolving = false;
      this.cdr.markForCheck();
    }).catch((error: unknown) => {
      if (seq !== this.resolveSeq) return;
      this.error = error instanceof Error ? error.message : String(error ?? 'Preview unavailable');
      this.resolvedViewerKey = null;
      this.renderedInputSignature = '';
      this.component = null;
      this.resolving = false;
      this.cdr.markForCheck();
    });
  }

  ngOnDestroy(): void {
    // Invalidates all pending probe/registry work when the pane or tab disappears.
    this.resolveSeq += 1;
    this.component = null;
    this.outletInputs = {};
  }

  get displayName(): string {
    const source = this.archiveContext?.entryPath || this.targetPath || '';
    const normalized = source.replace(/\\/g, '/').replace(/\/+$/, '');
    return normalized.split('/').pop() || source || 'Preview';
  }

  get displayPath(): string {
    return this.archiveContext?.entryPath || this.targetPath || '';
  }

  get displayKind(): string {
    const key = (this.resolvedViewerKey || this.viewerId || 'auto').trim();
    return key || 'auto';
  }

  get fileIcon(): string {
    switch ((this.resolvedViewerKey || this.viewerId || '').toLowerCase()) {
      case 'image': return '▧';
      case 'video': return '▶';
      case 'audio': return '♪';
      case 'pdf': return 'PDF';
      case 'archive': return '▦';
      case 'text':
      case 'markdown':
      case 'code': return '</>';
      case 'binary': return '01';
      default: return '◉';
    }
  }

  formatSize(bytes: number): string {
    if (!Number.isFinite(bytes) || bytes <= 0) return '';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let value = bytes;
    let unit = 0;
    while (value >= 1024 && unit < units.length - 1) {
      value /= 1024;
      unit += 1;
    }
    const digits = unit === 0 || value >= 100 ? 0 : value >= 10 ? 1 : 2;
    return `${value.toFixed(digits)} ${units[unit]}`;
  }

  supports(method: string): boolean {
    const viewer = this.mountedViewer;
    return !!viewer && typeof viewer[method] === 'function';
  }

  invoke(method: string): void {
    const viewer = this.mountedViewer;
    if (!viewer || typeof viewer[method] !== 'function') return;
    viewer[method]();
  }

  toggleFullscreen(): void {
    const stage = this.viewerStage?.nativeElement;
    if (!stage) return;

    if (document.fullscreenElement === stage) {
      void document.exitFullscreen?.().catch(() => {});
      return;
    }
    void stage.requestFullscreen?.().catch(() => {});
  }

  get mountedViewer(): any {
    return (this.outlet as any)?.componentInstance ?? null;
  }

  private shouldPreserveMountedViewer(viewerKey: string): boolean {
    return viewerKey === 'image' || viewerKey === 'video' || viewerKey === 'audio';
  }
}
