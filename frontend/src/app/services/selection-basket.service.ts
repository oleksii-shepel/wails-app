import { Injectable } from '@angular/core';
import { createBehaviorSubject, distinctUntilChanged, map, shareReplay } from '@epikodelabs/streamix';
import { FileType } from '../shared/protocol';
import { canonicalizeLocation, locationIdentity, locationLabel } from './location-reference';
import { ApiService } from './api.service';

export interface SelectionBasketEntry {
  id: string;
  path: string;
  label: string;
  addedAt: number;
  source?: string;
  type?: FileType;
  size?: number;
  modified?: number;
  metadataResolved: boolean;
}

interface StoredBasketEntry {
  path: string;
  addedAt: number;
  source?: string;
}

@Injectable({ providedIn: 'root' })
export class SelectionBasketService {
  private readonly storageKey = 'filemanager:selection-basket-v1';
  private readonly entriesSubject = createBehaviorSubject<SelectionBasketEntry[]>(this.load());

  readonly entries$ = this.entriesSubject;
  readonly count$ = this.entries$.pipe(
    map(entries => entries.length),
    distinctUntilChanged(),
    shareReplay(1),
  );

  constructor(private readonly api: ApiService) {
    for (const entry of this.entriesSubject.value) {
      void this.resolveMetadata(entry.id, entry.path);
    }
  }

  get snapshot(): readonly SelectionBasketEntry[] {
    return this.entriesSubject.value;
  }

  get paths(): string[] {
    return this.entriesSubject.value.map(entry => entry.path);
  }

  add(paths: readonly string[], source?: string): number {
    const current = [...this.entriesSubject.value];
    const ids = new Set(current.map(entry => entry.id));
    const added: SelectionBasketEntry[] = [];

    for (const raw of paths) {
      const path = canonicalizeLocation(raw);
      if (!path) continue;
      const id = locationIdentity(path);
      if (!id || ids.has(id)) continue;
      ids.add(id);
      added.push({
        id,
        path,
        label: locationLabel(path) || this.baseName(path),
        addedAt: Date.now(),
        source,
        metadataResolved: false,
      });
    }

    if (added.length === 0) return 0;
    this.publish([...current, ...added]);
    for (const entry of added) void this.resolveMetadata(entry.id, entry.path);
    return added.length;
  }

  remove(idOrPath: string): boolean {
    const identity = idOrPath.includes(':') && !idOrPath.includes('://')
      ? idOrPath
      : locationIdentity(canonicalizeLocation(idOrPath));
    const current = this.entriesSubject.value;
    const next = current.filter(entry => entry.id !== identity);
    if (next.length === current.length) return false;
    this.publish(next);
    return true;
  }

  removePaths(paths: readonly string[]): number {
    const identities = new Set(paths.map(path => locationIdentity(canonicalizeLocation(path))).filter(Boolean));
    if (identities.size === 0) return 0;
    const current = this.entriesSubject.value;
    const next = current.filter(entry => !identities.has(entry.id));
    const removed = current.length - next.length;
    if (removed > 0) this.publish(next);
    return removed;
  }

  clear(): void {
    if (this.entriesSubject.value.length === 0) return;
    this.publish([]);
  }

  contains(path: string): boolean {
    const id = locationIdentity(canonicalizeLocation(path));
    return !!id && this.entriesSubject.value.some(entry => entry.id === id);
  }

  private async resolveMetadata(id: string, path: string): Promise<void> {
    try {
      const info = await this.api.getFileInfo(path);
      this.update(id, {
        label: info.name || locationLabel(path) || this.baseName(path),
        type: info.type,
        size: info.size,
        modified: info.modified,
        metadataResolved: true,
      });
    } catch {
      this.update(id, { metadataResolved: true });
    }
  }

  private update(id: string, patch: Partial<SelectionBasketEntry>): void {
    const current = this.entriesSubject.value;
    const index = current.findIndex(entry => entry.id === id);
    if (index < 0) return;
    const next = [...current];
    next[index] = { ...next[index], ...patch };
    this.entriesSubject.next(next);
  }

  private publish(entries: SelectionBasketEntry[]): void {
    this.entriesSubject.next(entries);
    this.persist(entries);
  }

  private load(): SelectionBasketEntry[] {
    try {
      const raw = sessionStorage.getItem(this.storageKey);
      if (!raw) return [];
      const stored = JSON.parse(raw) as StoredBasketEntry[];
      if (!Array.isArray(stored)) return [];
      const result: SelectionBasketEntry[] = [];
      const seen = new Set<string>();
      for (const item of stored) {
        const path = canonicalizeLocation(item?.path ?? '');
        const id = locationIdentity(path);
        if (!path || !id || seen.has(id)) continue;
        seen.add(id);
        result.push({
          id,
          path,
          label: locationLabel(path) || this.baseName(path),
          addedAt: Number.isFinite(item.addedAt) ? item.addedAt : Date.now(),
          source: item.source,
          metadataResolved: false,
        });
      }
      return result;
    } catch {
      return [];
    }
  }

  private persist(entries: readonly SelectionBasketEntry[]): void {
    try {
      const stored: StoredBasketEntry[] = entries.map(({ path, addedAt, source }) => ({ path, addedAt, source }));
      sessionStorage.setItem(this.storageKey, JSON.stringify(stored));
    } catch {
      // Basket persistence is a convenience; the in-memory working set remains authoritative.
    }
  }

  private baseName(path: string): string {
    return path.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || path;
  }
}
