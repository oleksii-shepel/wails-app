import { CommonModule, NgComponentOutlet } from '@angular/common';
import { ChangeDetectionStrategy, Component, Input, OnChanges, Type, ViewChild } from '@angular/core';
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
    :host { display: block; min-height: 0; min-width: 0; height: 100%; }
    .surface-empty { height: 100%; }
  `],
})
export class SurfaceOutletComponent implements OnChanges {
  @Input({ required: true }) ctx!: SurfaceContext;

  component: Type<unknown> | null = null;
  outletInputs: Record<string, unknown> = {};

  @ViewChild(NgComponentOutlet) private outlet?: NgComponentOutlet;

  private resolveSeq = 0;

  constructor(private registry: SurfaceRegistryService) {}

  ngOnChanges(): void {
    const seq = ++this.resolveSeq;
    this.outletInputs = { ctx: this.ctx };

    void this.registry.resolve({ tab: this.ctx.tab }).then(({ component }) => {
      if (seq !== this.resolveSeq) return;
      this.component = component;
    }).catch(() => {
      if (seq !== this.resolveSeq) return;
      this.component = null;
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
