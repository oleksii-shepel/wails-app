import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, Input, ViewChild } from '@angular/core';
import { DirSurfaceComponent } from '../../components/dir-surface.component';
import { SurfaceContext } from '../surface-context';

@Component({
  selector: 'app-dir-surface-tab',
  standalone: true,
  imports: [CommonModule, DirSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-dir-surface #dir [vm]="ctx.dirVm"></app-dir-surface>
  `,
})
export class DirSurfaceTabComponent {
  @Input({ required: true }) ctx!: SurfaceContext;
  @ViewChild('dir') private dir?: DirSurfaceComponent;

  // Exposed for SurfaceOutletComponent proxying.
  get addressBar() { return this.dir?.addressBar; }
  get fileViewContainer() { return this.dir?.fileViewContainer; }
  get findWidget() { return this.dir?.findWidget; }
}

