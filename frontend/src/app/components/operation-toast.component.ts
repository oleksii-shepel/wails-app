// operation-toast.component.ts
import { Component, Input, Output, EventEmitter, OnInit } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { OperationService, OperationState } from '../services/operation.service';
import { ConflictResolution } from '../shared/protocol-enhanced';

@Component({
  selector: 'app-operation-toast',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
    <div 
      class="operation-toast"
      [class]="'status-' + operation.status"
      [class.minimized]="operation.isMinimized"
      [class.has-conflict]="hasConflict"
      [style.--op-color]="'var(' + operation.config.themeColor + ')'"
    >
      <!-- Icon Ring -->
      <div class="toast-icon-ring">
        <svg class="icon-svg" viewBox="0 0 24 24">
          <use [attr.href]="'#icon-' + operation.config.icon"></use>
        </svg>
        <svg class="progress-ring" viewBox="0 0 36 36" *ngIf="!hasConflict">
          <path class="ring-bg" d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"/>
          <path 
            class="ring-fill" 
            [attr.stroke-dasharray]="operation.percentComplete + ', 100'"
            d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
          />
        </svg>
        <!-- Warning icon for conflicts -->
        <svg class="conflict-icon" *ngIf="hasConflict" viewBox="0 0 24 24">
          <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none"/>
          <line x1="12" y1="8" x2="12" y2="12" stroke="currentColor" stroke-width="2"/>
          <circle cx="12" cy="16" r="1" fill="currentColor"/>
        </svg>
      </div>

      <!-- Content -->
      <div class="toast-content">
        <div class="toast-header">
          <span class="toast-title">{{ operation.config.title }}</span>
          <span class="toast-percent" *ngIf="!operation.isMinimized && !hasConflict">
            {{ operation.percentComplete | number:'1.0-0' }}%
          </span>
          <span class="conflict-badge" *ngIf="hasConflict">⚠️ Conflict</span>
        </div>

        <!-- Normal Progress View -->
        <ng-container *ngIf="!hasConflict">
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
        </ng-container>

        <!-- Conflict Resolution View -->
        <div class="conflict-resolution" *ngIf="hasConflict && !operation.isMinimized">
          <div class="conflict-files">
            <div class="conflict-file source">
              <div class="file-label">Source</div>
              <div class="file-path" [title]="sourcePath">
                {{ sourceFileName }}
              </div>
              <div class="file-meta" *ngIf="sourceSize">
                Size: {{ svc.formatBytes(sourceSize) }}
              </div>
            </div>
            
            <div class="conflict-arrow">→</div>
            
            <div class="conflict-file dest">
              <div class="file-label">Destination (exists)</div>
              <div class="file-path" [title]="destPath">
                {{ destFileName }}
              </div>
              <div class="file-meta" *ngIf="destSize">
                Size: {{ svc.formatBytes(destSize) }} · 
                <span *ngIf="isDirectory">Directory</span>
              </div>
            </div>
          </div>

          <div class="conflict-actions">
            <button class="conflict-btn skip" (click)="resolveConflict('skip')">
              <svg viewBox="0 0 24 24" width="14" height="14">
                <polyline points="9 18 15 12 9 6"></polyline>
              </svg>
              Skip
            </button>
            <button class="conflict-btn overwrite" (click)="resolveConflict('overwrite')">
              <svg viewBox="0 0 24 24" width="14" height="14">
                <path d="M3 12h14m-5-5l5 5-5 5"></path>
              </svg>
              Overwrite
            </button>
            <button class="conflict-btn rename" (click)="resolveConflict('rename')">
              <svg viewBox="0 0 24 24" width="14" height="14">
                <path d="M12 20h9M16.5 3.5l4 4L7 21H3v-4L16.5 3.5z"></path>
              </svg>
              Rename New
            </button>
            <button class="conflict-btn cancel" (click)="resolveConflict('cancel')">
              <svg viewBox="0 0 24 24" width="14" height="14">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
              Cancel
            </button>
          </div>

          <label class="apply-to-all">
            <input 
              type="checkbox" 
              [(ngModel)]="applyToAll"
              [disabled]="isResolving"
            >
            Apply to all conflicts
          </label>
        </div>

        <!-- Minimized conflict view -->
        <div class="conflict-mini" *ngIf="hasConflict && operation.isMinimized">
          <span class="conflict-mini-text">Conflict: {{ sourceFileName }}</span>
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
          (click)="onGlobalCollapse($event)"
          title="Collapse all to status bar"
        >
          <svg viewBox="0 0 24 24">
            <path d="M8 3v3a2 2 0 0 1-2 2H3m18 0h-3a2 2 0 0 1-2-2V3m0 18v-3a2 2 0 0 1 2-2h3M3 16h3a2 2 0 0 1 2 2v3"></path>
          </svg>
        </button>

        <button 
          class="action-btn cancel" 
          *ngIf="operation.status === 'running' && operation.config.cancellable && !hasConflict"
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

      <!-- Error Banner -->
      <div class="error-banner" *ngIf="operation.error && !hasConflict && !operation.isMinimized">
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
      width: 420px;
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

    .operation-toast.has-conflict::before {
      background: var(--vsc-warning);
    }

    .operation-toast.status-completed::before {
      background: var(--vsc-success);
    }
    .operation-toast.status-error::before {
      background: var(--vsc-error);
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
      z-index: 2;
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

    .conflict-icon {
      position: absolute;
      inset: 0;
      width: 100%;
      height: 100%;
      color: var(--vsc-warning);
      animation: pulse 2s infinite;
    }

    @keyframes pulse {
      0%, 100% { opacity: 1; transform: scale(1); }
      50% { opacity: 0.7; transform: scale(0.95); }
    }

    .toast-content {
      flex: 1;
      min-width: 0;
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

    .conflict-badge {
      font-size: var(--vsc-font-size-small);
      font-weight: 600;
      color: var(--vsc-warning);
      background: color-mix(in srgb, var(--vsc-warning) 15%, transparent);
      padding: 2px 8px;
      border-radius: 12px;
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

    /* Conflict Resolution Styles */
    .conflict-resolution {
      display: flex;
      flex-direction: column;
      gap: var(--vsc-padding-md);
      margin-top: var(--vsc-padding-sm);
      padding: var(--vsc-padding-md);
      background: color-mix(in srgb, var(--vsc-warning) 8%, transparent);
      border-radius: var(--vsc-border-radius);
      border: 1px solid color-mix(in srgb, var(--vsc-warning) 20%, transparent);
      animation: slideDown 0.2s ease;
    }

    @keyframes slideDown {
      from {
        opacity: 0;
        transform: translateY(-10px);
      }
      to {
        opacity: 1;
        transform: translateY(0);
      }
    }

    .conflict-files {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
    }

    .conflict-file {
      flex: 1;
      min-width: 0;
      padding: var(--vsc-padding-sm);
      background: var(--vsc-input-background);
      border-radius: var(--vsc-border-radius);
    }

    .conflict-file.source {
      border-left: 2px solid var(--vsc-accent-blue);
    }

    .conflict-file.dest {
      border-left: 2px solid var(--vsc-error);
    }

    .file-label {
      font-size: 10px;
      text-transform: uppercase;
      color: var(--vsc-foreground-dim);
      margin-bottom: 2px;
    }

    .conflict-arrow {
      font-size: 18px;
      color: var(--vsc-foreground-dim);
      opacity: 0.6;
    }

    .conflict-actions {
      display: grid;
      grid-template-columns: repeat(2, 1fr);
      gap: var(--vsc-padding-sm);
    }

    .conflict-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 4px;
      padding: var(--vsc-padding-sm) var(--vsc-padding-md);
      background: var(--vsc-button-secondary);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size-small);
      cursor: pointer;
      transition: all 0.15s;
      white-space: nowrap;
    }

    .conflict-btn:hover {
      background: var(--vsc-button-secondary-hover);
    }

    .conflict-btn.skip:hover {
      background: color-mix(in srgb, var(--vsc-foreground-dim) 20%, var(--vsc-button-secondary-hover));
    }

    .conflict-btn.overwrite:hover {
      background: color-mix(in srgb, var(--vsc-accent-blue) 20%, var(--vsc-button-secondary-hover));
      border-color: var(--vsc-accent-blue);
    }

    .conflict-btn.rename:hover {
      background: color-mix(in srgb, var(--vsc-accent-green) 20%, var(--vsc-button-secondary-hover));
      border-color: var(--vsc-accent-green);
    }

    .conflict-btn.cancel:hover {
      background: color-mix(in srgb, var(--vsc-error) 20%, var(--vsc-button-secondary-hover));
      border-color: var(--vsc-error);
    }

    .conflict-btn svg {
      stroke: currentColor;
      stroke-width: 2;
      fill: none;
    }

    .apply-to-all {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-dim);
      cursor: pointer;
    }

    .apply-to-all input {
      margin: 0;
      cursor: pointer;
    }

    .conflict-mini {
      margin-top: var(--vsc-padding-xs);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-warning);
    }

    .conflict-mini-text {
      background: color-mix(in srgb, var(--vsc-warning) 10%, transparent);
      padding: 2px 8px;
      border-radius: 12px;
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

    .action-btn.collapse:hover {
      background: color-mix(in srgb, var(--vsc-accent-purple) 20%, transparent);
      color: var(--vsc-accent-purple);
    }

    .action-btn.cancel:hover {
      background: color-mix(in srgb, var(--vsc-error) 20%, transparent);
      color: var(--vsc-error);
    }

    .action-btn.close:hover {
      background: var(--vsc-error);
      color: white;
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
      width: 260px;
      padding: var(--vsc-padding-md);
    }

    .operation-toast.minimized .toast-body,
    .operation-toast.minimized .conflict-resolution,
    .operation-toast.minimized .stats-grid,
    .operation-toast.minimized .progress-track {
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
export class OperationToastComponent implements OnInit {
  @Input() operation!: OperationState;
  @Output() minimize = new EventEmitter<string>();
  @Output() expand = new EventEmitter<string>();
  @Output() globalCollapse = new EventEmitter<void>();
  @Output() cancel = new EventEmitter<string>();
  @Output() dismiss = new EventEmitter<string>();
  @Output() resolve = new EventEmitter<{conflictId: string, resolution: ConflictResolution, applyToAll: boolean}>();

  applyToAll = false;
  isResolving = false;

  // Conflict data extracted from operation error
  sourcePath = '';
  sourceFileName = '';
  sourceSize = 0;
  destPath = '';
  destFileName = '';
  destSize = 0;
  isDirectory = false;
  conflictId = '';

  constructor(public svc: OperationService) {}

  ngOnInit(): void {
    this.parseConflictFromError();
  }

  private parseConflictFromError(): void {
    if (this.hasConflict && this.operation.error) {
      // Try to parse conflict info from error message
      // Format expected: "Conflict: [dest] already exists"
      const errorMsg = this.operation.error;
      
      // Extract source from currentFile
      if (this.operation.metrics.currentFile) {
        this.sourcePath = this.operation.metrics.currentFile;
        this.sourceFileName = this.getFileName(this.sourcePath);
      }
      
      // Try to extract destination from error
      const destMatch = errorMsg.match(/Conflict: (.+?) already exists/);
      if (destMatch) {
        this.destPath = destMatch[1];
        this.destFileName = this.getFileName(this.destPath);
      }
      
      // Check if it's a directory from the error or assume file
      this.isDirectory = errorMsg.includes('directory') || false;
    }
  }

  private getFileName(path: string): string {
    if (!path) return '';
    const parts = path.split(/[\\/]/);
    return parts[parts.length - 1] || path;
  }

  get hasConflict(): boolean {
    return this.operation.status === 'paused' && 
           !!this.operation.error && 
           this.operation.error.includes('Conflict');
  }

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

  onGlobalCollapse(event: Event): void {
    event.stopPropagation();
    this.globalCollapse.emit();
  }

  onCancel(event: Event): void {
    event.stopPropagation();
    this.cancel.emit(this.operation.id);
  }

  onDismiss(event: Event): void {
    event.stopPropagation();
    this.dismiss.emit(this.operation.id);
  }

  async resolveConflict(resolution: ConflictResolution): Promise<void> {
    if (this.isResolving) return;
    
    this.isResolving = true;
    console.log('Resolving conflict:', {
      resolution,
      applyToAll: this.applyToAll,
      conflictData: this.operation.conflictData,
      operationId: this.operation.id
    });
    
    // Use the actual conflictId from operation if available
    const conflictId = this.operation.conflictData?.conflictId || this.operation.id + '-conflict';
    console.log('Using conflictId:', conflictId);
    
    this.resolve.emit({
      conflictId,
      resolution,
      applyToAll: this.applyToAll
    });
    
    // Reset after a short delay
    setTimeout(() => {
      this.isResolving = false;
    }, 500);
  }
}