import { Injectable } from '@angular/core';

import type { FileInfo } from '../shared/protocol-enhanced';

export interface ResultsPanelMetadata {
  readonly origin?: string;
  readonly rootPath?: string;
  readonly query?: string;
  readonly kind?: string;
  readonly pluginId?: string;
  readonly collectionId?: string;
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
  readonly entries: readonly FileInfo[];
  readonly metadata: ResultsPanelMetadata;
}

const STORAGE_KEY = 'reassembly.resultsPanels.v1';
const MAX_PANELS = 24;

function parseResultsId(path: string): string | null {
  const match = /^results:\/\/([^/?#]+)/i.exec((path ?? '').trim());
  return match?.[1] ? decodeURIComponent(match[1]) : null;
}

@Injectable({ providedIn: 'root' })
export class ResultsPanelService {
  private readonly panels = new Map<string, ResultsPanelDefinition>();
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
      entries: entries.map(entry => ({ ...entry })),
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
  ): ResultsPanelDefinition {
    return this.create(title, entries, {
      origin: 'global-search',
      rootPath,
      query,
      kind: recursive ? 'recursive-search' : 'search',
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
      changed = true;
    }
    if (changed) this.persist();
  }

  get(path: string): ResultsPanelDefinition | null {
    this.hydrate();
    const id = parseResultsId(path);
    return id ? this.panels.get(id) ?? null : null;
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
    this.persist();
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
      if (!raw) return;

      const parsed = JSON.parse(raw) as ResultsPanelDefinition[];
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
          entries: panel.entries.map(entry => ({ ...entry })),
          metadata: { ...(panel.metadata ?? {}) },
        });
      }
    } catch {
      this.panels.clear();
    }
  }

  private trim(): void {
    const ordered = [...this.panels.values()]
      .sort((left, right) => right.createdAt - left.createdAt);

    for (const panel of ordered.slice(MAX_PANELS)) {
      this.panels.delete(panel.id);
    }
  }

  private persist(): void {
    try {
      sessionStorage.setItem(
        STORAGE_KEY,
        JSON.stringify([...this.panels.values()]),
      );
    } catch {
      // Session persistence is best-effort. The in-memory panel remains usable.
    }
  }
}