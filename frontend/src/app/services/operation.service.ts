import { Injectable, NgZone, OnDestroy } from '@angular/core';
import { 
  createSubject, 
  createBehaviorSubject,
  map,
  takeUntil,
  distinctUntilChanged,
  shareReplay
} from '@epikodelabs/streamix';
import { EventsOn, EventsOff } from 'wailsjs/runtime';
import {
  OperationKind,
  OperationStatus,
  OpStartEvent,
  OpProgressEvent,
  OpCompleteEvent,
  ConflictEvent
} from '../shared/protocol-enhanced';
import { OperationRegistryService } from './operation-registry.service';

// Define OperationState locally since it's a UI-specific extension of protocol types
export interface OperationState {
  id: string;
  config: {
    icon: string;
    title: string;
    themeColor: string;
    cancellable: boolean;
  };
  status: OperationStatus;
  error?: string;
  metrics: {
    totalBytes: number;
    processedBytes: number;
    totalItems: number;
    processedItems: number;
    currentFile?: string;
    currentFileSize?: number;
  };
  percentComplete: number;
  timing: {
    startedAt: number;
    elapsedMs: number;
    estimatedRemainingMs: number;
    speedBytesPerSecond: number;
  };
  isMinimized: boolean;
  isCollapsed: boolean;
  stackIndex: number;
}

@Injectable({ providedIn: 'root' })
export class OperationService implements OnDestroy {
  // Core state streams
  private operationsMap = new Map<string, OperationState>();
  readonly operations$ = createBehaviorSubject<Map<string, OperationState>>(new Map());
  
  // Derived streams using Streamix
  public activeCount$ = this.operations$.pipe(
    map(ops => {
      let count = 0;
      ops.forEach(op => {
        if (op.status === 'running' || op.status === 'pending') count++;
      });
      return count;
    }),
    distinctUntilChanged(),
    shareReplay(1)
  );

  public visibleOperations$ = this.operations$.pipe(
    map(ops => {
      const visible: OperationState[] = [];
      ops.forEach(op => {
        if (!op.isCollapsed) visible.push(op);
      });
      return visible.sort((a, b) => a.stackIndex - b.stackIndex).slice(0, 3);
    }),
    shareReplay(1)
  );

  public collapsedOperations$ = this.operations$.pipe(
    map(ops => {
      const collapsed: OperationState[] = [];
      ops.forEach(op => {
        if (op.isCollapsed) collapsed.push(op);
      });
      return collapsed;
    }),
    shareReplay(1)
  );

  public hasActiveOperations$ = this.activeCount$.pipe(
    map(count => count > 0),
    distinctUntilChanged(),
    shareReplay(1)
  );

  // Cleanup
  private destroySubject = createSubject<void>();
  private autoCloseTimers = new Map<string, ReturnType<typeof setTimeout>>();
  private readonly AUTO_CLOSE_DELAY = 5000;

  constructor(
    private zone: NgZone,
    private registry: OperationRegistryService
  ) {
    this.setupWailsEvents();
  }

  private setupWailsEvents(): void {
    const handleStart = (event: OpStartEvent) => {
      this.zone.run(() => this.handleStart(event));
    };

    const handleProgress = (event: OpProgressEvent) => {
      this.zone.run(() => this.handleProgress(event));
    };

    const handleComplete = (event: OpCompleteEvent) => {
      this.zone.run(() => this.handleComplete(event));
    };

    // Handle conflicts if needed
    const handleConflict = (event: ConflictEvent) => {
      this.zone.run(() => this.handleConflict(event));
    };

    EventsOn('operation:start', handleStart);
    EventsOn('operation:progress', handleProgress);
    EventsOn('operation:complete', handleComplete);
    EventsOn('operation:conflict', handleConflict);

    (this as any)._eventHandlers = { 
      handleStart, 
      handleProgress, 
      handleComplete,
      handleConflict 
    };
  }

  private handleStart(event: OpStartEvent): void {
    const config = this.registry.get(event.kind as OperationKind);
    const now = Date.now();
    
    const state: OperationState = {
      id: event.id,
      config,
      status: 'running',
      metrics: {
        totalBytes: event.totalBytes,
        processedBytes: 0,
        totalItems: event.totalItems,
        processedItems: 0,
        currentFile: event.sources[0]
      },
      percentComplete: 0,
      timing: {
        startedAt: now,
        elapsedMs: 0,
        estimatedRemainingMs: 0,
        speedBytesPerSecond: 0
      },
      isMinimized: false,
      isCollapsed: false,
      stackIndex: this.operationsMap.size
    };

    this.operationsMap.set(event.id, state);
    this.emitOperations();
  }

