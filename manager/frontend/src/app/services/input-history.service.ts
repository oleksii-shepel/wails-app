import { Injectable } from '@angular/core';

const STORAGE_PREFIX = 'fm.inputHistory.';
const MAX_PER_KEY = 20;

/**
 * Stores and retrieves per-key input history from localStorage so that any
 * dialog with a free-text field can offer autocomplete suggestions from
 * previously typed values.
 */
@Injectable({ providedIn: 'root' })
export class InputHistoryService {
  /** Returns the stored suggestions for a given key (most recent first). */
  get(key: string): string[] {
    try {
      const raw = localStorage.getItem(STORAGE_PREFIX + key);
      if (!raw) return [];
      const parsed = JSON.parse(raw);
      return Array.isArray(parsed) ? parsed : [];
    } catch {
      return [];
    }
  }

  /**
   * Pushes a new value to the front of the history for a given key.
   * Duplicates (case-insensitive) are deduplicated. The list is capped at
   * MAX_PER_KEY entries.
   */
  push(key: string, value: string): void {
    const trimmed = value.trim();
    if (!trimmed) return;

    const existing = this.get(key).filter(
      (v) => v.toLowerCase() !== trimmed.toLowerCase(),
    );
    const updated = [trimmed, ...existing].slice(0, MAX_PER_KEY);

    try {
      localStorage.setItem(STORAGE_PREFIX + key, JSON.stringify(updated));
    } catch {
      // Ignore storage errors (private browsing, quota exceeded).
    }
  }

  /** Removes all stored history for a given key. */
  clear(key: string): void {
    localStorage.removeItem(STORAGE_PREFIX + key);
  }
}