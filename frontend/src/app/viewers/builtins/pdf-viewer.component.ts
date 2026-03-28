import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { ViewerArchiveContext } from '../viewer-archive-context';
import { ViewerSurfaceComponent } from '../../surfaces/viewer/viewer-surface.component';
import type { FileProbe } from '../../shared/protocol';

@Component({
  selector: 'app-pdf-viewer',
  standalone: true,
  imports: [ViewerSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  styles: [`
    :host {
      display: flex;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      height: 100%;
      width: 100%;
      overflow: hidden;
    }

    app-viewer-surface {
      display: flex;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      width: 100%;
      height: 100%;
      overflow: hidden;
    }
  `],
  template: `
    <app-viewer-surface
      [targetPath]="targetPath"
      [archiveContext]="archiveContext"
      [hideHeader]="hideHeader"
      [probe]="probe"
      [viewerId]="viewerId">
    </app-viewer-surface>
  `,
})
export class PdfViewerComponent {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() hideHeader = false;
  @Input() probe: FileProbe | null = null;
  @Input() viewerId: string | null = null;
}