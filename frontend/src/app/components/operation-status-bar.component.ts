import { Component, OnInit, OnDestroy, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { 
  createSubject,
  takeUntil 
} from '@epikodelabs/streamix';
import { OperationService, OperationState } from '../services/operation.service';

@Component({
  selector: 'app-operation-status-bar',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="status-operations" *ngIf="hasOperations">
      <div 
        class="status-pill"
        [class.has-active]="hasActive"
        [class.toast-collapsed]="toastCollapsed"
        (click)="onPillClick()"
      >
        <div class="pulse-indicator" *ngIf="hasActive"></div>
        <div class="static-indicator" *ngIf="!hasActive"></div>
        <span class="pill-label">{{ count }} ops</span>
        <div class="pill-progress" *ngIf="overallProgress > 0">
          <div class="pill-progress-fill" [style.width.%]="overallProgress"></div>
        </div>
        <!-- Restore arrow when toast is collapsed -->
        <svg 
          *ngIf="toastCollapsed"
          class="restore-icon" 
          viewBox="0 0 24 24" 
          width="12" 
          height="12" 
          stroke="currentColor" 
          stroke-width="2" 
          fill="none"
        >
          <polyline points="6 15 12 9 18 15"></polyline>
        </svg>
      </div>

      <!-- Expanded Mini List -->
      <div class="operations-popover" *ngIf="expanded">
        <div class="popover-header">
          <span>Operations</span>
          <div class="header-actions">
            <button 
              *ngIf="toastCollapsed"
              class="btn-text restore-btn"
              (click)="restoreToastStack(); $event.stopPropagation()"
            >
              Show Toasts
            </button>
            <button 
              *ngIf="!toastCollapsed"
              class="btn-text"
              (click)="collapseToastStack(); $event.stopPropagation()"
            >
              Hide Toasts
            </button>
          </div>
        </div>
        <div class="popover-list">
          <div 
            class="popover-item"
            *ngFor="let op of operations"
            (click)="restoreOperation(op.id); $event.stopPropagation()"
            [class]="'status-' + op.status"
          >
            <div 
              class="item-icon"
              [style.color]="'var(' + op.config.themeColor + ')'"
            >
              <svg viewBox="0 0 24 24" width="14" height="14">
                <use [attr.href]="'#icon-' + op.config.icon"></use>
              </svg>
            </div>
            <span class="item-name">{{ op.config.title }}</span>
            <span class="item-percent">{{ op.percentComplete | number:'1.0-0' }}%</span>
            <div class="item-bar">
              <div 
                class="item-bar-fill"
                [style.background]="'var(' + op.config.themeColor + ')'"
                [style.width.%]="op.percentComplete"
              ></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: contents;
    }

    .status-operations {
      position: relative;
    }

    .status-pill {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      height: calc(var(--vsc-statusbar-height) - 4px);
      padding: 0 var(--vsc-padding-md);
      background: var(--vsc-badge-background);
      border-radius: var(--vsc-border-radius);
      cursor: pointer;
      transition: all var(--vsc-transition-fast);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-badge-foreground);
      border: 1px solid transparent;
    }

    .status-pill:hover {
      background: var(--vsc-list-hover-background);
    }

    .status-pill.has-active {
      background: color-mix(in srgb, var(--vsc-accent-blue) 20%, var(--vsc-badge-background));
      border-color: color-mix(in srgb, var(--vsc-accent-blue) 40%, transparent);
    }

    .status-pill.toast-collapsed {
      background: color-mix(in srgb, var(--vsc-accent-purple) 15%, var(--vsc-badge-background));
      border-color: color-mix(in srgb, var(--vsc-accent-purple) 30%, transparent);
    }

    .pulse-indicator {
      width: 6px;
      height: 6px;
      background: var(--vsc-accent-blue);
      border-radius: 50%;
      animation: pulse 1.5s ease-in-out infinite;
    }

    .static-indicator {
      width: 6px;
      height: 6px;
      background: var(--vsc-foreground-dim);
      border-radius: 50%;
      opacity: 0.5;
    }

    @keyframes pulse {
      0%, 100% { opacity: 1; transform: scale(1); }
      50% { opacity: 0.5; transform: scale(0.8); }
    }

    .pill-label {
      font-weight: 500;
    }

    .pill-progress {
      width: 40px;
      height: 3px;
      background: var(--vsc-border);
      border-radius: 1.5px;
      overflow: hidden;
    }

    .pill-progress-fill {
      height: 100%;
      background: var(--vsc-accent-blue);
      border-radius: 1.5px;
      transition: width var(--vsc-transition-fast);
    }

    .restore-icon {
      margin-left: 2px;
      stroke-linecap: round;
      stroke-linejoin: round;
      animation: bounce 2s infinite;
    }

    @keyframes bounce {
      0%, 100% { transform: translateY(0); }
      50% { transform: translateY(-2px); }
    }

    .operations-popover {
      position: absolute;
      bottom: calc(100% + 8px);
      right: 0;
      width: 260px;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-lg);
      box-shadow: var(--vsc-widget-shadow);
      animation: popover-enter 0.15s ease;
      z-index: var(--vsc-z-dropdown);
    }

    @keyframes popover-enter {
      from { opacity: 0; transform: translateY(8px); }
      to { opacity: 1; transform: translateY(0); }
    }

    .popover-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      border-bottom: 1px solid var(--vsc-border);
      font-size: var(--vsc-font-size-small);
      font-weight: 600;
      color: var(--vsc-foreground-bright);
    }

    .header-actions {
      display: flex;
      gap: var(--vsc-padding-sm);
    }

    .btn-text {
      background: none;
      border: none;
      color: var(--vsc-text-link);
      font-size: var(--vsc-font-size-small);
      cursor: pointer;
      padding: 2px 6px;
      border-radius: var(--vsc-border-radius);
      transition: all var(--vsc-transition-fast);
    }

    .btn-text:hover {
      background: var(--vsc-list-hover-background);
    }

    .restore-btn {
      color: var(--vsc-accent-purple);
      font-weight: 600;
    }

    .restore-btn:hover {
      background: color-mix(in srgb, var(--vsc-accent-purple) 20%, var(--vsc-list-hover-background));
    }

    .popover-list {
      max-height: 220px;
      overflow-y: auto;
    }

    .popover-item {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      cursor: pointer;
      transition: background var(--vsc-transition-fast);
      position: relative;
    }

    .popover-item:hover {
      background: var(--vsc-list-hover-background);
    }

    .popover-item.status-completed {
      opacity: 0.8;
    }

    .popover-item.status-error {
      background: color-mix(in srgb, var(--vsc-error) 5%, transparent);
    }

    .popover-item.status-error:hover {
      background: color-mix(in srgb, var(--vsc-error) 10%, var(--vsc-list-hover-background));
    }

    .item-icon {
      display: flex;
      align-items: center;
      justify-content: center;
    }

    .item-icon svg {
      stroke: currentColor;
      stroke-width: 2;
      fill: none;
    }

    .item-name {
      flex: 1;
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground);
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .item-percent {
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-dim);
      font-family: var(--vsc-font-family-mono);
      min-width: 35px;
      text-align: right;
    }

    .item-bar {
      position: absolute;
      bottom: 0;
      left: var(--vsc-padding-md);
      right: var(--vsc-padding-md);
      height: 2px;
      background: var(--vsc-border-subtle);
      border-radius: 1px;
      overflow: hidden;
    }

    .item-bar-fill {
      height: 100%;
      border-radius: 1px;
      transition: width var(--vsc-transition-fast);
    }
  `]
})
export class OperationStatusBarComponent implements OnInit, OnDestroy {
  @Input() toastCollapsed = false;
  @Output() toastCollapsedChange = new EventEmitter<boolean>();
  
  expanded = false;
  operations: OperationState[] = [];
  hasOperations = false;
  hasActive = false;
  count = 0;
  overallProgress = 0;

  private destroy$ = createSubject<void>();

  constructor(private operationService: OperationService) {}

  ngOnInit(): void {
    this.operationService.operations$.pipe(
      takeUntil(this.destroy$)
    ).subscribe(ops => {
      this.operations = Array.from(ops.values());
      this.hasOperations = this.operations.length > 0;
      this.count = this.operations.length;
      
      if (this.operations.length > 0) {
        const sum = this.operations.reduce((acc, op) => acc + op.percentComplete, 0);
        this.overallProgress = sum / this.operations.length;
      } else {
        this.overallProgress = 0;
      }
    });

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

  onPillClick(): void {
    this.expanded = !this.expanded;
  }

  collapseToastStack(): void {
    this.toastCollapsed = true;
    this.toastCollapsedChange.emit(true);
    this.expanded = false;
  }

  restoreToastStack(): void {
    this.toastCollapsed = false;
    this.toastCollapsedChange.emit(false);
    this.expanded = false;
  }

  restoreOperation(id: string): void {
    this.operationService.restore(id);
  }
}