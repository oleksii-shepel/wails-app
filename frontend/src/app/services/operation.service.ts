// operation.service.ts - Add resolveConflict method

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
import { ResolveConflict } from 'wailsjs/go/app/App';
import {
  OperationKind,
  OperationStatus,
  OpStartEvent,
  OpProgressEvent,
  OpCompleteEvent,
  ConflictEvent,
  ConflictResolution
} from '../shared/protocol-enhanced';
import { OperationRegistryService, OperationConfig } from './operation-registry.service';

export interface OperationState {
  id: string;
  config: OperationConfig;
  status: OperationStatus;
  error?: string;
  // Add conflict data
  conflictData?: {
    conflictId: string;
    sourcePath: string;
    destPath: string;
    sourceSize: number;
    destSize: number;
    isDirectory: boolean;
  };
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
  readonly operations$ = createBehaviorSubject<Map<string, OperationState>>(new Map<string, OperationState>());
  
  // Add conflicts stream
  private conflictsSubject = createSubject<ConflictEvent[]>();
  public conflicts$ = this.conflictsSubject.pipe(shareReplay(1));
  
  private pendingConflicts = new Map<string, ConflictEvent>();
  
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

  // Flag to track if events are set up
  private eventsSetup = false;

  constructor(
    private zone: NgZone,
    private registry: OperationRegistryService
  ) {
    this.setupWailsEvents();
  }

  private setupWailsEvents(): void {
    if (this.eventsSetup) return;
    
    const handleStart = (event: OpStartEvent) => {
      this.zone.run(() => this.handleStart(event));
    };

    const handleProgress = (event: OpProgressEvent) => {
      this.zone.run(() => this.handleProgress(event));
    };

    const handleComplete = (event: OpCompleteEvent) => {
      this.zone.run(() => this.handleComplete(event));
    };

    const handleConflict = (event: ConflictEvent) => {
      this.zone.run(() => this.handleConflict(event));
    };

    // Register event listeners
    EventsOn('operation:start', handleStart);
    EventsOn('operation:progress', handleProgress);
    EventsOn('operation:complete', handleComplete);
    EventsOn('operation:conflict', handleConflict);

    this.eventsSetup = true;

    // Store handlers for cleanup
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
    // Store the conflict
    this.pendingConflicts.set(event.conflictId, event);
    this.conflictsSubject.next(Array.from(this.pendingConflicts.values()));

    // Update the operation state
    const existing = this.operationsMap.get(event.operationId);
    if (existing) {
      const updated: OperationState = {
        ...existing,
        status: 'paused',
        error: `Conflict: ${event.destPath} already exists`,
        conflictData: {
          conflictId: event.conflictId,
          sourcePath: event.sourcePath,
          destPath: event.destPath,
          sourceSize: event.sourceSize,
          destSize: event.destSize,
          isDirectory: event.isDirectory
        },
        metrics: {
          ...existing.metrics,
          currentFile: event.sourcePath
        }
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

  // ============================================================================
  // Conflict Resolution
  // ============================================================================

  /**
   * Resolve a conflict by sending the user's decision to the backend
   */
  async resolveConflict(conflictId: string, resolution: ConflictResolution, applyToAll: boolean): Promise<boolean> {
    try {
      const result = await ResolveConflict(conflictId, resolution as any, applyToAll);
      
      if (result) {
        // Remove from pending conflicts
        const conflict = this.pendingConflicts.get(conflictId);
        this.pendingConflicts.delete(conflictId);
        this.conflictsSubject.next(Array.from(this.pendingConflicts.values()));

        // Resume or hide the operation depending on the user's choice
        if (conflict && !this.hasPendingConflictsForOperation(conflict.operationId)) {
          const op = this.operationsMap.get(conflict.operationId);
          if (op) {
            op.error = undefined;
            op.conflictData = undefined;

            // Skip/Cancel: immediately dismiss the toast so no moving dialog is shown again
            if (resolution === 'skip' || resolution === 'cancel') {
              this.dismiss(conflict.operationId);
            } else if (op.status === 'paused') {
              // For overwrite/rename, let it continue
              op.status = 'running';
              this.emitOperations();
            }
          }
        }
      }
      
      return result;
    } catch (error) {
      console.error('Failed to resolve conflict:', error);
      return false;
    }
  }

  /**
   * Check if an operation has any pending conflicts
   */
  private hasPendingConflictsForOperation(operationId: string): boolean {
    for (const conflict of this.pendingConflicts.values()) {
      if (conflict.operationId === operationId) {
        return true;
      }
    }
    return false;
  }

  /**
   * Get all pending conflicts
   */
  getPendingConflicts(): ConflictEvent[] {
    return Array.from(this.pendingConflicts.values());
  }

  async waitForOperationSettlement(
    operationId: string,
    appearanceTimeoutMs: number = 250,
    overallTimeoutMs: number = 300000,
  ): Promise<OperationStatus | 'dismissed' | 'missing'> {
    return await new Promise<OperationStatus | 'dismissed' | 'missing'>((resolve) => {
      let settled = false;
      let seenOperation = false;
      let appearanceTimer: ReturnType<typeof setTimeout> | null = null;
      let overallTimer: ReturnType<typeof setTimeout> | null = null;
      let subscription: { unsubscribe(): void } | null = null;

      const finish = (status: OperationStatus | 'dismissed' | 'missing') => {
        if (settled) return;
        settled = true;
        if (appearanceTimer) clearTimeout(appearanceTimer);
        if (overallTimer) clearTimeout(overallTimer);
        try { subscription?.unsubscribe(); } catch {}
        resolve(status);
      };

      appearanceTimer = setTimeout(() => {
        const op = this.operationsMap.get(operationId);
        if (!seenOperation && !op && !this.hasPendingConflictsForOperation(operationId)) {
          finish('missing');
        }
      }, appearanceTimeoutMs);

      overallTimer = setTimeout(() => {
        finish(seenOperation ? 'dismissed' : 'missing');
      }, overallTimeoutMs);

      subscription = this.operations$.subscribe((ops) => {
        const op = ops.get(operationId);
        if (op) {
          seenOperation = true;
          if (op.status === 'completed' || op.status === 'error' || op.status === 'cancelled') {
            finish(op.status);
          }
          return;
        }

        if (seenOperation && !this.hasPendingConflictsForOperation(operationId)) {
          finish('dismissed');
        }
      });
    });
  }

  /**
   * Get a specific pending conflict by ID
   */
  getConflict(conflictId: string): ConflictEvent | undefined {
    return this.pendingConflicts.get(conflictId);
  }

  // ============================================================================
  // Public API Methods
  // ============================================================================

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
      const result = await (window as any).go?.app?.App?.CancelOperation(id);
      if (!result) {
        console.warn('Cancel operation returned false');
      }
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

  // ============================================================================
  // Formatters
  // ============================================================================

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

  // ============================================================================
  // Cleanup
  // ============================================================================

  ngOnDestroy(): void {
    this.destroySubject.next();
    this.destroySubject.complete();
    
    // Clean up event listeners
    EventsOff('operation:start');
    EventsOff('operation:progress');
    EventsOff('operation:complete');
    EventsOff('operation:conflict');
    
    this.autoCloseTimers.forEach(clearTimeout);
  }
}