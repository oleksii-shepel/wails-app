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
    <ng-container [ngSwitch]="ctx.kind">
      <app-dir-surface
        #dirSurface
        *ngSwitchCase="'dir'"
        [vm]="$any(ctx).dirVm">
      </app-dir-surface>

      <app-viewer-host-surface
        *ngSwitchCase="'viewer'"
        [targetPath]="$any(ctx).viewer.targetPath"
        [fileList]="$any(ctx).viewer.fileList"
        [archiveContext]="$any(ctx).viewer.archiveContext"
        [viewerId]="$any(ctx).viewer.viewerId">
      </app-viewer-host-surface>
    </ng-container>
  `,
})
export class SurfaceHostComponent {
  @Input({ required: true }) ctx!: SurfaceContext;
  @ViewChild('dirSurface') dirSurface?: DirSurfaceComponent;
  @ViewChild(ViewerHostSurfaceComponent) viewerHost?: ViewerHostSurfaceComponent;

  // Convenience accessors for FilePaneComponent.
  get addressBar() { return this.dirSurface?.addressBar; }
  get fileViewContainer() { return this.dirSurface?.fileViewContainer; }
  get findWidget() { return this.dirSurface?.findWidget; }
  get viewer() { return this.viewerHost?.mountedViewer; }
}
