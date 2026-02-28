import { Component, OnInit, OnDestroy, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { OperationService, OperationState } from '../services/operation.service';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

@Component({
  selector: 'app-operation-status-bar',
  standalone: true,
  imports: [CommonModule],
  template: `
<div *ngIf="count"
class="pill"
(click)="toggle()">

<span class="dot" [class.active]="active" [class.warn]="conflict"></span>

<span>{{count}} operation{{count>1?'s':''}}</span>

<div class="prog" *ngIf="active && !conflict">
<div class="fill" [style.width.%]="progress"></div>
</div>

<span *ngIf="conflict" class="warn-label">Conflict</span>

</div>
`,
  styles: [`
.pill{
display:flex;
align-items:center;
gap:8px;
padding:2px 10px;
border-radius:4px;
background:var(--vsc-statusBarItem-background);
cursor:pointer;
font-size:12px;
}

.pill:hover{background:var(--vsc-statusBarItem-hoverBackground)}

.dot{
width:6px;height:6px;border-radius:50%;
background:gray;
}

.dot.active{background:var(--vsc-accent-blue)}
.dot.warn{background:var(--vsc-warning)}

.prog{
width:40px;height:3px;
background:var(--vsc-border);
border-radius:2px;
overflow:hidden;
}

.fill{
height:100%;
background:var(--vsc-accent-blue);
}

.warn-label{
color:var(--vsc-warning);
font-weight:600;
}
`]
})
export class OperationStatusBarComponent implements OnInit, OnDestroy {

  @Input() collapsed = false;
  @Output() collapsedChange = new EventEmitter<boolean>();

  count = 0;
  active = false;
  conflict = false;
  progress = 0;

  destroy$ = createSubject<void>();

  constructor(private svc: OperationService) { }

  ngOnInit() {
    this.svc.operations$
      .pipe(takeUntil(this.destroy$))
      .subscribe(map => {
        const ops = [...map.values()];
        this.count = ops.length;

        const running = ops.filter(o => o.status === 'running');
        this.active = running.length > 0;

        this.progress = running.length
          ? running.reduce((a, o) => a + o.percentComplete, 0) / running.length
          : 0;

        this.conflict = ops.some(o => o.status === 'paused' && o.conflictData);
      });
  }

  ngOnDestroy() {
    this.destroy$.next();
    this.destroy$.complete();
  }

  toggle() {
    this.collapsed = !this.collapsed;
    this.collapsedChange.emit(this.collapsed);
  }
}