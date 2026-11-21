import { Injectable, signal } from '@angular/core';


export type SyncMode = 'left-to-right' | 'right-to-left' | 'bidirectional' | 'custom';

export interface SyncHistoryEntry {
  readonly id: string;
  readonly startedAt: number;
  readonly completedAt: number;
  readonly leftRoot: string;
  readonly rightRoot: string;
  readonly mode: SyncMode;
  readonly recursive: boolean;
  readonly planned: number;
  readonly succeeded: number;
  readonly skipped: number;
  readonly failed: number;
  readonly conflicts: number;
}

const SYNC_HISTORY_KEY = 'reassembly.syncHistory.v1';
const SYNC_HISTORY_LIMIT = 100;

export type OperationJournalState =
  | 'completed'
  | 'partial'
  | 'failed'
  | 'undone'
  | 'redone';

export interface OperationJournalEntry {
  readonly id: string;
  readonly label: string;
  readonly kind: string;
  readonly startedAt: number;
  readonly completedAt: number;
  readonly state: OperationJournalState;
  readonly reversible: boolean;
  readonly affectedPaths: readonly string[];
  readonly summary?: string;
  readonly error?: string;
}

export interface ReversibleAction {
  readonly label: string;
  readonly undo: () => Promise<void>;
  readonly redo: () => Promise<void>;
  readonly kind?: string;
  readonly affectedPaths?: readonly string[];
  readonly summary?: string;
}

@Injectable({ providedIn: 'root' })
export class OperationHistoryService {
  private readonly undoStack: Array<{ action: ReversibleAction; journalId: string }> = [];
  private readonly redoStack: Array<{ action: ReversibleAction; journalId: string }> = [];
  private readonly running = signal(false);
  private readonly revision = signal(0);
  private readonly limit = 100;
  private syncHistory: SyncHistoryEntry[] | null = null;
  private journal: OperationJournalEntry[] = [];

  get canUndo(): boolean {
    this.revision();
    return !this.running() && this.undoStack.length > 0;
  }

  get canRedo(): boolean {
    this.revision();
    return !this.running() && this.redoStack.length > 0;
  }

  get isBusy(): boolean {
    return this.running();
  }

  record(action: ReversibleAction): void {
    if (this.running()) return;
    const entry = this.appendJournal({
      label: action.label,
      kind: action.kind ?? 'file-operation',
      reversible: true,
      affectedPaths: action.affectedPaths ?? [],
      summary: action.summary,
      state: 'completed',
    });
    this.undoStack.push({ action, journalId: entry.id });
    if (this.undoStack.length > this.limit) {
      this.undoStack.splice(0, this.undoStack.length - this.limit);
    }
    this.redoStack.length = 0;
  }

  recordOutcome(entry: {
    label: string;
    kind?: string;
    reversible?: boolean;
    affectedPaths?: readonly string[];
    summary?: string;
    state?: OperationJournalState;
    error?: string;
    startedAt?: number;
  }): OperationJournalEntry {
    return this.appendJournal({
      label: entry.label,
      kind: entry.kind ?? 'file-operation',
      reversible: entry.reversible ?? false,
      affectedPaths: entry.affectedPaths ?? [],
      summary: entry.summary,
      state: entry.state ?? 'completed',
      error: entry.error,
      startedAt: entry.startedAt,
    });
  }

  recentOperations(limit = 100): readonly OperationJournalEntry[] {
    this.revision();
    return this.journal.slice(0, Math.max(0, limit)).map(entry => ({
      ...entry,
      affectedPaths: [...entry.affectedPaths],
    }));
  }

  async undo(): Promise<string | null> {
    if (!this.canUndo) return null;
    const entry = this.undoStack.pop();
    if (!entry) return null;
    const { action, journalId } = entry;

    this.running.set(true);
    try {
      await action.undo();
      this.redoStack.push(entry);
      this.markJournalState(journalId, 'undone');
      return action.label;
    } catch (error) {
      this.undoStack.push(entry);
      throw error;
    } finally {
      this.running.set(false);
    }
  }

