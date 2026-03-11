import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, Input, OnChanges, Type } from '@angular/core';
import { ViewerRegistryService } from './viewer-registry.service';
import { ViewerArchiveContext } from '../components/viewer-surface.component';

@Component({
  selector: 'app-viewer-host-surface',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container *ngIf="component; else empty">
      <ng-container *ngComponentOutlet="component; inputs: outletInputs"></ng-container>
    </ng-container>

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
    .viewer-host-empty {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      align-items: center;
      justify-content: center;
      color: var(--vsc-foreground, #ccc);
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
  @Input() archiveContext: ViewerArchiveContext | null = null;
  /** Optional explicit viewer id (from tab metadata) */
  @Input() viewerId: string | null = null;

  component: Type<unknown> | null = null;
  outletInputs: Record<string, unknown> = {};

  private resolveSeq = 0;
  private resolvedViewerKey: string | null = null;

  constructor(
    private registry: ViewerRegistryService,
    private cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(): void {
    const req = {
      targetPath: this.targetPath ?? '',
      archiveContext: this.archiveContext ?? null,
      viewerId: this.viewerId ?? null,
    };

    this.outletInputs = {
      targetPath: req.targetPath,
      archiveContext: req.archiveContext,
      viewerId: req.viewerId,
    };

    const viewerKey = this.registry.resolveId(req);
    if (this.component && this.resolvedViewerKey === viewerKey) {
      this.cdr.markForCheck();
      return;
    }

    const seq = ++this.resolveSeq;
    this.resolvedViewerKey = viewerKey;
    this.cdr.markForCheck();

    // Async resolve so we can lazy-load contributions.
    void this.registry.resolve(req).then(({ component }) => {
      if (seq !== this.resolveSeq) return;
      this.component = component;
      this.cdr.markForCheck();
    }).catch(() => {
      if (seq !== this.resolveSeq) return;
      this.resolvedViewerKey = null;
      this.component = null;
      this.cdr.markForCheck();
    });
  }
}

