import { Component, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, map, takeUntil } from '@epikodelabs/streamix';
import { OperationService } from '../services/operation.service';
import { OperationState } from '../services/operation.types';

@Component({
  selector: 'app-status-bar',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="status-bar">
      <div class="status-left">
        <span class="status-text">{{ itemsSelected }} items selected</span>
        <span class="divider"></span>
        <span class="status-text">{{ totalSize }}</span>
      </div>
      
      <div class="status-center">
        <div 
          class="operations-pill" 
          *ngIf="hasActiveOperations"
          (click)="toggleOperationsPanel()"
          [class.has-completed]="hasCompleted"
        >
          <div class="pulse-dot" *ngIf="hasActiveOperations"></div>
          <span>{{ activeCount }} operation{{ activeCount !== 1 ? 's' : '' }}</span>
          <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2" fill="none" [class.rotated]="showPanel">
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
        </div>
      </div>

      <div class="status-right">
        <span class="status-text">{{ currentPath }}</span>
      </div>

      <!-- Mini Operations Panel -->
      <div class="mini-panel" *ngIf="showPanel">
        <div class="mini-header">
          <span>Active Operations</span>
          <button class="btn-text" (click)="collapseAll()">Collapse All</button>
        </div>
        <div class="mini-list">
          <div 
            class="mini-item" 
            *ngFor="let op of recentOperations"
            (click)="restoreOperation(op.id)"
            [class]="'status-' + op.status"
          >
            <div class="mini-icon" [style.--mini-color]="'var(' + op.config.themeColor + ')'">
              <div class="mini-progress" [style.width.%]="op.percentComplete"></div>
            </div>
            <div class="mini-info">
              <span class="mini-title">{{ op.config.title }}</span>
              <span class="mini-detail">{{ formatSpeed(op) }} • {{ formatETA(op) }}</span>
            </div>
            <div class="mini-percent">{{ op.percentComplete | number:'1.0-0' }}%</div>
          </div>
        </div>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      height: var(--vsc-statusbar-height, 22px);
      background: var(--vsc-statusbar-background, #007acc);
      border-top: 1px solid var(--vsc-border, #454545);
    }

    .status-bar {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 100%;
      padding: 0 var(--vsc-padding-md, 8px);
      position: relative;
      font-size: var(--vsc-font-size-small, 12px);
      color: var(--vsc-foreground, #cccccc);
      font-family: var(--vsc-font-family, -apple-system, BlinkMacSystemFont, sans-serif);
    }

    .status-left, .status-right {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-md, 8px);
    }

    .divider {
      width: 1px;
      height: 14px;
      background: var(--vsc-border, #454545);
    }

    .status-center {
      position: absolute;
      left: 50%;
      transform: translateX(-50%);
    }

    .operations-pill {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm, 4px);
      padding: 2px var(--vsc-padding-md, 8px);
      background: rgba(255, 255, 255, 0.1);
      border: 1px solid rgba(255, 255, 255, 0.2);
      border-radius: 11px;
      color: white;
      cursor: pointer;
      transition: all 0.2s;
      font-weight: 500;
      font-size: var(--vsc-font-size-small, 12px);
    }

    .operations-pill:hover {
      background: rgba(255, 255, 255, 0.2);
    }

    .operations-pill.has-completed {
      background: rgba(16, 185, 129, 0.3);
      border-color: rgba(16, 185, 129, 0.5);
    }

    .pulse-dot {
      width: 6px;
      height: 6px;
      background: currentColor;
      border-radius: 50%;
      animation: pulse 2s infinite;
    }

    @keyframes pulse {
      0%, 100% { opacity: 1; transform: scale(1); }
      50% { opacity: 0.5; transform: scale(0.8); }
    }

    .operations-pill svg {
      transition: transform 0.2s;
    }

    .operations-pill svg.rotated {
      transform: rotate(180deg);
    }

    .mini-panel {
      position: absolute;
      bottom: 100%;
      left: 50%;
      transform: translateX(-50%);
      margin-bottom: var(--vsc-padding-sm, 4px);
      background: var(--vsc-panel-background, #252526);
      border: 1px solid var(--vsc-border, #454545);
      border-radius: var(--vsc-border-radius-lg, 4px);
      padding: var(--vsc-padding-md, 8px);
      min-width: 280px;
      box-shadow: var(--vsc-widget-shadow, 0 5px 8px 0 rgba(0, 0, 0, 0.36));
      backdrop-filter: blur(20px);
      animation: slide-up 0.15s ease;
      z-index: 1000;
    }

    @keyframes slide-up {
      from { opacity: 0; transform: translateX(-50%) translateY(10px); }
      to { opacity: 1; transform: translateX(-50%) translateY(0); }
    }

    .mini-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: var(--vsc-padding-sm, 4px);
      padding-bottom: var(--vsc-padding-sm, 4px);
      border-bottom: 1px solid var(--vsc-border, #454545);
      font-weight: 600;
      color: var(--vsc-foreground-bright, #cccccc);
      font-size: var(--vsc-font-size-small, 12px);
    }

    .btn-text {
      background: none;
      border: none;
      color: var(--vsc-text-link, #3794ff);
      font-size: 11px;
      cursor: pointer;
      padding: 2px 6px;
      border-radius: var(--vsc-border-radius, 3px);
    }

    .btn-text:hover {
      background: var(--vsc-list-hover-background, #2a2d2e);
    }

    .mini-list {
      display: flex;
      flex-direction: column;
      gap: var(--vsc-padding-sm, 4px);
    }

    .mini-item {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm, 4px);
      padding: var(--vsc-padding-sm, 4px);
      background: var(--vsc-list-hover-background, #2a2d2e);
      border-radius: var(--vsc-border-radius, 3px);
      cursor: pointer;
      transition: all 0.15s;
    }

    .mini-item:hover {
      background: var(--vsc-list-focus-background, #062f4a);
    }

    .mini-icon {
      width: 28px;
      height: 28px;
      border-radius: var(--vsc-border-radius, 3px);
      background: rgba(255, 255, 255, 0.1);
      position: relative;
      overflow: hidden;
      flex-shrink: 0;
    }

    .mini-progress {
      position: absolute;
      bottom: 0;
      left: 0;
      right: 0;
      height: 3px;
      background: var(--mini-color, var(--vsc-accent-blue, #007acc));
      transition: width 0.3s;
    }

    .mini-item.status-completed .mini-icon { 
      background: rgba(16, 185, 129, 0.2); 
    }
    .mini-item.status-completed .mini-progress { 
      background: var(--vsc-success, #89d185); 
    }
    .mini-item.status-error .mini-icon { 
      background: rgba(239, 68, 68, 0.2); 
    }
    .mini-item.status-error .mini-progress { 
      background: var(--vsc-error, #f48771); 
    }
    .mini-item.status-cancelled .mini-icon { 
      background: rgba(245, 158, 11, 0.2); 
    }
    .mini-item.status-cancelled .mini-progress { 
      background: var(--vsc-warning, #cca700); 
    }

    .mini-info {
      flex: 1;
      display: flex;
      flex-direction: column;
      gap: 1px;
      min-width: 0;
    }

    .mini-title {
      font-size: 11px;
      font-weight: 500;
      color: var(--vsc-foreground, #cccccc);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .mini-detail {
      font-size: 10px;
      color: var(--vsc-foreground-dim, #888888);
      font-family: var(--vsc-font-family-mono, Consolas, monospace);
    }

    .mini-percent {
      font-size: 11px;
      font-weight: 600;
      color: var(--vsc-foreground-bright, #cccccc);
      font-family: var(--vsc-font-family-mono, Consolas, monospace);
      min-width: 32px;
      text-align: right;
    }
  `]
})
export class StatusBarComponent implements OnInit, OnDestroy {
  // Local state
  hasActiveOperations = false;
  showPanel = false;
  activeCount = 0;
  hasCompleted = false;
  recentOperations: OperationState[] = [];
  
  itemsSelected = 0;
  totalSize = '0 B';
  currentPath = '/';

  // Cleanup
  private destroy$ = createSubject<void>();

  constructor(private operationService: OperationService) {}

  ngOnInit(): void {
    // Subscribe to active count
    this.operationService.activeCount$.pipe(
      takeUntil(this.destroy$)
    ).subscribe(count => {
      this.activeCount = count;
      this.hasActiveOperations = count > 0;
    });

    // Subscribe to has active operations
    this.operationService.hasActiveOperations$.pipe(
      takeUntil(this.destroy$)
    ).subscribe(hasActive => {
      this.hasActiveOperations = hasActive;
    });

    // Subscribe to all operations to check for completed and get recent list
    this.operationService.operations$.pipe(
      takeUntil(this.destroy$)
    ).subscribe((ops: Map<string, OperationState>) => {
      // Check if any completed
      let hasCompleted = false;
      ops.forEach(op => {
        if (op.status === 'completed') hasCompleted = true;
      });
      this.hasCompleted = hasCompleted;

      // Get recent operations (first 5)
      const opsArray: OperationState[] = [];
      ops.forEach(op => opsArray.push(op));
      this.recentOperations = opsArray.slice(0, 5);
    });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  toggleOperationsPanel(): void {
    this.showPanel = !this.showPanel;
  }

  restoreOperation(id: string): void {
    this.operationService.restore(id);
    this.showPanel = false;
  }

  collapseAll(): void {
    this.operationService.collapseAll();
    this.showPanel = false;
  }

  formatSpeed(op: OperationState): string {
    return this.operationService.formatSpeed(op.timing.speedBytesPerSecond);
  }

  formatETA(op: OperationState): string {
    return this.operationService.formatDuration(op.timing.estimatedRemainingMs);
  }
}