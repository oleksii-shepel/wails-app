import { CommonModule } from '@angular/common';
import { Component, Input, ViewChild, ChangeDetectionStrategy } from '@angular/core';
import { DirSurfaceComponent } from '../components/dir-surface.component';
import { ViewerHostSurfaceComponent } from '../viewers/viewer-host-surface.component';
import { SurfaceContext } from './surface-context';

@Component({
  selector: 'app-surface-host',
  standalone: true,
  imports: [CommonModule, DirSurfaceComponent, ViewerHostSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container [ngSwitch]="(ctx.tab.kind || 'dir')">
      <app-dir-surface
        #dirSurface
        *ngSwitchCase="'dir'"
        [vm]="ctx.dirVm">
      </app-dir-surface>

      <app-viewer-host-surface
        *ngSwitchCase="'viewer'"
        [targetPath]="ctx.viewer.targetPath"
        [archiveContext]="ctx.viewer.archiveContext"
        [viewerId]="ctx.viewer.viewerId">
      </app-viewer-host-surface>
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
