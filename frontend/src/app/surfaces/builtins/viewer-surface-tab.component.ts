import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, Input, ViewChild } from '@angular/core';
import { ViewerHostSurfaceComponent } from '../../viewers/viewer-host-surface.component';
import { ViewerSurfaceContext } from '../surface-context';

@Component({
  selector: 'app-viewer-surface-tab',
  standalone: true,
  imports: [CommonModule, ViewerHostSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-viewer-host-surface
      [targetPath]="ctx.viewer.targetPath"
      [fileList]="ctx.viewer.fileList"
      [archiveContext]="ctx.viewer.archiveContext"
      [viewerId]="ctx.viewer.viewerId">
    </app-viewer-host-surface>
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

    app-viewer-host-surface {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }
  `],
})
export class ViewerSurfaceTabComponent {
  @Input({ required: true }) ctx!: ViewerSurfaceContext;

  @ViewChild(ViewerHostSurfaceComponent) viewerHost?: ViewerHostSurfaceComponent;
}

