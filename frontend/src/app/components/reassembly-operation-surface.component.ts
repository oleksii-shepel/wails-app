import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  EventEmitter,
  OnDestroy,
  OnInit,
  Output,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

import {
  OperationService,
  type OperationState,
} from '../services/operation.service';
import type { ConflictResolution } from '../shared/protocol-enhanced';

@Component({
  selector: 'app-reassembly-operation-surface',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <aside
      class="surface"
      *ngIf="operations.length > 0"
      aria-label="File operations">

      <header>
        <div class="heading">
          <strong>Operations</strong>
          <span>{{ activeCount }} active</span>
          <span *ngIf="pausedCount">{{ pausedCount }} waiting</span>
          <span *ngIf="failedCount">{{ failedCount }} failed</span>
        </div>

        <div class="header-actions">
          <button type="button" (click)="openDetails.emit()">Details</button>
          <button
            type="button"
            *ngIf="finishedCount > 0"
            (click)="dismissFinished()">
            Clear finished
          </button>
        </div>
      </header>

      <article
        class="operation"
        *ngFor="let operation of operations; trackBy: trackOperation"
        [class.operation-failed]="operation.status === 'error'"
        [class.operation-paused]="operation.status === 'paused'">

        <div class="row">
          <span class="kind">{{ operation.config.title }}</span>
          <span
            class="status"
            [class.failed]="operation.status === 'error'"
            [class.paused]="operation.status === 'paused'">
            {{ statusLabel(operation) }}
          </span>
        </div>

        <div class="current" *ngIf="operation.metrics.currentFile">
          {{ leafName(operation.metrics.currentFile) }}
        </div>

        <div class="progress-track" *ngIf="operation.status !== 'error'">
          <div
            class="progress-value"
            [class.indeterminate]="operation.metrics.totalBytes <= 0 && isActive(operation)"
            [style.width.%]="operation.metrics.totalBytes > 0 ? operation.percentComplete : null">
          </div>
        </div>

        <div class="meta">
          <span *ngIf="operation.metrics.totalItems > 0">
            {{ operation.metrics.processedItems }}/{{ operation.metrics.totalItems }} items
          </span>

          <span *ngIf="operation.metrics.totalBytes > 0">
            {{ operationService.formatBytes(operation.metrics.processedBytes) }}
            /
            {{ operationService.formatBytes(operation.metrics.totalBytes) }}
          </span>

          <span *ngIf="operation.timing.speedBytesPerSecond > 0 && isActive(operation)">
            {{ operationService.formatSpeed(operation.timing.speedBytesPerSecond) }}
          </span>

          <span *ngIf="operation.timing.estimatedRemainingMs > 0 && isActive(operation)">
            {{ operationService.formatDuration(operation.timing.estimatedRemainingMs) }} left
          </span>

          <span *ngIf="operation.metrics.totalBytes > 0">
            {{ operation.percentComplete | number:'1.0-0' }}%
          </span>
        </div>

        <div class="error-message" *ngIf="operation.error && !operation.conflictData">
          {{ operation.error }}
        </div>

        <section class="conflict" *ngIf="operation.conflictData as conflict">
          <strong>Destination already exists</strong>
          <span class="conflict-path" [title]="conflict.destPath">
            {{ leafName(conflict.destPath) }}
          </span>

          <label class="apply-all">
            <input
              type="checkbox"
              [checked]="applyToAll.has(operation.id)"
              (change)="setApplyToAll(operation.id, $event)">
            Apply to remaining conflicts
          </label>

          <div class="conflict-actions">
            <button type="button" (click)="resolveConflict(operation, 'overwrite')">
              Replace
            </button>
            <button type="button" (click)="resolveConflict(operation, 'rename')">
              Keep both
            </button>
            <button type="button" (click)="resolveConflict(operation, 'skip')">
              Skip
            </button>
            <button type="button" class="danger" (click)="resolveConflict(operation, 'cancel')">
              Cancel operation
            </button>
          </div>
        </section>

        <div class="operation-actions" *ngIf="!operation.conflictData">
          <button
            type="button"
            *ngIf="canCancel(operation)"
            [disabled]="cancelling.has(operation.id)"
            (click)="cancel(operation)">
            {{ cancelling.has(operation.id) ? 'Cancelling…' : 'Cancel' }}
          </button>

          <button
            type="button"
            *ngIf="canDismiss(operation)"
            (click)="dismiss(operation)">
            Dismiss
          </button>
        </div>
      </article>
    </aside>
  `,
  styles: [`
    :host {
      position: fixed;
      left: 10px;
      bottom: 10px;
      z-index: 8500;
      pointer-events: none;
    }

    .surface {
      box-sizing: border-box;
      width: min(430px, calc(100vw - 20px));
      max-height: min(58vh, 500px);
      overflow: auto;
      padding: 8px;
      border: 1px solid var(--vsc-border, #454545);
      border-radius: 7px;
      background: color-mix(
        in srgb,
        var(--vsc-editor-background, #1e1e1e) 96%,
        transparent
      );
      color: var(--vsc-foreground, #cccccc);
      box-shadow: 0 8px 28px rgba(0, 0, 0, .35);
      backdrop-filter: blur(8px);
      font: 11px/1.35 var(--vsc-font-family, system-ui, sans-serif);
      pointer-events: auto;
    }

    header,
    .row,
    .meta,
    .heading,
    .header-actions,
    .operation-actions,
    .conflict-actions {
      display: flex;
      align-items: center;
      gap: 8px;
    }

    header {
      justify-content: space-between;
      padding: 2px 4px 7px;
      color: var(--vsc-description-foreground, #969696);
    }

    .heading {
      min-width: 0;
      flex-wrap: wrap;
    }

    header strong {
      color: var(--vsc-foreground, #cccccc);
    }

    button {
      min-height: 24px;
      padding: 2px 7px;
      border: 1px solid transparent;
      border-radius: 4px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      font: inherit;
    }

    button:hover:not(:disabled) {
      border-color: var(--vsc-border, #454545);
      background: var(--vsc-list-hover-background, #2a2d2e);
    }

    button:disabled {
      opacity: .45;
      cursor: default;
    }

    .operation {
      padding: 8px;
      border-top: 1px solid var(--vsc-border-subtle, #2b2b2b);
    }

    .operation-failed {
      border-left: 2px solid var(--vsc-error-foreground, #f48771);
    }

    .operation-paused {
      border-left: 2px solid var(--vsc-warning-foreground, #cca700);
    }

    .row {
      justify-content: space-between;
    }

    .kind {
      font-weight: 600;
    }

    .status {
      color: var(--vsc-accent-blue, #4aa3ff);
      text-transform: lowercase;
    }

    .status.failed {
      color: var(--vsc-error-foreground, #f48771);
    }

    .status.paused {
      color: var(--vsc-warning-foreground, #cca700);
    }

    .current,
    .conflict-path {
      margin-top: 4px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-description-foreground, #969696);
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
    }

    .progress-track {
      position: relative;
      height: 4px;
      margin-top: 7px;
      overflow: hidden;
      border-radius: 999px;
      background: color-mix(
        in srgb,
        var(--vsc-foreground, #cccccc) 12%,
        transparent
      );
    }

    .progress-value {
      height: 100%;
      min-width: 2px;
      border-radius: inherit;
      background: var(--vsc-accent-blue, #007acc);
      transition: width .12s linear;
    }

    .progress-value.indeterminate {
      width: 32%;
      animation: reassembly-operation-slide 1.1s ease-in-out infinite;
    }

    @keyframes reassembly-operation-slide {
      0% { transform: translateX(-120%); }
      100% { transform: translateX(420%); }
    }

    .meta {
      flex-wrap: wrap;
      margin-top: 6px;
      color: var(--vsc-description-foreground, #969696);
      font-size: 10px;
    }

    .meta span + span::before {
      content: "·";
      margin-right: 8px;
      opacity: .5;
    }

    .error-message {
      margin-top: 7px;
      padding: 6px 7px;
      border-radius: 4px;
      background: color-mix(in srgb, var(--vsc-error-foreground, #f48771) 9%, transparent);
      color: var(--vsc-error-foreground, #f48771);
    }

    .operation-actions {
      justify-content: flex-end;
      margin-top: 7px;
    }

    .conflict {
      display: grid;
      gap: 6px;
      margin-top: 8px;
      padding: 7px;
      border: 1px solid color-mix(in srgb, var(--vsc-warning-foreground, #cca700) 38%, transparent);
      border-radius: 5px;
      background: color-mix(in srgb, var(--vsc-warning-foreground, #cca700) 7%, transparent);
    }

    .conflict > strong {
      color: var(--vsc-warning-foreground, #cca700);
    }

    .apply-all {
      display: flex;
      align-items: center;
      gap: 5px;
      color: var(--vsc-description-foreground, #969696);
      font-size: 10px;
    }

    .conflict-actions {
      flex-wrap: wrap;
    }

    .danger {
      color: var(--vsc-error-foreground, #f48771);
    }
  `],
})
export class ReassemblyOperationSurfaceComponent implements OnInit, OnDestroy {
  @Output() openDetails = new EventEmitter<void>();

  operations: OperationState[] = [];
  readonly cancelling = new Set<string>();
  readonly applyToAll = new Set<string>();

  private readonly destroy$ = createSubject<void>();

  constructor(
    public readonly operationService: OperationService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get activeCount(): number {
    return this.operations.filter(operation => this.isActive(operation)).length;
  }

  get pausedCount(): number {
    return this.operations.filter(operation => operation.status === 'paused').length;
  }

  get failedCount(): number {
    return this.operations.filter(operation => operation.status === 'error').length;
  }

  get finishedCount(): number {
    return this.operations.filter(operation => this.canDismiss(operation)).length;
  }

  ngOnInit(): void {
    this.operationService.visibleOperations$
      .pipe(takeUntil(this.destroy$))
      .subscribe(operations => {
        this.operations = operations;

        for (const id of Array.from(this.cancelling)) {
          const operation = operations.find(item => item.id === id);
          if (!operation || operation.status === 'cancelled' || operation.status === 'error' || operation.status === 'completed') {
            this.cancelling.delete(id);
          }
        }

        this.cdr.markForCheck();
      });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
  }

  isActive(operation: OperationState): boolean {
    return operation.status === 'running' || operation.status === 'pending';
  }

  canCancel(operation: OperationState): boolean {
    return operation.config.cancellable &&
      (operation.status === 'running' || operation.status === 'pending' || operation.status === 'paused');
  }

  canDismiss(operation: OperationState): boolean {
    return operation.status === 'completed' ||
      operation.status === 'error' ||
      operation.status === 'cancelled';
  }

  statusLabel(operation: OperationState): string {
    if (operation.status === 'paused' && operation.conflictData) {
      return 'waiting for decision';
    }
    return operation.status;
  }

  async cancel(operation: OperationState): Promise<void> {
    if (!this.canCancel(operation) || this.cancelling.has(operation.id)) return;

    this.cancelling.add(operation.id);
    this.cdr.markForCheck();

    await this.operationService.cancel(operation.id);
  }

  dismiss(operation: OperationState): void {
    this.applyToAll.delete(operation.id);
    this.cancelling.delete(operation.id);
    this.operationService.dismiss(operation.id);
  }

  dismissFinished(): void {
    this.operationService.dismissFinished();
  }

  setApplyToAll(operationId: string, event: Event): void {
    const checked = (event.target as HTMLInputElement | null)?.checked ?? false;
    if (checked) {
      this.applyToAll.add(operationId);
    } else {
      this.applyToAll.delete(operationId);
    }
  }

  async resolveConflict(
    operation: OperationState,
    resolution: ConflictResolution,
  ): Promise<void> {
    const conflictId = operation.conflictData?.conflictId;
    if (!conflictId) return;

    const resolved = await this.operationService.resolveConflict(
      conflictId,
      resolution,
      this.applyToAll.has(operation.id),
    );

    if (resolved && (resolution === 'skip' || resolution === 'cancel')) {
      this.applyToAll.delete(operation.id);
    }
  }

  trackOperation(_index: number, operation: OperationState): string {
    return operation.id;
  }

  leafName(path: string): string {
    const normalized = (path ?? '').replace(/\\/g, '/').replace(/\/+$/, '');
    const index = normalized.lastIndexOf('/');
    return index >= 0 ? normalized.slice(index + 1) : normalized;
  }
}
