import { Injectable } from '@angular/core';

const STORAGE_KEY = 'command-palette-history-v1';
const MAX_HISTORY = 15;

@Injectable({ providedIn: 'root' })
export class CommandPaletteHistoryService {
  private history: string[] = [];

  constructor() {
    this.load();
  }

  getHistory(): string[] {
    return [...this.history];
  }

  recordCommand(commandId: string): void {
    // Move to front or add
    this.history = this.history.filter(id => id !== commandId);
    this.history.unshift(commandId);
    if (this.history.length > MAX_HISTORY) {
      this.history = this.history.slice(0, MAX_HISTORY);
    }
    this.save();
  }

  clearHistory(): void {
    this.history = [];
    this.save();
  }

  private load(): void {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw) {
        const parsed = JSON.parse(raw) as string[];
        if (Array.isArray(parsed)) {
          this.history = parsed;
        }
      }
    } catch {
      // ignore
    }
  }

  private save(): void {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.history));
    } catch {
      // ignore
    }
  }
}