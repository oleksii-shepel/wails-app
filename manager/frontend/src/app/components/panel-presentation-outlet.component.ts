import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  Input,
  OnChanges,
  Type,
} from '@angular/core';
import type {
  PanelPresentationContribution,
  PanelPresentationModel,
} from '../plugin/plugin-registry-api';

@Component({
  selector: 'app-panel-presentation-outlet',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <ng-container
      *ngIf="component"
      [ngComponentOutlet]="component"
      [ngComponentOutletInputs]="{ model: model }">
    </ng-container>
  `,
  styles: [`
    :host {
      display: flex;
      flex: 1 1 0%;
      min-width: 0;
      min-height: 0;
      overflow: hidden;
    }
  `],
})
export class PanelPresentationOutletComponent implements OnChanges {
  @Input({ required: true }) presentation!: PanelPresentationContribution;
  @Input({ required: true }) model!: PanelPresentationModel;

  component: Type<unknown> | null = null;
  private resolveSeq = 0;
  private loadedId = '';

  constructor(private readonly cdr: ChangeDetectorRef) {}

  ngOnChanges(): void {
    if (!this.presentation) {
      this.component = null;
      this.loadedId = '';
      return;
    }

    if (this.loadedId === this.presentation.id && this.component) {
      this.cdr.markForCheck();
      return;
    }

    const seq = ++this.resolveSeq;
    const presentation = this.presentation;
    Promise.resolve(presentation.load()).then(component => {
      if (seq !== this.resolveSeq) return;
      this.component = component;
      this.loadedId = presentation.id;
      this.cdr.markForCheck();
    }).catch(() => {
      if (seq !== this.resolveSeq) return;
      this.component = null;
      this.loadedId = '';
      this.cdr.markForCheck();
    });
  }
}
