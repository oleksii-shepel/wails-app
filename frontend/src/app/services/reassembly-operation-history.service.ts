import { Injectable } from '@angular/core';


export type ReassemblySyncMode = 'left-to-right' | 'right-to-left' | 'bidirectional' | 'custom';

export interface ReassemblySyncHistoryEntry {
  readonly id: string;
  readonly startedAt: number;
  readonly completedAt: number;
  readonly leftRoot: string;
  readonly rightRoot: string;
  readonly mode: ReassemblySyncMode;
  readonly recursive: boolean;
  readonly planned: number;
  readonly succeeded: number;
  readonly skipped: number;
  readonly failed: number;
  readonly conflicts: number;
}

const SYNC_HISTORY_KEY = 'reassembly.syncHistory.v1';
const SYNC_HISTORY_LIMIT = 100;

export interface ReassemblyReversibleAction {
  readonly label: string;
  readonly undo: () => Promise<void>;
  readonly redo: () => Promise<void>;
}

@Injectable({ providedIn: 'root' })
export class ReassemblyOperationHistoryService {
  private readonly undoStack: ReassemblyReversibleAction[] = [];
  private readonly redoStack: ReassemblyReversibleAction[] = [];
  private running = false;
  private readonly limit = 100;
  private syncHistory: ReassemblySyncHistoryEntry[] | null = null;

  get canUndo(): boolean {
    return !this.running && this.undoStack.length > 0;
  }

  get canRedo(): boolean {
    return !this.running && this.redoStack.length > 0;
  }

  record(action: ReassemblyReversibleAction): void {
    if (this.running) return;
    this.undoStack.push(action);
    if (this.undoStack.length > this.limit) {
      this.undoStack.splice(0, this.undoStack.length - this.limit);
    }
    this.redoStack.length = 0;
  }

  async undo(): Promise<string | null> {
    if (!this.canUndo) return null;
    const action = this.undoStack.pop();
    if (!action) return null;

    this.running = true;
    try {
      await action.undo();
      this.redoStack.push(action);
      return action.label;
    } catch (error) {
      this.undoStack.push(action);
      throw error;
    } finally {
      this.running = false;
    }
  }

  async redo(): Promise<string | null> {
    if (!this.canRedo) return null;
    const action = this.redoStack.pop();
    if (!action) return null;

    this.running = true;
    try {
      await action.redo();
      this.undoStack.push(action);
      return action.label;
    } catch (error) {
      this.redoStack.push(action);
      throw error;
    } finally {
      this.running = false;
    }
  }

  clear(): void {
    if (this.running) return;
    this.undoStack.length = 0;
    this.redoStack.length = 0;
  }

  recordSync(entry: Omit<ReassemblySyncHistoryEntry, 'id'> & { readonly id?: string }): ReassemblySyncHistoryEntry {
    const recorded: ReassemblySyncHistoryEntry = {
      ...entry,
      id: entry.id ?? globalThis.crypto?.randomUUID?.() ?? `${entry.completedAt}-${Math.random().toString(16).slice(2)}`,
    };
    const history = this.loadSyncHistory();
    history.unshift(recorded);
    if (history.length > SYNC_HISTORY_LIMIT) history.splice(SYNC_HISTORY_LIMIT);
    this.persistSyncHistory(history);
    return recorded;
  }

  recentSyncs(limit = 20): readonly ReassemblySyncHistoryEntry[] {
    return this.loadSyncHistory().slice(0, Math.max(0, limit)).map(entry => ({ ...entry }));
  }

  clearSyncHistory(): void {
    this.syncHistory = [];
    try { localStorage.removeItem(SYNC_HISTORY_KEY); } catch { /* unavailable storage is non-fatal */ }
  }

  private loadSyncHistory(): ReassemblySyncHistoryEntry[] {
    if (this.syncHistory) return this.syncHistory;
    try {
      const raw = localStorage.getItem(SYNC_HISTORY_KEY);
      const parsed = raw ? JSON.parse(raw) : [];
      this.syncHistory = Array.isArray(parsed) ? parsed.filter(this.isSyncHistoryEntry).slice(0, SYNC_HISTORY_LIMIT) : [];
    } catch {
      this.syncHistory = [];
    }
    return this.syncHistory;
  }

  private persistSyncHistory(history: readonly ReassemblySyncHistoryEntry[]): void {
    this.syncHistory = [...history];
    try { localStorage.setItem(SYNC_HISTORY_KEY, JSON.stringify(this.syncHistory)); } catch { /* history is best-effort */ }
  }

  private readonly isSyncHistoryEntry = (value: unknown): value is ReassemblySyncHistoryEntry => {
    if (!value || typeof value !== 'object') return false;
    const entry = value as Partial<ReassemblySyncHistoryEntry>;
    return typeof entry.id === 'string' && typeof entry.startedAt === 'number' && typeof entry.completedAt === 'number' &&
      typeof entry.leftRoot === 'string' && typeof entry.rightRoot === 'string' && typeof entry.mode === 'string' &&
      typeof entry.planned === 'number' && typeof entry.succeeded === 'number' && typeof entry.skipped === 'number' &&
      typeof entry.failed === 'number' && typeof entry.conflicts === 'number';
  };

}