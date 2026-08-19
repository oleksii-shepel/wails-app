import { CommonModule, NgComponentOutlet } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, EventEmitter, Input, OnChanges, Output, Type, ViewChild } from '@angular/core';
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
  @Input() active = false;
  /** Re-emits `firstEntryArrived` from dynamic surfaces so parents may react. */
  @Output() firstEntryArrived = new EventEmitter<void>();

  component: Type<unknown> | null = null;
  outletInputs: Record<string, unknown> = {};

  @ViewChild(NgComponentOutlet) private outlet?: NgComponentOutlet;

  private resolveSeq = 0;
  private resolvedSurfaceKey: string | null = null;
  private renderedInputSignature = '';
  private mountedSubscription: { unsubscribe: () => void } | null = null;

  constructor(
    private registry: SurfaceRegistryService,
    private cdr: ChangeDetectorRef,
  ) {}

  ngOnChanges(): void {
    const surfaceKey = this.ctx ? `${this.ctx.surfaceId}:${this.ctx.tab.id}` : 'dir';
    const inputSignature = JSON.stringify(this.ctx ?? null);

    // FilePaneComponent rebuilds the surface context object on every change
    // detection pass. Reload the dynamic surface only when the surface identity
    // or active tab actually changes; otherwise just push fresh inputs into the
    // mounted one so long-running viewers keep their runtime state.
    if (this.component && this.resolvedSurfaceKey === surfaceKey) {
      if (this.renderedInputSignature !== inputSignature) {
        this.outletInputs = { ctx: this.ctx, active: this.active };
        this.renderedInputSignature = inputSignature;
        this.cdr.markForCheck();

        return;
      }

      this.outletInputs = { ctx: this.ctx, active: this.active };
      this.cdr.markForCheck();
      return;
    }

    const seq = ++this.resolveSeq;
    this.resolvedSurfaceKey = surfaceKey;
    const targetCtx = this.ctx;
    this.renderedInputSignature = inputSignature;
    this.cdr.markForCheck();

    void this.registry.resolve({ tab: targetCtx.tab, context: targetCtx }).then(({ component }) => {
      if (seq !== this.resolveSeq) return;
      this.outletInputs = { ctx: targetCtx, active: this.active };
      this.component = component;
      this.cdr.markForCheck();
      // Give Angular a tick to create the component instance, then wire outputs
      setTimeout(() => {
        const inst = this.mountedInstance;
        if (!inst) return;
        // Clean up previous subscription
        try { this.mountedSubscription?.unsubscribe(); } catch {}
        if (inst.firstEntryArrived && typeof inst.firstEntryArrived.subscribe === 'function') {
          const sub = inst.firstEntryArrived.subscribe(() => this.firstEntryArrived.emit());
          this.mountedSubscription = { unsubscribe: () => sub.unsubscribe() };
        }
      }, 0);
    }).catch(() => {
      if (seq !== this.resolveSeq) return;
      this.resolvedSurfaceKey = null;
      this.renderedInputSignature = '';
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
  get viewerHost() { return this.mountedInstance?.viewerHost; }
  get viewer() { return this.viewerHost?.mountedViewer; }

  supports(method: string): boolean {
    return typeof this.mountedInstance?.[method] === 'function';
  }

  invoke(method: string, ...args: unknown[]): unknown {
    const fn = this.mountedInstance?.[method];
    if (typeof fn !== 'function') return undefined;
    return fn.apply(this.mountedInstance, args);
  }

  focus(): void {
    const instance = this.mountedInstance as any;
    if (typeof instance?.focus === 'function') {
      instance.focus();
      return;
    }

    const element =
      instance?.elementRef?.nativeElement ??
      instance?.host?.nativeElement ??
      null;

    element?.focus?.({ preventScroll: true });
  }
}