import { Injectable } from '@angular/core';

import type { FileInfo } from '../shared/protocol-enhanced';

interface RecentFileRecord {
  readonly openedAt: number;
  readonly entry: FileInfo;
}

const STORAGE_KEY = 'reassembly.recentFiles.v1';
const MAX_RECENT_FILES = 100;

@Injectable({ providedIn: 'root' })
export class RecentFilesService {
  private records: RecentFileRecord[] | null = null;

  record(entry: FileInfo): void {
    if (!entry?.path || entry.path === '__parent__') return;
    const records = this.load();
    const key = this.key(entry.path);
    const next: RecentFileRecord[] = [
      { openedAt: Date.now(), entry: { ...entry } },
      ...records.filter(record => this.key(record.entry.path) !== key),
    ].slice(0, MAX_RECENT_FILES);

    this.records = next;
    this.persist();
  }

  recent(limit = 50): FileInfo[] {
    return this.load()
      .slice(0, Math.max(0, limit))
      .map(record => ({ ...record.entry }));
  }

  clear(): void {
    this.records = [];
    try { localStorage.removeItem(STORAGE_KEY); } catch { /* best effort */ }
  }

  private load(): RecentFileRecord[] {
    if (this.records) return this.records;
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      const parsed = raw ? JSON.parse(raw) as RecentFileRecord[] : [];
      this.records = (parsed ?? [])
        .filter(record => !!record?.entry?.path)
        .sort((a, b) => b.openedAt - a.openedAt)
        .slice(0, MAX_RECENT_FILES)
        .map(record => ({ openedAt: Number(record.openedAt) || 0, entry: { ...record.entry } }));
    } catch {
      this.records = [];
    }
    return this.records;
  }

  private persist(): void {
    try { localStorage.setItem(STORAGE_KEY, JSON.stringify(this.records ?? [])); } catch { /* best effort */ }
  }

  private key(path: string): string {
    return path.trim().replace(/\\/g, '/').toLocaleLowerCase();
  }
}
