import { Injectable } from '@angular/core';

import type { FileInfo } from '../shared/protocol-enhanced';

export interface ResultsPanelMetadata {
  readonly resultCount?: number;
  readonly recoveredPartial?: boolean;
  readonly origin?: string;
  readonly rootPath?: string;
  readonly query?: string;
  readonly kind?: string;
  readonly pluginId?: string;
  readonly collectionId?: string;
  readonly refreshedAt?: string;
  readonly refreshWarnings?: string;
  readonly reclaimableBytes?: string;
  readonly duplicateGroups?: string;
  readonly searchStrategy?: 'auto' | 'live' | 'indexed';
  readonly minDuplicateSize?: string;
}

export interface ResultsPanelCreateOptions {
  readonly title: string;
  readonly entries: readonly FileInfo[];
  readonly metadata?: ResultsPanelMetadata;
}

export interface ResultsPanelDefinition {
  readonly id: string;
  readonly title: string;
  readonly path: string;
  readonly createdAt: number;
  readonly updatedAt?: number;
  readonly entries: readonly FileInfo[];
  readonly metadata: ResultsPanelMetadata;
  readonly pinned?: boolean;
}

export interface ResultsPanelRefreshResult {
  readonly entries: readonly FileInfo[];
  readonly metadata?: ResultsPanelMetadata;
  readonly title?: string;
}

export type ResultsPanelRefresher =
  () => Promise<ResultsPanelRefreshResult | readonly FileInfo[]>;

const STORAGE_KEY = 'reassembly.resultsPanels.v1';
const PINNED_STORAGE_KEY = 'reassembly.resultsPanels.pinned.v1';
const MAX_PANELS = 24;
const MAX_PERSISTED_ENTRIES_PER_PANEL = 2000;
const MAX_PERSISTED_TOTAL_ENTRIES = 8000;
const RECOVERY_KEY = 'reassembly.resultsPanels.recovery.v1';

