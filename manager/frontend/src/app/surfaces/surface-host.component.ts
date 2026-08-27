import { CommonModule } from '@angular/common';
import { Component, Input, ViewChild, ChangeDetectionStrategy } from '@angular/core';
import { ViewerHostSurfaceComponent } from '../viewers/viewer-host-surface.component';
import { DirSurfaceTabComponent } from './builtins/dir-surface-tab.component';
import { SurfaceContext } from './surface-context';

@Component({
  selector: 'app-surface-host',
  standalone: true,
  imports: [CommonModule, DirSurfaceTabComponent, ViewerHostSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container [ngSwitch]="ctx.kind">
      <app-dir-surface-tab
        #dirSurface
        *ngSwitchCase="'dir'"
        [ctx]="$any(ctx)"
        [active]="true">
      </app-dir-surface-tab>

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
  @ViewChild('dirSurface') dirSurface?: DirSurfaceTabComponent;
  @ViewChild(ViewerHostSurfaceComponent) viewerHost?: ViewerHostSurfaceComponent;

  // Convenience accessors for pane host.
  get addressBar() { return this.dirSurface?.addressBar; }
  get fileViewContainer() { return this.dirSurface?.fileViewContainer; }
  get findWidget() { return this.dirSurface?.findWidget; }
  get viewer() { return this.viewerHost?.mountedViewer; }
}
