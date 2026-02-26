import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { 
  createSubject,
  map, 
  takeUntil, 
  shareReplay 
} from '@epikodelabs/streamix';
import { OperationService } from '../services/operation.service';
import { OperationState } from '../services/operation.types';
import { OperationToastComponent } from './operation-toast.component';

@Component({
  selector: 'app-operation-toast-stack',
  standalone: true,
  imports: [CommonModule, OperationToastComponent],
  template: `
    <div class="toast-stack-host">
      <!-- Visible Toasts -->
      <div class="toast-stack" *ngIf="visibleOps.length > 0">
        <div 
          class="toast-item"
          *ngFor="let op of visibleOps; let i = index; trackBy: trackById"
          [style.--stack-index]="i"
        >
          <app-operation-toast
            [operation]="op"
            (minimize)="onMinimize($event)"
            (expand)="onExpand($event)"
            (collapse)="onCollapse($event)"
            (cancel)="onCancel($event)"
            (dismiss)="onDismiss($event)"
          ></app-operation-toast>
        </div>
      </div>

      <!-- Collapsed Indicator Pill -->
      <div 
        class="collapsed-pill"
        *ngIf="collapsedCount > 0"
        (click)="onRestoreAll()"
      >
        <div class="pill-content">
          <div class="activity-indicator" *ngIf="hasActive">
            <div class="indicator-dot"></div>
          </div>
          <span class="pill-text">{{ collapsedCount }} operation{{ collapsedCount > 1 ? 's' : '' }}</span>
          <svg class="pill-arrow" viewBox="0 0 24 24">
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
        </div>
        
        <!-- Mini progress bars -->
        <div class="pill-progress-bars" *ngIf="collapsedOps.length > 0">
          <div 
            *ngFor="let op of collapsedOps.slice(0, 4)"
            class="mini-bar"
            [style.background]="'var(' + op.config.themeColor + ')'"
            [style.width.%]="op.percentComplete"
          ></div>
        </div>
      </div>
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
      gap: var(--vsc-padding-md);
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

    .collapsed-pill {
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: 20px;
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      cursor: pointer;
      transition: all var(--vsc-transition-normal);
      box-shadow: var(--vsc-widget-shadow);
      min-width: 140px;
    }

    .collapsed-pill:hover {
      background: var(--vsc-list-hover-background);
      border-color: var(--vsc-border-bright);
      transform: translateY(-1px);
    }

    .pill-content {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground);
      font-weight: 500;
    }

    .activity-indicator {
      width: 14px;
      height: 14px;
      position: relative;
    }

    .indicator-dot {
      width: 8px;
      height: 8px;
      background: var(--vsc-accent-blue);
      border-radius: 50%;
      position: absolute;
      top: 50%;
      left: 50%;
      transform: translate(-50%, -50%);
      animation: pulse 1.5s ease-in-out infinite;
    }

    @keyframes pulse {
      0%, 100% { opacity: 1; transform: translate(-50%, -50%) scale(1); }
      50% { opacity: 0.5; transform: translate(-50%, -50%) scale(0.8); }
    }

    .pill-arrow {
      width: 14px;
      height: 14px;
      margin-left: auto;
      stroke: var(--vsc-foreground-dim);
      stroke-width: 2;
      fill: none;
      stroke-linecap: round;
      stroke-linejoin: round;
      transition: transform var(--vsc-transition-fast);
    }

    .collapsed-pill:hover .pill-arrow {
      transform: rotate(180deg);
    }

    .pill-progress-bars {
      display: flex;
      gap: 2px;
      margin-top: var(--vsc-padding-sm);
      height: 2px;
    }

    .mini-bar {
      flex: 1;
      height: 100%;
      border-radius: 1px;
      opacity: 0.7;
      transition: opacity var(--vsc-transition-fast);
    }

    .mini-bar:hover {
      opacity: 1;
    }
  `]
})
export class OperationToastStackComponent implements OnInit, OnDestroy {
  visibleOps: OperationState[] = [];
  collapsedOps: OperationState[] = [];
  collapsedCount = 0;
  hasActive = false;

  private destroy$ = createSubject<void>();

  constructor(private operationService: OperationService) {}

  ngOnInit(): void {
    // Subscribe to visible operations
    this.operationService.visibleOperations$.pipe(
      takeUntil(this.destroy$)
    ).subscribe(ops => {
      this.visibleOps = ops;
    });

    // Subscribe to collapsed operations
    this.operationService.collapsedOperations$.pipe(
      takeUntil(this.destroy$)
    ).subscribe(ops => {
      this.collapsedOps = ops;
      this.collapsedCount = ops.length;
    });

    // Subscribe to active count
    this.operationService.hasActiveOperations$.pipe(
      takeUntil(this.destroy$)
    ).subscribe(hasActive => {
      this.hasActive = hasActive;
    });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  trackById(index: number, op: OperationState): string {
    return op.id;
  }

  onMinimize(id: string): void {
    this.operationService.minimize(id);
  }

  onExpand(id: string): void {
    this.operationService.expand(id);
  }

  onCollapse(id: string): void {
    this.operationService.collapse(id);
  }

  onCancel(id: string): void {
    this.operationService.cancel(id);
  }

  onDismiss(id: string): void {
    this.operationService.dismiss(id);
  }

  onRestoreAll(): void {
    this.operationService.restoreAll();
  }
}