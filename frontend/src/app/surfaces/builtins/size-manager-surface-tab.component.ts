import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { SizeManagerComponent } from '../size-manager/size-manager.component';
import { SizeManagerSurfaceContext } from '../surface-context';

@Component({
  selector: 'app-size-manager-surface-tab',
  standalone: true,
  imports: [CommonModule, SizeManagerComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `<app-size-manager [ctx]="ctx"></app-size-manager>`,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }

    app-size-manager {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      overflow: hidden;
    }
  `],
})
export class SizeManagerSurfaceTabComponent {
  @Input({ required: true }) ctx!: SizeManagerSurfaceContext;
}