  private handleProgress(event: OpProgressEvent): void {
    const existing = this.operationsMap.get(event.id);
    if (!existing) return;

    const totalBytes = existing.metrics.totalBytes;
    const processedBytes = event.processedBytes;
    
    const percentComplete = totalBytes > 0 
      ? Math.min(100, (processedBytes / totalBytes) * 100)
      : 0;

    const updated: OperationState = {
      ...existing,
      metrics: {
        ...existing.metrics,
        processedBytes: event.processedBytes,
        processedItems: event.processedItems,
        currentFile: event.currentFile,
        currentFileSize: event.currentFileSize
      },
      percentComplete,
      timing: {
        startedAt: existing.timing.startedAt,
        elapsedMs: event.elapsedMs,
        estimatedRemainingMs: event.remainingMs,
        speedBytesPerSecond: event.speedBps
      }
    };

    this.operationsMap.set(event.id, updated);
    this.emitOperations();
  }

  private handleComplete(event: OpCompleteEvent): void {
    const existing = this.operationsMap.get(event.id);
    if (!existing) return;

    const updated: OperationState = {
      ...existing,
      status: event.status as OperationStatus,
      metrics: {
        ...existing.metrics,
        processedBytes: event.finalMetrics.processedBytes,
        processedItems: event.finalMetrics.processedItems
      },
      percentComplete: 100,
      timing: event.timing,
      error: event.error
    };

    this.operationsMap.set(event.id, updated);
    this.emitOperations();

    if (event.status === 'completed' || event.status === 'cancelled') {
      this.scheduleAutoClose(event.id);
    }
  }

  private handleConflict(event: ConflictEvent): void {
    // Pause the operation in UI
    const existing = this.operationsMap.get(event.operationId);
    if (existing) {
      const updated: OperationState = {
        ...existing,
        status: 'paused'
      };
      this.operationsMap.set(event.operationId, updated);
      this.emitOperations();
    }
  }

  private emitOperations(): void {
    this.operations$.next(new Map(this.operationsMap));
  }

  private scheduleAutoClose(id: string): void {
    if (this.autoCloseTimers.has(id)) {
      clearTimeout(this.autoCloseTimers.get(id));
    }

    const timer = setTimeout(() => {
      this.dismiss(id);
    }, this.AUTO_CLOSE_DELAY);

    this.autoCloseTimers.set(id, timer);
  }

  // Public API methods

  minimize(id: string): void {
    this.patchOperation(id, { isMinimized: true });
  }

  expand(id: string): void {
    this.patchOperation(id, { isMinimized: false });
  }

  collapse(id: string): void {
    this.patchOperation(id, { isCollapsed: true, isMinimized: false });
    this.reindexStack();
  }

  restore(id: string): void {
    this.patchOperation(id, { isCollapsed: false, isMinimized: false });
    this.reindexStack();
  }

  async cancel(id: string): Promise<void> {
    try {
      await (window as any).go?.main?.App?.CancelOperation(id);
    } catch (e) {
      console.error('Failed to cancel operation:', e);
    }
  }

  dismiss(id: string): void {
    if (this.autoCloseTimers.has(id)) {
      clearTimeout(this.autoCloseTimers.get(id));
      this.autoCloseTimers.delete(id);
    }

    this.operationsMap.delete(id);
    this.emitOperations();
    this.reindexStack();
  }

  collapseAll(): void {
    this.operationsMap.forEach(op => {
      op.isCollapsed = true;
      op.isMinimized = false;
    });
    this.emitOperations();
  }

  restoreAll(): void {
    this.operationsMap.forEach(op => {
      op.isCollapsed = false;
      op.isMinimized = false;
    });
    this.emitOperations();
    this.reindexStack();
  }

  private patchOperation(id: string, patch: Partial<OperationState>): void {
    const current = this.operationsMap.get(id);
    if (!current) return;

    this.operationsMap.set(id, { ...current, ...patch });
    this.emitOperations();
  }

  private reindexStack(): void {
    let index = 0;
    const sorted = Array.from(this.operationsMap.values())
      .filter(o => !o.isCollapsed)
      .sort((a, b) => a.stackIndex - b.stackIndex);

    sorted.forEach(op => {
      const updated = { ...op, stackIndex: index++ };
      this.operationsMap.set(op.id, updated);
    });

    this.emitOperations();
  }

  // Formatters

  formatBytes(bytes: number): string {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  }

  formatSpeed(bps: number): string {
    return this.formatBytes(bps) + '/s';
  }

  formatDuration(ms: number): string {
    if (ms < 0 || !isFinite(ms)) return '--:--';
    const seconds = Math.floor(ms / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    
    if (hours > 0) {
      return `${hours}:${(minutes % 60).toString().padStart(2, '0')}:${(seconds % 60).toString().padStart(2, '0')}`;
    }
    return `${minutes}:${(seconds % 60).toString().padStart(2, '0')}`;
  }

  ngOnDestroy(): void {
    this.destroySubject.next();
    this.destroySubject.complete();
    
    EventsOff('operation:start');
    EventsOff('operation:progress');
    EventsOff('operation:complete');
    EventsOff('operation:conflict');
    
    this.autoCloseTimers.forEach(clearTimeout);
  }
}