  async redo(): Promise<string | null> {
    if (!this.canRedo) return null;
    const entry = this.redoStack.pop();
    if (!entry) return null;
    const { action, journalId } = entry;

    this.running.set(true);
    try {
      await action.redo();
      this.undoStack.push(entry);
      this.markJournalState(journalId, 'redone');
      return action.label;
    } catch (error) {
      this.redoStack.push(entry);
      throw error;
    } finally {
      this.running.set(false);
    }
  }

  clear(): void {
    if (this.running()) return;
    this.undoStack.length = 0;
    this.redoStack.length = 0;
    this.revision.update(value => value + 1);
  }

  clearJournal(): void {
    if (this.running()) return;
    this.journal = [];
    this.revision.update(value => value + 1);
  }

  recordSync(entry: Omit<SyncHistoryEntry, 'id'> & { readonly id?: string }): SyncHistoryEntry {
    const recorded: SyncHistoryEntry = {
      ...entry,
      id: entry.id ?? globalThis.crypto?.randomUUID?.() ?? `${entry.completedAt}-${Math.random().toString(16).slice(2)}`,
    };
    const history = this.loadSyncHistory();
    history.unshift(recorded);
    if (history.length > SYNC_HISTORY_LIMIT) history.splice(SYNC_HISTORY_LIMIT);
    this.persistSyncHistory(history);
    return recorded;
  }

  recentSyncs(limit = 20): readonly SyncHistoryEntry[] {
    return this.loadSyncHistory().slice(0, Math.max(0, limit)).map(entry => ({ ...entry }));
  }

  clearSyncHistory(): void {
    this.syncHistory = [];
    try { localStorage.removeItem(SYNC_HISTORY_KEY); } catch { /* unavailable storage is non-fatal */ }
  }

  private appendJournal(entry: {
    label: string;
    kind: string;
    reversible: boolean;
    affectedPaths: readonly string[];
    summary?: string;
    state: OperationJournalState;
    error?: string;
    startedAt?: number;
  }): OperationJournalEntry {
    const completedAt = Date.now();
    const recorded: OperationJournalEntry = {
      id: globalThis.crypto?.randomUUID?.() ?? `${completedAt}-${Math.random().toString(16).slice(2)}`,
      label: entry.label,
      kind: entry.kind,
      startedAt: entry.startedAt ?? completedAt,
      completedAt,
      state: entry.state,
      reversible: entry.reversible,
      affectedPaths: [...entry.affectedPaths],
      summary: entry.summary,
      error: entry.error,
    };
    this.journal.unshift(recorded);
    if (this.journal.length > this.limit) this.journal.splice(this.limit);
    this.revision.update(value => value + 1);
    return recorded;
  }

  private markJournalState(id: string, state: 'undone' | 'redone'): void {
    const index = this.journal.findIndex(entry => entry.id === id && entry.reversible);
    if (index < 0) return;
    this.journal[index] = { ...this.journal[index], state };
    this.revision.update(value => value + 1);
  }

  private loadSyncHistory(): SyncHistoryEntry[] {
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

  private persistSyncHistory(history: readonly SyncHistoryEntry[]): void {
    this.syncHistory = [...history];
    try { localStorage.setItem(SYNC_HISTORY_KEY, JSON.stringify(this.syncHistory)); } catch { /* history is best-effort */ }
  }

  private readonly isSyncHistoryEntry = (value: unknown): value is SyncHistoryEntry => {
    if (!value || typeof value !== 'object') return false;
    const entry = value as Partial<SyncHistoryEntry>;
    return typeof entry.id === 'string' && typeof entry.startedAt === 'number' && typeof entry.completedAt === 'number' &&
      typeof entry.leftRoot === 'string' && typeof entry.rightRoot === 'string' && typeof entry.mode === 'string' &&
      typeof entry.planned === 'number' && typeof entry.succeeded === 'number' && typeof entry.skipped === 'number' &&
      typeof entry.failed === 'number' && typeof entry.conflicts === 'number';
  };

}