function parseResultsId(path: string): string | null {
  const match = /^results:\/\/([^/?#]+)/i.exec((path ?? '').trim());
  return match?.[1] ? decodeURIComponent(match[1]) : null;
}

@Injectable({ providedIn: 'root' })
export class ResultsPanelService {
  private readonly panels = new Map<string, ResultsPanelDefinition>();
  private readonly refreshers = new Map<string, ResultsPanelRefresher>();
  private hydrated = false;

  create(
    title: string,
    entries: readonly FileInfo[],
    metadata: ResultsPanelMetadata = {},
  ): ResultsPanelDefinition {
    this.hydrate();

    const id =
      globalThis.crypto?.randomUUID?.() ??
      `${Date.now()}-${Math.random().toString(16).slice(2)}`;

    const path = `results://${encodeURIComponent(id)}`;
    const definition: ResultsPanelDefinition = {
      id,
      title: title.trim() || 'Results',
      path,
      createdAt: Date.now(),
      entries: [...entries],
      metadata: { ...metadata },
    };

    this.panels.set(id, definition);
    this.trim();
    this.persist();
    return definition;
  }


  createSearch(
    title: string,
    entries: readonly FileInfo[],
    rootPath: string,
    query: string,
    recursive = false,
    strategy: 'auto' | 'live' | 'indexed' = 'auto',
  ): ResultsPanelDefinition {
    return this.create(title, entries, {
      origin: 'global-search',
      rootPath,
      query,
      kind: recursive ? 'recursive-search' : 'search',
      searchStrategy: strategy,
    });
  }

  createRecentFiles(
    entries: readonly FileInfo[],
    title = 'Recent Files',
  ): ResultsPanelDefinition {
    return this.create(title, entries, {
      origin: 'recent-files',
      kind: 'recent-files',
    });
  }

  createDuplicateResults(
    rootPath: string,
    entries: readonly FileInfo[],
    title = 'Duplicate Results',
  ): ResultsPanelDefinition {
    return this.create(title, entries, {
      origin: 'duplicate-finder',
      rootPath,
      kind: 'duplicates',
    });
  }

  createPluginCollection(
    pluginId: string,
    collectionId: string,
    title: string,
    entries: readonly FileInfo[],
    metadata: ResultsPanelMetadata = {},
  ): ResultsPanelDefinition {
    const owner = pluginId.trim();
    const collection = collectionId.trim();
    if (!owner) throw new Error('Plugin collection requires a plugin id.');
    if (!collection) throw new Error('Plugin collection requires a collection id.');

    return this.create(title, entries, {
      ...metadata,
      origin: `plugin:${owner}`,
      kind: metadata.kind || 'plugin-collection',
      pluginId: owner,
      collectionId: collection,
    });
  }

  removePluginCollections(pluginId: string): void {
    this.hydrate();
    const owner = pluginId.trim();
    if (!owner) return;

    let changed = false;
    for (const [id, panel] of this.panels) {
      if (panel.metadata.pluginId !== owner) continue;
      this.panels.delete(id);
      this.refreshers.delete(id);
      changed = true;
    }
    if (changed) this.persist();
  }

  get(path: string): ResultsPanelDefinition | null {
    this.hydrate();
    const id = parseResultsId(path);
    return id ? this.panels.get(id) ?? null : null;
  }

  async stream(
    path: string,
    options: {
      readonly batchSize?: number;
      readonly signal?: AbortSignal;
      readonly onBatch: (
        batch: readonly FileInfo[],
        loaded: number,
        total: number,
      ) => void;
    },
  ): Promise<number> {
    const panel = this.get(path);
    if (!panel) return 0;

    const batchSize = Math.max(64, Math.min(2048, options.batchSize ?? 512));
    const total = panel.entries.length;

    for (let start = 0; start < total; start += batchSize) {
      if (options.signal?.aborted) {
        throw new DOMException('Results stream cancelled.', 'AbortError');
      }

      const end = Math.min(total, start + batchSize);
      options.onBatch(panel.entries.slice(start, end), end, total);

      // Yield between batches so very large saved/virtual collections do not
      // monopolize the renderer while being attached to a pane.
      if (end < total) {
        await this.nextFrame(options.signal);
      }
    }

    return total;
  }

  private async nextFrame(signal?: AbortSignal): Promise<void> {
    if (signal?.aborted) {
      throw new DOMException('Results stream cancelled.', 'AbortError');
    }

    await new Promise<void>((resolve, reject) => {
      let handle = 0;
      const abort = () => {
        if (handle && typeof cancelAnimationFrame === 'function') {
          cancelAnimationFrame(handle);
        }
        reject(new DOMException('Results stream cancelled.', 'AbortError'));
      };

      signal?.addEventListener('abort', abort, { once: true });

      if (typeof requestAnimationFrame === 'function') {
        handle = requestAnimationFrame(() => {
          signal?.removeEventListener('abort', abort);
          resolve();
        });
      } else {
        setTimeout(() => {
          signal?.removeEventListener('abort', abort);
          resolve();
        }, 0);
      }
    });
  }

  has(path: string): boolean {
    return !!this.get(path);
  }

  title(path: string): string | null {
    return this.get(path)?.title ?? null;
  }

  remove(path: string): void {
    this.hydrate();
    const id = parseResultsId(path);
    if (!id) return;
    this.panels.delete(id);
    this.refreshers.delete(id);
    this.persist();
  }

  update(
    path: string,
    update: {
      entries?: readonly FileInfo[];
      metadata?: ResultsPanelMetadata;
      title?: string;
    },
  ): ResultsPanelDefinition | null {
    this.hydrate();
    const id = parseResultsId(path);
    const current = id ? this.panels.get(id) : null;
    if (!id || !current) return null;

    const next: ResultsPanelDefinition = {
      ...current,
      title: update.title?.trim() || current.title,
      entries: update.entries
        ? update.entries.map(entry => ({ ...entry }))
        : current.entries,
      metadata: update.metadata
        ? { ...current.metadata, ...update.metadata }
        : current.metadata,
      updatedAt: Date.now(),
    };

    this.panels.set(id, next);
    this.persist();
    return next;
  }

  registerRefresher(
    path: string,
    refresher: ResultsPanelRefresher,
  ): () => void {
    this.hydrate();
    const id = parseResultsId(path);
    if (!id || !this.panels.has(id)) {
      throw new Error(`Unknown results panel: ${path}`);
    }

    this.refreshers.set(id, refresher);
    return () => {
      if (this.refreshers.get(id) === refresher) {
        this.refreshers.delete(id);
      }
    };
  }

  canRefresh(path: string): boolean {
    this.hydrate();
    const id = parseResultsId(path);
    return !!id && this.refreshers.has(id);
  }

  async refresh(path: string): Promise<ResultsPanelDefinition | null> {
    this.hydrate();
    const id = parseResultsId(path);
    const current = id ? this.panels.get(id) : null;
    const refresher = id ? this.refreshers.get(id) : null;
    if (!id || !current || !refresher) return null;

    const value = await refresher();
    const result: ResultsPanelRefreshResult = Array.isArray(value)
      ? { entries: value }
      : value as ResultsPanelRefreshResult;

    return this.update(path, {
      entries: result.entries,
      metadata: result.metadata,
      title: result.title,
    });
  }

  rename(path: string, title: string): void {
    this.hydrate();
    const id = parseResultsId(path);
    const current = id ? this.panels.get(id) : null;
    const nextTitle = title.trim();
    if (!id || !current || !nextTitle) return;

    this.panels.set(id, { ...current, title: nextTitle });
    this.persist();
  }

  setPinned(path: string, pinned: boolean): void {
    this.hydrate();
    const id = parseResultsId(path);
    const current = id ? this.panels.get(id) : null;
    if (!id || !current) return;

    this.panels.set(id, { ...current, pinned });
    this.persist();
  }

  pinned(): readonly ResultsPanelDefinition[] {
    this.hydrate();
    return [...this.panels.values()]
      .filter(panel => !!panel.pinned)
      .sort((a,b) => b.createdAt - a.createdAt);
  }

  recent(): readonly ResultsPanelDefinition[] {
    this.hydrate();
    return [...this.panels.values()]
      .sort((left, right) => right.createdAt - left.createdAt);
  }

  private hydrate(): void {
    if (this.hydrated) return;
    this.hydrated = true;

    try {
      const raw = sessionStorage.getItem(STORAGE_KEY);
      const parsed = raw
        ? JSON.parse(raw) as ResultsPanelDefinition[]
        : [];

      for (const panel of parsed ?? []) {
        if (
          !panel?.id ||
          !panel?.path ||
          !Array.isArray(panel.entries)
        ) {
          continue;
        }

        this.panels.set(panel.id, {
          ...panel,
          entries: [...panel.entries],
          metadata: {
            ...(panel.metadata ?? {}),
            recoveredPartial:
              Number(panel.metadata?.resultCount ?? panel.entries.length) >
              panel.entries.length,
          },
        });
      }
    } catch {
      this.panels.clear();
    }

    try {
      const raw = localStorage.getItem(PINNED_STORAGE_KEY);
      const parsed = raw ? JSON.parse(raw) as ResultsPanelDefinition[] : [];

      for (const panel of parsed ?? []) {
        if (!panel?.id || !panel?.path || !Array.isArray(panel.entries)) continue;

        // Plugin-owned collections are lifecycle-owned and are intentionally
        // not resurrected from snapshots when their plugin is absent.
        if (panel.metadata?.pluginId) continue;

        this.panels.set(panel.id, {
          ...panel,
          pinned: true,
          entries: panel.entries.map(entry => ({ ...entry })),
          metadata: { ...(panel.metadata ?? {}) },
        });
      }
    } catch {
      // Session panels remain usable if persistent pinned hydration fails.
    }
  }

  private trim(): void {
    const orderedUnpinned = [...this.panels.values()]
      .filter(panel => !panel.pinned)
      .sort((left, right) => right.createdAt - left.createdAt);

    for (const panel of orderedUnpinned.slice(MAX_PANELS)) {
      this.panels.delete(panel.id);
    }
  }

  private persist(): void {
    try {
      let remaining = MAX_PERSISTED_TOTAL_ENTRIES;
      const payload = [...this.panels.values()]
        .sort((a,b)=>b.createdAt-a.createdAt)
        .map(panel => {
          const count = Math.min(
            panel.entries.length,
            MAX_PERSISTED_ENTRIES_PER_PANEL,
            remaining,
          );
          remaining -= count;

          return {
            ...panel,
            entries: panel.entries.slice(0,count),
            metadata: {
              ...panel.metadata,
              resultCount: panel.entries.length,
              recoveredPartial: false,
            },
          };
        });

      const encoded = JSON.stringify(payload);
      sessionStorage.setItem(STORAGE_KEY, encoded);
      localStorage.setItem(RECOVERY_KEY, encoded);
    } catch {
      // The complete result model remains in memory. Persistence is bounded
      // recovery state, not the authoritative result store.
    }
  }
}