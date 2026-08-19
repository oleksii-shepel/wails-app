// operation-status-bar.component.ts - Updated with stack toggle
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
        [class.has-conflict]="hasConflict"
        [class.is-expanded]="!toastCollapsed"
        (click)="onPillClick()"
      >
        <div class="pill-indicator" [class.pulse]="hasActive" [class.warning]="hasConflict"></div>
        
        <span class="pill-label">{{ count }} operation{{ count !== 1 ? 's' : '' }}</span>
        
        <div class="pill-progress" *ngIf="hasActive && !hasConflict && overallProgress > 0">
          <div class="pill-progress-fill" [style.width.%]="overallProgress"></div>
        </div>

        <span class="conflict-badge" *ngIf="hasConflict">⚠️ Conflict</span>

        <svg 
          class="expand-icon"
          [class.expanded]="!toastCollapsed"
          viewBox="0 0 24 24" 
          width="12" 
          height="12" 
          stroke="currentColor" 
          stroke-width="2" 
          fill="none"
        >
          <polyline points="6 9 12 15 18 9"></polyline>
        </svg>
      </div>
    </div>
  `,
  styles: [`
    :host { display: contents; }
    
    .status-operations { position: relative; }
    
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
      user-select: none;
    }

    .status-pill:hover {
      background: var(--vsc-list-hover-background);
      border-color: var(--vsc-border-bright);
    }

    .status-pill.has-active {
      background: color-mix(in srgb, var(--vsc-accent-blue) 15%, var(--vsc-badge-background));
    }

    .status-pill.has-conflict {
      background: color-mix(in srgb, var(--vsc-warning) 20%, var(--vsc-badge-background));
      border-color: color-mix(in srgb, var(--vsc-warning) 40%, transparent);
      animation: conflict-glow 2s infinite;
    }

    @keyframes conflict-glow {
      0%, 100% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--vsc-warning) 40%, transparent); }
      50% { box-shadow: 0 0 6px 0 color-mix(in srgb, var(--vsc-warning) 60%, transparent); }
    }

    .status-pill.is-expanded {
      background: var(--vsc-list-hover-background);
    }

    .pill-indicator {
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: var(--vsc-foreground-dim);
      opacity: 0.5;
      flex-shrink: 0;
    }

    .pill-indicator.pulse {
      background: var(--vsc-accent-blue);
      opacity: 1;
      animation: pulse 1.5s ease-in-out infinite;
    }

    .pill-indicator.warning {
      background: var(--vsc-warning);
      opacity: 1;
      animation: pulse 1s ease-in-out infinite;
    }

    @keyframes pulse {
      0%, 100% { opacity: 1; transform: scale(1); }
      50% { opacity: 0.5; transform: scale(0.8); }
    }

    .pill-label { font-weight: 500; white-space: nowrap; }
    
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

    .conflict-badge {
      font-size: 10px;
      font-weight: 600;
      color: var(--vsc-warning);
      background: color-mix(in srgb, var(--vsc-warning) 15%, transparent);
      padding: 1px 6px;
      border-radius: 10px;
      white-space: nowrap;
    }

    .expand-icon {
      margin-left: 2px;
      stroke-linecap: round;
      stroke-linejoin: round;
      transition: transform var(--vsc-transition-fast);
      flex-shrink: 0;
    }

    .expand-icon.expanded { transform: rotate(180deg); }
  `]
})
export class OperationStatusBarComponent implements OnInit, OnDestroy {
  @Input() toastCollapsed = false;
  @Input() stackElement?: Element;
  @Output() toastCollapsedChange = new EventEmitter<boolean>();
  
  operations: OperationState[] = [];
  hasOperations = false;
  hasActive = false;
  hasConflict = false;
  count = 0;
  overallProgress = 0;

  private destroy$ = createSubject<void>();
  private _hideTimer: ReturnType<typeof setTimeout> | null = null;
  private _toastDismissTimer: ReturnType<typeof setTimeout> | null = null;
  private _mutationSub: { unsubscribe(): void } | null = null;

  constructor(private operationService: OperationService) {}

  ngOnInit(): void {
    this.operationService.operations$.pipe(
      takeUntil(this.destroy$)
    ).subscribe(ops => {
      this.operations = Array.from(ops.values());
      this.hasOperations = this.operations.length > 0;
      this.count = this.operations.length;
      
      const activeOps = this.operations.filter(op => op.status === 'running');
      this.hasActive = activeOps.length > 0;
      
      if (activeOps.length > 0) {
        const sum = activeOps.reduce((acc, op) => acc + op.percentComplete, 0);
        this.overallProgress = sum / activeOps.length;
      } else {
        this.overallProgress = 0;
      }

      const conflictOp = this.operations.find(
        op => op.status === 'paused' && !!op.conflictData
      );
      const newHasConflict = !!conflictOp;
      
      if (newHasConflict && !this.hasConflict && this.toastCollapsed) {
        this.toastCollapsed = false;
        this.toastCollapsedChange.emit(false);
      }
      
      this.hasConflict = newHasConflict;
    });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  onPillClick(): void {
    this.toastCollapsed = !this.toastCollapsed;
    this.toastCollapsedChange.emit(this.toastCollapsed);
  }
}