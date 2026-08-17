import { CommonModule, NgComponentOutlet } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, Input, OnChanges, Type, ViewChild } from '@angular/core';
import { ApiService } from '../services/api.service';
import { ViewerRegistryService } from './viewer-registry.service';
import { ViewerArchiveContext } from './viewer-archive-context';
import type { FileProbe } from '../shared/protocol';
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
    <div class="viewer-stage" [class.resolving]="resolving">
      <ng-container *ngIf="component; else empty">
        <div class="viewer-content" [attr.data-viewer-key]="resolvedViewerKey || 'viewer'">
          <ng-container *ngComponentOutlet="component; inputs: outletInputs"></ng-container>
        </div>
      </ng-container>

      <div class="viewer-transition-status" *ngIf="resolving && component" aria-live="polite">
        Loading preview…
      </div>
    </div>

    <ng-template #empty>
      <div class="viewer-host-empty">
        <div class="title">Preview</div>
        <div class="sub">Select a file to preview</div>
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
    .viewer-transition-status {
      position: absolute;
      top: 6px;
      right: 8px;
      z-index: 10;
      padding: 2px 6px;
      border-radius: 3px;
      background: color-mix(
        in srgb,
        var(--vsc-menu-background, var(--vsc-panel-background)) 88%,
        transparent
      );
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
      color: var(--vsc-foreground, var(--vsc-foreground));
      opacity: 0.8;
      gap: 6px;
      user-select: none;
    }
    .viewer-host-empty .title { font-weight: 600; }
    .viewer-host-empty .sub { font-size: 12px; opacity: 0.8; }
  `],
})
export class ViewerHostSurfaceComponent implements OnChanges {
  @Input() targetPath = '';
  resolving = false;
  @Input() fileList: ViewerFileListItem[] = [];
  @Input() archiveContext: ViewerArchiveContext | null = null;
  /** Optional explicit viewer id (from tab metadata) */
  @Input() viewerId: string | null = null;

  component: Type<unknown> | null = null;
  outletInputs: Record<string, unknown> = {};

  @ViewChild(NgComponentOutlet) private outlet?: NgComponentOutlet;

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
      try {
        req.probe = req.archiveContext
          ? await this.api.probeArchiveFile(req.archiveContext.fsPath, req.archiveContext.nestedChain, req.archiveContext.entryPath)
          : await this.api.probeFile(req.targetPath);
      } catch {
        req.probe = null;
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
    }).catch(() => {
      if (seq !== this.resolveSeq) return;
      this.resolvedViewerKey = null;
      this.renderedInputSignature = '';
      this.component = null;
      this.resolving = false;
      this.cdr.markForCheck();
    });
  }

  get mountedViewer(): any {
    return (this.outlet as any)?.componentInstance ?? null;
  }

  private shouldPreserveMountedViewer(viewerKey: string): boolean {
    return viewerKey === 'image' || viewerKey === 'video' || viewerKey === 'audio';
  }
}