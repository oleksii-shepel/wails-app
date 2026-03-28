import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { ViewerArchiveContext } from '../viewer-archive-context';
import { ViewerSurfaceComponent } from '../../surfaces/viewer/viewer-surface.component';
import type { FileProbe } from '../../shared/protocol';

@Component({
  selector: 'app-json-viewer',
  standalone: true,
  imports: [ViewerSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-viewer-surface
      [targetPath]="targetPath"
      [archiveContext]="archiveContext"
      [probe]="probe"
      [viewerId]="viewerId">
    </app-viewer-surface>
  `,
})
export class JsonViewerComponent {
  @Input() targetPath = '';
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() probe: FileProbe | null = null;
  @Input() viewerId: string | null = null;
}