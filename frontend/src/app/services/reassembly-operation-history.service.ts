import { Injectable } from '@angular/core';

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
}
