import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, Input } from '@angular/core';
import { SizeManagerSurfaceComponent } from './size-manager-surface.component';
import type { SizeManagerSurfaceContext } from '../surface-context';

@Component({
  selector: 'app-size-manager-surface-tab',
  standalone: true,
  imports: [CommonModule, SizeManagerSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-size-manager-surface [ctx]="ctx"></app-size-manager-surface>
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
  `],
})
export class SizeManagerSurfaceTabComponent {
  @Input({ required: true }) ctx!: SizeManagerSurfaceContext;
}