import { Component, OnInit, OnDestroy, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { 
  createSubject,
  takeUntil 
} from '@epikodelabs/streamix';
import { OperationService } from '../services/operation.service';
import { OperationState } from '../services/operation.service'; // Import from service
import { OperationToastComponent } from './operation-toast.component';

@Component({
  selector: 'app-operation-toast-stack',
  standalone: true,
  imports: [CommonModule, OperationToastComponent],
  template: `
    <div class="toast-stack-host" *ngIf="!isGloballyCollapsed && visibleOps.length > 0">
      <div class="toast-stack">
        <div 
          class="toast-item"
          *ngFor="let op of visibleOps; let i = index; trackBy: trackById"
          [style.--stack-index]="i"
        >
          <app-operation-toast
            [operation]="op"
            (minimize)="onMinimize($event)"
            (expand)="onExpand($event)"
            (collapse)="onGlobalCollapse()"
            (cancel)="onCancel($event)"
            (dismiss)="onDismiss($event)"
          ></app-operation-toast>
        </div>
      </div>

      <!-- Global Collapse Button (shown when multiple toasts) -->
      <button 
        *ngIf="visibleOps.length > 1"
        class="global-collapse-btn"
        (click)="onGlobalCollapse()"
        title="Collapse all to status bar"
      >
        <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2" fill="none">
          <path d="M8 3v3a2 2 0 0 1-2 2H3m18 0h-3a2 2 0 0 1-2-2V3m0 18v-3a2 2 0 0 1 2-2h3M3 16h3a2 2 0 0 1 2 2v3"></path>
        </svg>
        <span>Collapse</span>
      </button>
    </div>
  `,
  styles: [`
    :host {
      position: fixed;
      top: calc(var(--vsc-titlebar-height) + 20px);
      right: 20px;
      z-index: var(--vsc-z-modal);
      pointer-events: none;
    }

    .toast-stack-host {
      display: flex;
      flex-direction: column;
      align-items: flex-end;
      gap: var(--vsc-padding-sm);
      pointer-events: auto;
    }

    .toast-stack {
      display: flex;
      flex-direction: column;
      gap: var(--vsc-padding-sm);
    }

    .toast-item {
      --stack-index: 0;
      animation: stack-enter 0.3s ease both;
      animation-delay: calc(var(--stack-index) * 50ms);
      transition: all var(--vsc-transition-normal);
    }

    @keyframes stack-enter {
      from {
        opacity: 0;
        transform: translateX(30px) scale(0.95);
      }
      to {
        opacity: 1;
        transform: translateX(0) scale(1);
      }
    }

    .toast-item:nth-child(n+2) {
      margin-top: -4px;
    }

    .toast-item:hover {
      transform: translateX(-4px);
      z-index: 10;
    }

    .toast-item:hover ~ .toast-item {
      opacity: 0.7;
    }

    .global-collapse-btn {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-xs) var(--vsc-padding-md);
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: 20px;
      color: var(--vsc-foreground-dim);
      cursor: pointer;
      font-size: var(--vsc-font-size-small);
      font-family: var(--vsc-font-family);
      transition: all var(--vsc-transition-fast);
      box-shadow: var(--vsc-widget-shadow);
      margin-top: var(--vsc-padding-xs);
    }

    .global-collapse-btn:hover {
      background: var(--vsc-list-hover-background);
      border-color: var(--vsc-border-bright);
      color: var(--vsc-foreground);
    }

    .global-collapse-btn svg {
      stroke-linecap: round;
      stroke-linejoin: round;
    }
  `]
})
export class OperationToastStackComponent implements OnInit, OnDestroy {
  @Input() isGloballyCollapsed = false;
  @Output() isGloballyCollapsedChange = new EventEmitter<boolean>();
  
  visibleOps: OperationState[] = [];
  private destroy$ = createSubject<void>();

  constructor(private operationService: OperationService) {}

  ngOnInit(): void {
    // Subscribe to all non-collapsed operations (individual collapse)
    this.operationService.visibleOperations$.pipe(
      takeUntil(this.destroy$)
    ).subscribe(ops => {
      this.visibleOps = ops;
    });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  trackById(index: number, op: OperationState): string {
    return op.id;
  }

  onGlobalCollapse(): void {
    this.isGloballyCollapsed = true;
    this.isGloballyCollapsedChange.emit(true);
    // Also collapse all individual operations
    this.operationService.collapseAll();
  }

  restore(): void {
    this.isGloballyCollapsed = false;
    this.isGloballyCollapsedChange.emit(false);
    // Restore all individual operations
    this.operationService.restoreAll();
  }

  onMinimize(id: string): void {
    this.operationService.minimize(id);
  }

  onExpand(id: string): void {
    this.operationService.expand(id);
  }

  onCancel(id: string): void {
    this.operationService.cancel(id);
  }

  onDismiss(id: string): void {
    this.operationService.dismiss(id);
  }
}