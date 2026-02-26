import { Component, Input, Output, EventEmitter } from '@angular/core';
import { CommonModule } from '@angular/common';
import { OperationState } from '../services/operation.types';
import { OperationService } from '../services/operation.service';

@Component({
  selector: 'app-operation-toast',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div 
      class="operation-toast"
      [class]="'status-' + operation.status"
      [class.minimized]="operation.isMinimized"
      [style.--op-color]="'var(' + operation.config.themeColor + ')'"
    >
      <!-- Icon Ring -->
      <div class="toast-icon-ring">
        <svg class="icon-svg" viewBox="0 0 24 24">
          <use [attr.href]="'#icon-' + operation.config.icon"></use>
        </svg>
        <svg class="progress-ring" viewBox="0 0 36 36">
          <path class="ring-bg" d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"/>
          <path 
            class="ring-fill" 
            [attr.stroke-dasharray]="operation.percentComplete + ', 100'"
            d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
          />
        </svg>
      </div>

      <!-- Content -->
      <div class="toast-content" (click)="toggleMinimize()">
        <div class="toast-header">
          <span class="toast-title">{{ operation.config.title }}</span>
          <span class="toast-percent" *ngIf="!operation.isMinimized">
            {{ operation.percentComplete | number:'1.0-0' }}%
          </span>
          <span class="toast-percent-mini" *ngIf="operation.isMinimized">
            {{ operation.percentComplete | number:'1.0-0' }}%
          </span>
        </div>

        <div class="toast-body" *ngIf="!operation.isMinimized">
          <div class="file-path" [title]="operation.metrics.currentFile">
            <span class="path-text">{{ currentFileName }}</span>
          </div>

          <div class="stats-grid">
            <div class="stat">
              <span class="stat-label">Speed</span>
              <span class="stat-value speed">{{ svc.formatSpeed(operation.timing.speedBytesPerSecond) }}</span>
            </div>
            <div class="stat">
              <span class="stat-label">ETA</span>
              <span class="stat-value">{{ svc.formatDuration(operation.timing.estimatedRemainingMs) }}</span>
            </div>
            <div class="stat">
              <span class="stat-label">Size</span>
              <span class="stat-value">
                {{ svc.formatBytes(operation.metrics.processedBytes) }} / 
                {{ svc.formatBytes(operation.metrics.totalBytes) }}
              </span>
            </div>
          </div>

          <div class="progress-track">
            <div class="progress-fill" [style.width.%]="operation.percentComplete">
              <div class="progress-shimmer" *ngIf="operation.status === 'running'"></div>
            </div>
          </div>

          <div class="items-counter" *ngIf="operation.metrics.totalItems > 1">
            {{ operation.metrics.processedItems }} of {{ operation.metrics.totalItems }} items
          </div>
        </div>
      </div>

      <!-- Actions -->
      <div class="toast-actions">
        <button 
          class="action-btn minimize" 
          (click)="onMinimize($event)"
          [title]="operation.isMinimized ? 'Expand' : 'Minimize'"
        >
          <svg viewBox="0 0 24 24" *ngIf="!operation.isMinimized">
            <line x1="5" y1="12" x2="19" y2="12"></line>
          </svg>
          <svg viewBox="0 0 24 24" *ngIf="operation.isMinimized">
            <polyline points="6 9 12 15 18 9"></polyline>
          </svg>
        </button>

        <button 
          class="action-btn collapse" 
          (click)="onCollapse($event)"
          title="Hide to status bar"
        >
          <svg viewBox="0 0 24 24">
            <path d="M8 3v3a2 2 0 0 1-2 2H3m18 0h-3a2 2 0 0 1-2-2V3m0 18v-3a2 2 0 0 1 2-2h3M3 16h3a2 2 0 0 1 2 2v3"></path>
          </svg>
        </button>

        <button 
          class="action-btn cancel" 
          *ngIf="operation.status === 'running' && operation.config.cancellable"
          (click)="onCancel($event)"
          title="Cancel"
        >
          <svg viewBox="0 0 24 24">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>

        <button 
          class="action-btn close" 
          *ngIf="isCompleted"
          (click)="onDismiss($event)"
          title="Dismiss"
        >
          <svg viewBox="0 0 24 24">
            <line x1="18" y1="6" x2="6" y2="18"></line>
            <line x1="6" y1="6" x2="18" y2="18"></line>
          </svg>
        </button>
      </div>

      <!-- Status Badge -->
      <div class="status-badge" [class]="operation.status" *ngIf="isCompleted">
        <svg *ngIf="operation.status === 'completed'" viewBox="0 0 24 24">
          <polyline points="20 6 9 17 4 12"></polyline>
        </svg>
        <svg *ngIf="operation.status === 'error'" viewBox="0 0 24 24">
          <line x1="12" y1="8" x2="12" y2="12"></line>
          <line x1="12" y1="16" x2="12.01" y2="16"></line>
        </svg>
        <svg *ngIf="operation.status === 'cancelled'" viewBox="0 0 24 24">
          <rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect>
        </svg>
      </div>

      <!-- Error Banner -->
      <div class="error-banner" *ngIf="operation.error && !operation.isMinimized">
        <svg viewBox="0 0 24 24">
          <circle cx="12" cy="12" r="10"></circle>
          <line x1="12" y1="8" x2="12" y2="12"></line>
          <line x1="12" y1="16" x2="12.01" y2="16"></line>
        </svg>
        <span>{{ operation.error }}</span>
      </div>
    </div>
  `,
  styles: [`
    :host {
      display: block;
      animation: toast-enter 0.25s var(--vsc-transition-normal) both;
    }

    @keyframes toast-enter {
      from {
        opacity: 0;
        transform: translateX(20px) scale(0.96);
      }
      to {
        opacity: 1;
        transform: translateX(0) scale(1);
      }
    }

    .operation-toast {
      --op-color: var(--vsc-accent-blue);
      
      position: relative;
      display: flex;
      align-items: flex-start;
      gap: var(--vsc-padding-md);
      width: 380px;
      padding: var(--vsc-padding-lg);
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-lg);
      box-shadow: var(--vsc-widget-shadow);
      font-family: var(--vsc-font-family);
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground);
      overflow: hidden;
      transition: all var(--vsc-transition-normal);
    }

    .operation-toast:hover {
      border-color: var(--vsc-border-bright);
      box-shadow: 0 8px 16px 0 rgba(0, 0, 0, 0.48);
    }

    .operation-toast::before {
      content: '';
      position: absolute;
      left: 0;
      top: 0;
      bottom: 0;
      width: 3px;
      background: var(--op-color);
    }

    .operation-toast.status-completed::before {
      background: var(--vsc-success);
    }
    .operation-toast.status-error::before,
    .operation-toast.status-error .ring-fill {
      background: var(--vsc-error);
      stroke: var(--vsc-error);
    }
    .operation-toast.status-cancelled::before {
      background: var(--vsc-warning);
    }

    .toast-icon-ring {
      position: relative;
      width: 40px;
      height: 40px;
      flex-shrink: 0;
    }

    .icon-svg {
      position: absolute;
      inset: 8px;
      width: 24px;
      height: 24px;
      color: var(--op-color);
    }

    .progress-ring {
      width: 100%;
      height: 100%;
      transform: rotate(-90deg);
    }

    .ring-bg {
      fill: none;
      stroke: var(--vsc-border);
      stroke-width: 2.5;
    }

    .ring-fill {
      fill: none;
      stroke: var(--op-color);
      stroke-width: 2.5;
      stroke-linecap: round;
      transition: stroke-dasharray var(--vsc-transition-fast);
    }

    .toast-content {
      flex: 1;
      min-width: 0;
      cursor: pointer;
    }

    .toast-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: var(--vsc-padding-sm);
    }

    .toast-title {
      font-weight: 600;
      color: var(--vsc-foreground-bright);
    }

    .toast-percent {
      font-size: var(--vsc-font-size-small);
      font-weight: 700;
      color: var(--op-color);
      font-family: var(--vsc-font-family-mono);
    }

    .toast-percent-mini {
      font-size: var(--vsc-font-size);
      font-weight: 700;
      color: var(--op-color);
    }

    .toast-body {
      display: flex;
      flex-direction: column;
      gap: var(--vsc-padding-sm);
    }

    .file-path {
      font-family: var(--vsc-font-family-mono);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-dim);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .stats-grid {
      display: grid;
      grid-template-columns: 1fr 1fr 1fr;
      gap: var(--vsc-padding-sm);
      font-size: var(--vsc-font-size-small);
    }

    .stat {
      display: flex;
      flex-direction: column;
      gap: 1px;
    }

    .stat-label {
      color: var(--vsc-foreground-dim);
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: 0.3px;
    }

    .stat-value {
      color: var(--vsc-foreground);
      font-family: var(--vsc-font-family-mono);
    }

    .stat-value.speed {
      color: var(--op-color);
      font-weight: 600;
    }

    .progress-track {
      height: 4px;
      background: var(--vsc-border-subtle);
      border-radius: 2px;
      overflow: hidden;
      margin-top: var(--vsc-padding-xs);
    }

    .progress-fill {
      height: 100%;
      background: var(--op-color);
      border-radius: 2px;
      transition: width var(--vsc-transition-fast);
      position: relative;
      overflow: hidden;
    }

    .progress-shimmer {
      position: absolute;
      inset: 0;
      background: linear-gradient(90deg, transparent, rgba(255,255,255,0.25), transparent);
      animation: shimmer 1.5s infinite;
    }

    @keyframes shimmer {
      0% { transform: translateX(-100%); }
      100% { transform: translateX(200%); }
    }

    .items-counter {
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-dim);
      text-align: center;
    }

    .toast-actions {
      display: flex;
      flex-direction: column;
      gap: var(--vsc-padding-xs);
      opacity: 0;
      transition: opacity var(--vsc-transition-fast);
    }

    .operation-toast:hover .toast-actions {
      opacity: 1;
    }

    .action-btn {
      width: 24px;
      height: 24px;
      display: flex;
      align-items: center;
      justify-content: center;
      background: var(--vsc-button-secondary);
      border: none;
      border-radius: var(--vsc-border-radius);
      color: var(--vsc-foreground-dim);
      cursor: pointer;
      transition: all var(--vsc-transition-fast);
    }

    .action-btn:hover {
      background: var(--vsc-button-secondary-hover);
      color: var(--vsc-foreground);
    }

    .action-btn svg {
      width: 14px;
      height: 14px;
      stroke: currentColor;
      stroke-width: 2;
      fill: none;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    .action-btn.cancel:hover {
      background: color-mix(in srgb, var(--vsc-error) 20%, transparent);
      color: var(--vsc-error);
    }

    .action-btn.close:hover {
      background: var(--vsc-error);
      color: white;
    }

    .status-badge {
      position: absolute;
      top: var(--vsc-padding-sm);
      right: var(--vsc-padding-sm);
      width: 18px;
      height: 18px;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: 50%;
      animation: badge-pop 0.3s var(--vsc-transition-normal) both;
    }

    @keyframes badge-pop {
      from { transform: scale(0); opacity: 0; }
      to { transform: scale(1); opacity: 1; }
    }

    .status-badge.completed {
      background: color-mix(in srgb, var(--vsc-success) 20%, transparent);
      color: var(--vsc-success);
    }

    .status-badge.error {
      background: color-mix(in srgb, var(--vsc-error) 20%, transparent);
      color: var(--vsc-error);
    }

    .status-badge.cancelled {
      background: color-mix(in srgb, var(--vsc-warning) 20%, transparent);
      color: var(--vsc-warning);
    }

    .status-badge svg {
      width: 12px;
      height: 12px;
      stroke: currentColor;
      stroke-width: 2.5;
      fill: none;
      stroke-linecap: round;
      stroke-linejoin: round;
    }

    .error-banner {
      position: absolute;
      bottom: 0;
      left: 0;
      right: 0;
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      background: color-mix(in srgb, var(--vsc-error) 10%, transparent);
      border-top: 1px solid color-mix(in srgb, var(--vsc-error) 30%, transparent);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-error);
      animation: slide-up 0.2s ease;
    }

    @keyframes slide-up {
      from { transform: translateY(100%); }
      to { transform: translateY(0); }
    }

    .error-banner svg {
      width: 14px;
      height: 14px;
      flex-shrink: 0;
      stroke: currentColor;
      stroke-width: 2;
      fill: none;
    }

    .operation-toast.minimized {
      width: 220px;
      padding: var(--vsc-padding-md);
    }

    .operation-toast.minimized .toast-body,
    .operation-toast.minimized .toast-actions {
      display: none;
    }

    .operation-toast.minimized .toast-header {
      margin: 0;
    }

    .operation-toast.minimized .toast-icon-ring {
      width: 32px;
      height: 32px;
    }

    .operation-toast.minimized .icon-svg {
      inset: 6px;
      width: 20px;
      height: 20px;
    }

    .operation-toast.minimized .action-btn.minimize {
      opacity: 1;
    }
  `]
})
export class OperationToastComponent {
  @Input() operation!: OperationState;
  @Output() minimize = new EventEmitter<string>();
  @Output() expand = new EventEmitter<string>();
  @Output() collapse = new EventEmitter<string>();
  @Output() cancel = new EventEmitter<string>();
  @Output() dismiss = new EventEmitter<string>();

  constructor(public svc: OperationService) {}

  get currentFileName(): string {
    const path = this.operation.metrics.currentFile || '';
    return path.split(/[\\/]/).pop() || path;
  }

  get isCompleted(): boolean {
    return ['completed', 'error', 'cancelled'].includes(this.operation.status);
  }

  toggleMinimize(): void {
    if (this.operation.isMinimized) {
      this.expand.emit(this.operation.id);
    } else {
      this.minimize.emit(this.operation.id);
    }
  }

  onMinimize(event: Event): void {
    event.stopPropagation();
    this.minimize.emit(this.operation.id);
  }

  onCollapse(event: Event): void {
    event.stopPropagation();
    this.collapse.emit(this.operation.id);
  }

  onCancel(event: Event): void {
    event.stopPropagation();
    this.cancel.emit(this.operation.id);
  }

  onDismiss(event: Event): void {
    event.stopPropagation();
    this.dismiss.emit(this.operation.id);
  }
}