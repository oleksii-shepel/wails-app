import { Component, OnInit, OnDestroy, Input, ElementRef } from '@angular/core';
import { CommonModule } from '@angular/common';
import { OperationService, OperationState } from '../services/operation.service';
import { OperationToastComponent } from './operation-toast.component';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

@Component({
  selector: 'app-operation-toast-stack',
  standalone: true,
  imports: [CommonModule, OperationToastComponent],
  template: `
<div class="stack" *ngIf="!collapsed && ops.length">
<app-operation-toast
*ngFor="let op of ops;trackBy:track"
[operation]="op"
(cancel)="svc.cancel($event)"
(dismiss)="svc.dismiss($event)"
(resolve)="svc.resolveConflict($event.conflictId,$event.resolution,$event.applyToAll)">
</app-operation-toast>
</div>
`,
  styles: [`
:host{
position:fixed;
right:16px;
bottom:30px;
z-index:1000;
}

.stack{
display:flex;
flex-direction:column;
gap:8px;
}
`]
})
export class OperationToastStackComponent implements OnInit, OnDestroy {

  @Input() collapsed = false;
  ops: OperationState[] = [];
  destroy$ = createSubject<void>();

  constructor(public svc: OperationService, public elementRef: ElementRef) { }

  ngOnInit() {
    this.svc.visibleOperations$
      .pipe(takeUntil(this.destroy$))
      .subscribe(o => this.ops = o);
  }

  ngOnDestroy() {
    this.destroy$.next();
    this.destroy$.complete();
  }

  track(_: number, o: OperationState) { return o.id; }
}