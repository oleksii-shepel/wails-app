import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  OnDestroy,
  OnInit,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

import {
  OperationService,
  type OperationState,
} from '../services/operation.service';

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
        <strong>Operations</strong>
        <span>{{ activeCount }} active</span>
      </header>

      <article
        class="operation"
        *ngFor="let operation of operations; trackBy: trackOperation">

        <div class="row">
          <span class="kind">{{ operation.config.label }}</span>
          <span class="status" [class.failed]="operation.status === 'failed'">
            {{ operation.status }}
          </span>
        </div>

        <div class="current" *ngIf="operation.metrics.currentFile">
          {{ leafName(operation.metrics.currentFile) }}
        </div>

        <div class="progress-track">
          <div
            class="progress-value"
            [class.indeterminate]="operation.metrics.totalBytes <= 0"
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

          <span *ngIf="operation.timing.speedBytesPerSecond > 0">
            {{ operationService.formatSpeed(operation.timing.speedBytesPerSecond) }}
          </span>

          <span *ngIf="operation.metrics.totalBytes > 0">
            {{ operation.percentComplete | number:'1.0-0' }}%
          </span>
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
      width: min(390px, calc(100vw - 20px));
      max-height: min(52vh, 420px);
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
    }

    header,
    .row,
    .meta {
      display: flex;
      align-items: center;
      gap: 8px;
    }

    header {
      justify-content: space-between;
      padding: 2px 4px 7px;
      color: var(--vsc-description-foreground, #969696);
    }

    header strong {
      color: var(--vsc-foreground, #cccccc);
    }

    .operation {
      padding: 8px;
      border-top: 1px solid var(--vsc-border-subtle, #2b2b2b);
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

    .current {
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
  `],
})
export class ReassemblyOperationSurfaceComponent implements OnInit, OnDestroy {
  operations: OperationState[] = [];

  private readonly destroy$ = createSubject<void>();

  constructor(
    public readonly operationService: OperationService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get activeCount(): number {
    return this.operations.filter(operation =>
      operation.status === 'running' || operation.status === 'pending'
    ).length;
  }

  ngOnInit(): void {
    this.operationService.visibleOperations$
      .pipe(takeUntil(this.destroy$))
      .subscribe(operations => {
        this.operations = operations;
        this.cdr.markForCheck();
      });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
    this.destroy$.complete();
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
