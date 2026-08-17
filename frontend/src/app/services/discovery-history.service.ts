import { Injectable } from '@angular/core';

export type DiscoverySearchType = 'all' | 'files' | 'folders' | 'archives';
export type DiscoverySearchSort = 'relevance' | 'name' | 'path' | 'type';

export interface DiscoverySearchSpec {
  readonly rootPath: string;
  readonly query: string;
  readonly recursive: boolean;
  readonly type: DiscoverySearchType;
  readonly sort: DiscoverySearchSort;
}

export interface DiscoverySearchHistoryEntry extends DiscoverySearchSpec {
  readonly id: string;
  readonly searchedAt: number;
  readonly resultCount: number;
}

export interface SavedDiscoverySearch extends DiscoverySearchSpec {
  readonly id: string;
  readonly name: string;
  readonly createdAt: number;
  readonly updatedAt: number;
}

interface DiscoveryState {
  readonly version: 1;
  history: DiscoverySearchHistoryEntry[];
  saved: SavedDiscoverySearch[];
}

const STORAGE_KEY = 'reassembly.discovery.v1';
const MAX_HISTORY = 80;
const MAX_SAVED = 80;

@Injectable({ providedIn: 'root' })
export class DiscoveryHistoryService {
  history(): readonly DiscoverySearchHistoryEntry[] {
    return this.read().history.map(entry => ({ ...entry }));
  }

  saved(): readonly SavedDiscoverySearch[] {
    return this.read().saved
      .map(entry => ({ ...entry }))
      .sort((a,b) => a.name.localeCompare(b.name, undefined, {
        numeric: true,
        sensitivity: 'base',
      }));
  }

  record(spec: DiscoverySearchSpec, resultCount: number): void {
    if (!spec.query.trim() || !spec.rootPath.trim()) return;

    const state = this.read();
    const key = this.specKey(spec);
    const existing = state.history.find(entry => this.specKey(entry) === key);

    state.history = [
      {
        ...spec,
        id: existing?.id ?? this.id('history'),
        searchedAt: Date.now(),
        resultCount: Math.max(0, resultCount),
      },
      ...state.history.filter(entry => this.specKey(entry) !== key),
    ].slice(0, MAX_HISTORY);

    this.write(state);
  }

  save(name: string, spec: DiscoverySearchSpec, id?: string): SavedDiscoverySearch {
    const title = name.trim();
    if (!title) throw new Error('Saved search name is required.');

    const state = this.read();
    const now = Date.now();
    const existing =
      (id ? state.saved.find(entry => entry.id === id) : undefined) ??
      state.saved.find(entry =>
        entry.name.localeCompare(title, undefined, { sensitivity: 'base' }) === 0
      );

    const saved: SavedDiscoverySearch = {
      ...spec,
      id: existing?.id ?? this.id('saved'),
      name: title,
      createdAt: existing?.createdAt ?? now,
      updatedAt: now,
    };

    state.saved = [
      ...state.saved.filter(entry => entry.id !== saved.id),
      saved,
    ].slice(0, MAX_SAVED);

    this.write(state);
    return saved;
  }

  removeSaved(id: string): void {
    const state = this.read();
    state.saved = state.saved.filter(entry => entry.id !== id);
    this.write(state);
  }

  removeHistory(id: string): void {
    const state = this.read();
    state.history = state.history.filter(entry => entry.id !== id);
    this.write(state);
  }

  clearHistory(): void {
    const state = this.read();
    state.history = [];
    this.write(state);
  }

  private read(): DiscoveryState {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return this.empty();

      const value = JSON.parse(raw) as Partial<DiscoveryState>;
      return {
        version: 1,
        history: Array.isArray(value.history)
          ? value.history.filter(entry => this.validSpec(entry)).slice(0, MAX_HISTORY)
          : [],
        saved: Array.isArray(value.saved)
          ? value.saved.filter(entry => this.validSpec(entry) && !!entry?.name).slice(0, MAX_SAVED)
          : [],
      };
    } catch {
      return this.empty();
    }
  }

  private write(state: DiscoveryState): void {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch {
      // Discovery remains functional when persistence is unavailable.
    }
  }

  private validSpec(value: any): boolean {
    return !!value &&
      typeof value.rootPath === 'string' &&
      typeof value.query === 'string' &&
      typeof value.recursive === 'boolean';
  }

  private specKey(spec: DiscoverySearchSpec): string {
    return [
      spec.rootPath.trim().replace(/\\/g, '/').toLocaleLowerCase(),
      spec.query.trim().toLocaleLowerCase(),
      spec.recursive ? 'r' : 'n',
      spec.type,
      spec.sort,
    ].join('|');
  }

  private id(prefix: string): string {
    return `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }

  private empty(): DiscoveryState {
    return { version: 1, history: [], saved: [] };
  }
}
