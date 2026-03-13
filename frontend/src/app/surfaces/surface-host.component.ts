import { CommonModule } from '@angular/common';
import { Component, Input, ViewChild, ChangeDetectionStrategy } from '@angular/core';
import { DirSurfaceComponent } from '../components/dir-surface.component';
import { ViewerHostSurfaceComponent } from '../viewers/viewer-host-surface.component';
import { SizeManagerComponent } from './size-manager/size-manager.component';
import { SurfaceContext } from './surface-context';

@Component({
  selector: 'app-surface-host',
  standalone: true,
  imports: [CommonModule, DirSurfaceComponent, ViewerHostSurfaceComponent, SizeManagerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container [ngSwitch]="ctx.kind">
      <app-dir-surface
        #dirSurface
        *ngSwitchCase="'dir'"
        [vm]="$any(ctx).dirVm">
      </app-dir-surface>

      <app-viewer-host-surface
        *ngSwitchCase="'viewer'"
        [targetPath]="$any(ctx).viewer.targetPath"
        [archiveContext]="$any(ctx).viewer.archiveContext"
        [viewerId]="$any(ctx).viewer.viewerId">
      </app-viewer-host-surface>

      <app-size-manager
        *ngSwitchCase="'size-manager'"
        [ctx]="$any(ctx)">
      </app-size-manager>
    </ng-container>
  `,
})
export class SurfaceHostComponent {
  @Input({ required: true }) ctx!: SurfaceContext;
  @ViewChild('dirSurface') dirSurface?: DirSurfaceComponent;

  // Convenience accessors for FilePaneComponent.
  get addressBar() { return this.dirSurface?.addressBar; }
  get fileViewContainer() { return this.dirSurface?.fileViewContainer; }
  get findWidget() { return this.dirSurface?.findWidget; }
}
