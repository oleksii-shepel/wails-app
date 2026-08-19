import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, Component, EventEmitter, Input, Output } from '@angular/core';
import { SizeManagerSurfaceComponent } from './size-manager-surface.component';
import type { SizeManagerSurfaceContext } from '../surface-context';

@Component({
  selector: 'app-size-manager-surface-tab',
  standalone: true,
  imports: [CommonModule, SizeManagerSurfaceComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-size-manager-surface
      [ctx]="ctx"
      (firstEntryArrived)="firstEntryArrived.emit()">
    </app-size-manager-surface>
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
  @Input() active = false;
  /** Bubbled up from the inner surface — parent uses this to drop its loader. */
  @Output() firstEntryArrived = new EventEmitter<void>();
}