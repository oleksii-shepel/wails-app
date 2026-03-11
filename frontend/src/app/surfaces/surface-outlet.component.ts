import { CommonModule, NgComponentOutlet } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, Input, OnChanges, Type, ViewChild } from '@angular/core';
import { SurfaceRegistryService } from './surface-registry.service';
import { SurfaceContext } from './surface-context';

@Component({
  selector: 'app-surface-outlet',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container *ngIf="component; else empty">
      <ng-container *ngComponentOutlet="component; inputs: outletInputs"></ng-container>
    </ng-container>

    <ng-template #empty>
      <div class="surface-empty"></div>
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

    .surface-empty {
      flex: 1 1 0%;
      min-height: 0;
    }
  `],
})
export class SurfaceOutletComponent implements OnChanges {
  @Input({ required: true }) ctx!: SurfaceContext;

  component: Type<unknown> | null = null;
  outletInputs: Record<string, unknown> = {};

  @ViewChild(NgComponentOutlet) private outlet?: NgComponentOutlet;

  private resolveSeq = 0;
  private resolvedSurfaceKey: string | null = null;

  constructor(
    private registry: SurfaceRegistryService,
    private cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(): void {
    this.outletInputs = { ctx: this.ctx };
    const surfaceKey = this.ctx?.tab?.kind ?? 'dir';

    // FilePaneComponent rebuilds the surface context object on every change
    // detection pass. Reload the dynamic surface only when the surface kind
    // actually changes; otherwise just push fresh inputs into the mounted one.
    if (this.component && this.resolvedSurfaceKey === surfaceKey) {
      this.cdr.markForCheck();
      return;
    }

    const seq = ++this.resolveSeq;
    this.resolvedSurfaceKey = surfaceKey;
    this.cdr.markForCheck();

    void this.registry.resolve({ tab: this.ctx.tab }).then(({ component }) => {
      if (seq !== this.resolveSeq) return;
      this.component = component;
      this.cdr.markForCheck();
    }).catch(() => {
      if (seq !== this.resolveSeq) return;
      this.resolvedSurfaceKey = null;
      this.component = null;
      this.cdr.markForCheck();
    });
  }

  // Proxy access used by FilePaneComponent for keyboard commands.
  private get mountedInstance(): any {
    return (this.outlet as any)?.componentInstance ?? null;
  }
  get addressBar() { return this.mountedInstance?.addressBar; }
  get fileViewContainer() { return this.mountedInstance?.fileViewContainer; }
  get findWidget() { return this.mountedInstance?.findWidget; }
}
