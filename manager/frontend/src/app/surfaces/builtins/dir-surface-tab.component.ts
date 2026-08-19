import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, Input, ViewChild } from '@angular/core';
import { DirSurfaceComponent } from '../../components/dir-surface.component';
import { DirSurfaceContext } from '../surface-context';

@Component({
  selector: 'app-dir-surface-tab',
  standalone: true,
  imports: [CommonModule, DirSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-dir-surface
      #dir
      [vm]="ctx.dirVm"
      [surfaceId]="ctx.paneId + ':' + ctx.tab.id"
      [pinSurfaceId]="ctx.tab.surfaceId || 'dir'"
      [surfaceActive]="active"
      [surfacePath]="currentSurfacePath">
    </app-dir-surface>
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

    app-dir-surface {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }
  `],
})
export class DirSurfaceTabComponent {
  @Input({ required: true }) ctx!: DirSurfaceContext;
  @Input() active = false;
  @ViewChild('dir') private dir?: DirSurfaceComponent;

  get currentSurfacePath(): string {
    if (this.active) {
      return this.ctx?.dirVm?.currentPath || this.ctx?.tab?.path || '';
    }

    return this.ctx?.tab?.path || this.ctx?.dirVm?.currentPath || '';
  }

  // Exposed for SurfaceOutletComponent proxying.
  get addressBar() { return this.dir?.addressBar; }
  get fileViewContainer() { return this.dir?.fileViewContainer; }
  get findWidget() { return this.dir?.findWidget; }
}