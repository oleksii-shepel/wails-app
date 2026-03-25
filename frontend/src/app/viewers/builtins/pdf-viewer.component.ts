import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { ViewerArchiveContext } from '../viewer-archive-context';
import { ViewerSurfaceComponent } from '../../surfaces/viewer/viewer-surface.component';

@Component({
  selector: 'app-pdf-viewer',
  standalone: true,
  imports: [ViewerSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-viewer-surface
      [targetPath]="targetPath"
      [archiveContext]="archiveContext"
      [viewerId]="viewerId">
    </app-viewer-surface>
  `,
})
export class PdfViewerComponent {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
}