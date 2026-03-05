// operation-toast.component.ts
import { Component, Input, Output, EventEmitter, OnChanges, OnInit, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { OperationService, OperationState } from '../services/operation.service';
import { ConflictResolution } from '../shared/protocol-enhanced';
import { createSubject } from '@epikodelabs/streamix';

@Component({
  selector: 'app-operation-toast',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
  <div class="toast" [class.is-conflict]="hasConflict">

    <!-- ── NORMAL / PROGRESS ── -->
    <ng-container *ngIf="!hasConflict">

      <div class="ring-wrap">
        <svg viewBox="0 0 36 36" class="ring">
          <circle class="ring-bg" cx="18" cy="18" r="15.9"/>
          <circle class="ring-fg" cx="18" cy="18" r="15.9"
            [style.stroke-dasharray]="operation.percentComplete + ' 100'"
            [style.stroke-dashoffset]="0"/>
        </svg>
        <span class="ring-pct">{{operation.percentComplete | number:'1.0-0'}}</span>
      </div>

      <div class="body">
        <div class="op-header">
          <span class="op-title">{{operation.config.title}}</span>
          <button class="icon-btn" *ngIf="operation.status === 'running'" (click)="onCancel($event)" title="Cancel">
            <svg viewBox="0 0 16 16" fill="currentColor"><path d="M8 1a7 7 0 1 0 0 14A7 7 0 0 0 8 1zm3.5 9.1-1.4 1.4L8 9.4l-2.1 2.1-1.4-1.4L6.6 8 4.5 5.9l1.4-1.4L8 6.6l2.1-2.1 1.4 1.4L9.4 8l2.1 2.1z"/></svg>
          </button>
          <button class="icon-btn" *ngIf="isCompleted" (click)="onDismiss($event)" title="Dismiss">
            <svg viewBox="0 0 16 16" fill="currentColor"><path d="M9.41 8 12 5.41 10.59 4 8 6.59 5.41 4 4 5.41 6.59 8 4 10.59 5.41 12 8 9.41 10.59 12 12 10.59 9.41 8z"/></svg>
          </button>
        </div>

        <div class="file-name">{{currentFileName}}</div>

        <div class="progress-bar">
          <div class="progress-fill" [style.width.%]="operation.percentComplete"></div>
        </div>

        <div class="stats-row">
          <span>{{svc.formatSpeed(operation.timing.speedBytesPerSecond)}}</span>
          <span class="eta">{{svc.formatDuration(operation.timing.estimatedRemainingMs)}}</span>
          <span>{{svc.formatBytes(operation.metrics.processedBytes)}} / {{svc.formatBytes(operation.metrics.totalBytes)}}</span>
        </div>
      </div>

    </ng-container>

    <!-- ── CONFLICT ── -->
    <ng-container *ngIf="hasConflict">
      <div class="conflict-body">

        <div class="conflict-heading">
          <svg class="warn-icon" viewBox="0 0 16 16" fill="currentColor">
            <path d="M7.56 1h.88l6.54 12.26-.44.74H1.44L1 13.26Zm.44 2L2.28 13h11.44ZM8 11a.75.75 0 1 1 0 1.5A.75.75 0 0 1 8 11zm.75-5v4.5h-1.5V6h1.5z"/>
          </svg>
          <div>
            <div class="conflict-title">File already exists</div>
            <div class="conflict-sub">Choose how to handle this conflict</div>
          </div>
        </div>

        <div class="file-compare">
          <div class="file-side">
            <div class="side-label">Source</div>
            <div class="side-path">{{conflictFileName(operation.conflictData?.sourcePath)}}</div>
            <div class="side-dir">{{conflictDir(operation.conflictData?.sourcePath)}}</div>
            <div class="side-meta">{{svc.formatBytes(operation.conflictData?.sourceSize || 0)}}</div>
          </div>

          <div class="compare-arrow">
            <svg viewBox="0 0 16 16" fill="currentColor"><path d="M8.22 3.22a.75.75 0 0 1 1.06 0l4.25 4.25a.75.75 0 0 1 0 1.06l-4.25 4.25a.75.75 0 0 1-1.06-1.06L11.19 8.5H2.75a.75.75 0 0 1 0-1.5h8.44L8.22 4.28a.75.75 0 0 1 0-1.06z"/></svg>
          </div>

          <div class="file-side dest">
            <div class="side-label">Destination</div>
            <div class="side-path">{{conflictFileName(operation.conflictData?.destPath)}}</div>
            <div class="side-dir">{{conflictDir(operation.conflictData?.destPath)}}</div>
            <div class="side-meta">
              {{svc.formatBytes(operation.conflictData?.destSize || 0)}}
              <span *ngIf="operation.conflictData?.isDirectory" class="folder-badge">folder</span>
            </div>
          </div>
        </div>

        <div class="resolution-btns">
          <button class="res-btn" (click)="resolveConflict('skip')">Skip</button>
          <button class="res-btn primary" (click)="resolveConflict('overwrite')">Replace</button>
          <button class="res-btn" *ngIf="canKeepBoth" (click)="resolveConflict('rename')">Keep Both</button>
          <button class="res-btn danger" (click)="resolveConflict('cancel')">Cancel All</button>
        </div>

        <label class="apply-all">
          <input type="checkbox" [(ngModel)]="applyToAll">
          <span>Apply to all conflicts</span>
        </label>

      </div>
    </ng-container>

  </div>
  `,
  styles: [`
    :host {
      display: block;
      animation: slide-in .18s cubic-bezier(.16,1,.3,1);
    }

    @keyframes slide-in {
      from { opacity: 0; transform: translateY(8px) scale(.98); }
      to   { opacity: 1; transform: translateY(0) scale(1); }
    }

    /* ── Shell ── */
    .toast {
      width: 360px;
      padding: 12px 14px;
      background: var(--vsc-panel-background, #1e1e1e);
      border: 1px solid var(--vsc-border, #3c3c3c);
      border-radius: 6px;
      box-shadow: 0 8px 24px rgba(0,0,0,.45);
      color: var(--vsc-foreground, #ccc);
      font-size: 12px;
      display: flex;
      gap: 12px;
      transition: border-color .15s;
    }
    
    .toast.is-conflict { border-color: #e8a94d; padding: 0; }

    /* ── Ring ── */
    .ring-wrap {
      position: relative;
      width: 36px;
      height: 36px;
      flex-shrink: 0;
    }

    .ring {
      width: 36px;
      height: 36px;
      transform: rotate(-90deg);
    }

    .ring-bg {
      fill: none;
      stroke: var(--vsc-border, #3c3c3c);
      stroke-width: 2.5;
    }

    .ring-fg {
      fill: none;
      stroke: var(--vsc-accent-blue, #007fd4);
      stroke-width: 2.5;
      stroke-linecap: round;
      stroke-dashoffset: 0;
      /* circumference ≈ 100 in viewBox units when r=15.9 */
      transition: stroke-dasharray .25s ease;
    }

    .ring-pct {
      position: absolute;
      inset: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      font-size: 9px;
      font-family: monospace;
      font-weight: 600;
    }

    /* ── Body ── */
    .body { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 4px; }

    .op-header {
      display: flex;
      align-items: center;
      gap: 6px;
    }

    .op-title {
      flex: 1;
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .icon-btn {
      flex-shrink: 0;
      width: 18px;
      height: 18px;
      padding: 0;
      border: none;
      background: none;
      color: var(--vsc-foreground, #ccc);
      opacity: 0;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;
      border-radius: 3px;
      transition: opacity .15s, background .15s;
    }

    .toast:hover .icon-btn { opacity: .6; }
    .icon-btn:hover { opacity: 1 !important; background: var(--vsc-toolbar-hover, rgba(255,255,255,.08)); }
    .icon-btn svg { width: 14px; height: 14px; }

    .file-name {
      font-family: monospace;
      font-size: 11px;
      opacity: .65;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .progress-bar {
      height: 2px;
      background: var(--vsc-border, #3c3c3c);
      border-radius: 1px;
      overflow: hidden;
    }

    .progress-fill {
      height: 100%;
      background: var(--vsc-accent-blue, #007fd4);
      transition: width .2s linear;
    }

    .stats-row {
      display: flex;
      gap: 10px;
      font-family: monospace;
      font-size: 11px;
      opacity: .55;
    }

    .eta { flex: 1; }

    /* ── Conflict body ── */
    .conflict-body {
      width: 100%;
      display: flex;
      flex-direction: column;
      gap: 0;
    }

    .conflict-heading {
      display: flex;
      align-items: flex-start;
      gap: 10px;
      padding: 12px 14px 10px;
      border-bottom: 1px solid rgba(232,169,77,.2);
    }

    .warn-icon {
      width: 16px;
      height: 16px;
      flex-shrink: 0;
      color: #e8a94d;
      margin-top: 1px;
    }

    .conflict-title {
      font-weight: 600;
      font-size: 12px;
      color: #e8a94d;
      line-height: 1.3;
    }

    .conflict-sub {
      font-size: 11px;
      opacity: .6;
      margin-top: 1px;
    }

    /* ── File compare ── */
    .file-compare {
      display: grid;
      grid-template-columns: 1fr 24px 1fr;
      align-items: center;
      gap: 0;
      padding: 10px 14px;
      border-bottom: 1px solid var(--vsc-border, #3c3c3c);
    }

    .file-side {
      display: flex;
      flex-direction: column;
      gap: 2px;
      min-width: 0;
    }

    .file-side.dest .side-path,
    .file-side.dest .side-dir { color: #e8a94d; opacity: .9; }

    .side-label {
      font-size: 10px;
      text-transform: uppercase;
      letter-spacing: .06em;
      opacity: .45;
      font-weight: 600;
    }

    .side-path {
      font-family: monospace;
      font-size: 11px;
      font-weight: 600;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .side-dir {
      font-family: monospace;
      font-size: 10px;
      opacity: .45;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .side-meta {
      font-size: 11px;
      opacity: .6;
      margin-top: 2px;
      display: flex;
      align-items: center;
      gap: 5px;
    }

    .folder-badge {
      font-size: 10px;
      background: rgba(255,255,255,.08);
      border-radius: 3px;
      padding: 0 4px;
    }

    .compare-arrow {
      display: flex;
      align-items: center;
      justify-content: center;
      opacity: .35;
    }

    .compare-arrow svg { width: 14px; height: 14px; }

    /* ── Resolution buttons ── */
    .resolution-btns {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(0, 1fr));
      gap: 6px;
      padding: 10px 14px 8px;
    }

    .res-btn {
      padding: 5px 0;
      font-size: 11px;
      font-weight: 500;
      border-radius: 4px;
      border: 1px solid var(--vsc-border, #3c3c3c);
      background: var(--vsc-button-secondary, rgba(255,255,255,.05));
      color: var(--vsc-foreground, #ccc);
      cursor: pointer;
      transition: background .12s, border-color .12s;
      white-space: nowrap;
    }

    .res-btn:hover {
      background: var(--vsc-button-secondary-hover, rgba(255,255,255,.1));
      border-color: var(--vsc-focusBorder, #007fd4);
    }

    .res-btn.primary {
      background: var(--vsc-button-background, #0e639c);
      border-color: transparent;
      color: #fff;
    }

    .res-btn.primary:hover { background: var(--vsc-button-hover, #1177bb); }

    .res-btn.danger {
      color: #f48771;
      border-color: rgba(244,135,113,.3);
    }

    .res-btn.danger:hover {
      background: rgba(244,135,113,.1);
      border-color: #f48771;
    }

    /* ── Apply to all ── */
    .apply-all {
      display: flex;
      align-items: center;
      gap: 7px;
      padding: 0 14px 11px;
      font-size: 11px;
      opacity: .65;
      cursor: pointer;
      user-select: none;
    }

    .apply-all:hover { opacity: 1; }
    .apply-all input { cursor: pointer; accent-color: var(--vsc-accent-blue, #007fd4); }
  `]
})
export class OperationToastComponent implements OnChanges, OnInit, OnDestroy {
  @Input() operation!: OperationState;

  @Output() cancel  = new EventEmitter<string>();
  @Output() dismiss = new EventEmitter<string>();
  @Output() resolve = new EventEmitter<{ conflictId: string; resolution: ConflictResolution; applyToAll: boolean }>();

  applyToAll  = false;
  isResolving = false;

  private destroy$ = createSubject<void>();
  private timer: any;

  constructor(public svc: OperationService) {}

  ngOnInit()    { this.setupAutoDismiss(); }
  ngOnChanges() { this.setupAutoDismiss(); }

  ngOnDestroy() {
    this.clearTimer();
    this.destroy$.next();
    this.destroy$.complete();
  }

  private setupAutoDismiss() {
    this.clearTimer();
    if (this.isCompleted && !this.hasConflict) {
      this.timer = setTimeout(() => this.dismiss.emit(this.operation.id), 800);
    }
  }

  private clearTimer() {
    if (this.timer) { clearTimeout(this.timer); this.timer = null; }
  }

  get hasConflict() {
    return this.operation.status === 'paused' && !!this.operation.conflictData;
  }

  get isCompleted() {
    return ['completed', 'error', 'cancelled'].includes(this.operation.status);
  }

  get canKeepBoth() {
    if (this.operation?.config?.kind !== 'move') return true;

    const src = this.operation.conflictData?.sourcePath;
    const dst = this.operation.conflictData?.destPath;
    if (!src || !dst) return true; // fallback: allow if uncertain

    const srcDir = this.parentDir(src);
    const dstDir = this.parentDir(dst);
    return srcDir !== dstDir; // allow keep-both when moving into a different folder
  }

  get currentFileName() {
    const p = this.operation.metrics.currentFile || '';
    return p.split(/[\\/]/).pop() || p;
  }

  conflictFileName(path?: string): string {
    if (!path) return '';
    return path.split(/[\\/]/).pop() || path;
  }

  conflictDir(path?: string): string {
    if (!path) return '';
    const parts = path.split(/[\\/]/);
    return parts.slice(0, -1).join('/') || '/';
  }

  private parentDir(path: string): string {
    const parts = path.split(/[\\/]/);
    parts.pop();
    return parts.join('/') || '/';
  }

  onCancel(e: Event)  { e.stopPropagation(); this.cancel.emit(this.operation.id); }
  onDismiss(e: Event) { e.stopPropagation(); this.clearTimer(); this.dismiss.emit(this.operation.id); }

  async resolveConflict(resolution: ConflictResolution) {
    if (this.isResolving) return;
    if (resolution === 'rename' && !this.canKeepBoth) return;
    this.isResolving = true;
    this.resolve.emit({
      conflictId: this.operation.conflictData?.conflictId || this.operation.id,
      resolution,
      applyToAll: this.applyToAll,
    });
    setTimeout(() => this.isResolving = false, 400);
  }
}