import { Injectable } from '@angular/core';

import { ApiService } from './api.service';
import { PanelSourceRegistryService } from './panel-source-registry.service';
import { FileInfo, FileType } from '../shared/protocol';

export type DiscoverySearchEntry = FileInfo & {
  contentMatchCount?: number;
  contentSnippet?: string;
};

export interface DiscoverySearchOptions {
  readonly recursive?: boolean;
  readonly showHidden?: boolean;
  readonly maxResults?: number;
  readonly maxVisitedDirectories?: number;
  readonly maxDepth?: number;
  readonly type?: 'all' | 'files' | 'folders' | 'archives';
  readonly contentQuery?: string;
  readonly caseSensitive?: boolean;
  readonly signal?: AbortSignal;
  readonly onProgress?: (progress: DiscoverySearchProgress) => void;
  readonly onBatch?: (
    batch: readonly DiscoverySearchEntry[],
    progress: DiscoverySearchProgress,
  ) => void;
  readonly batchSize?: number;
}

export interface DiscoverySearchProgress {
  readonly sourceId: string;
  readonly visitedDirectories: number;
  readonly matchedEntries: number;
  readonly currentPath: string;
}

export interface DiscoverySearchResult {
  readonly rootPath: string;
  readonly sourceId: string;
  readonly sourceKind: string;
  readonly entries: readonly DiscoverySearchEntry[];
  readonly visitedDirectories: number;
  readonly truncated: boolean;
  readonly warnings: readonly string[];
}
import { WorkSchedulerService } from './work-scheduler.service';

@Injectable({ providedIn: 'root' })
export class DiscoverySearchService {
  constructor(
    private readonly api: ApiService,
    private readonly panelSources: PanelSourceRegistryService,
    private readonly work: WorkSchedulerService,
  ) {}

  async search(
    rootPath: string,
    query: string,
    options: DiscoverySearchOptions = {},
  ): Promise<DiscoverySearchResult> {
    const root = rootPath.trim();
    const needle = query.trim();
    const contentNeedle = (options.contentQuery ?? '').trim();

    if (!root || (!needle && !contentNeedle)) {
      return {
        rootPath: root,
        sourceId: '',
        sourceKind: '',
        entries: [],
        visitedDirectories: 0,
        truncated: false,
        warnings: [],
      };
    }

    const resolved = this.panelSources.resolve({
      path: root,
      entries: [],
      inArchive: false,
    });

    // Preserve the fast native backend search for ordinary local files.
    if (
      !options.onBatch &&
      resolved.source.kind === 'filesystem' &&
      /^([a-z]:[\\/]|\/|\\\\)/i.test(root)
    ) {
      try {
        const result = contentNeedle
          ? await this.api.searchContent(
              root,
              needle,
              contentNeedle,
              options.recursive !== false,
              Boolean(options.caseSensitive),
            )
          : await this.api.search(
              root,
              needle,
              options.recursive !== false,
            );

        const rawMatches = (result.matches ?? []) as DiscoverySearchEntry[];
        const filtered = contentNeedle && !rawMatches.some(m => typeof m.contentMatchCount === 'number')
          ? await this.filterEntriesByContent(rawMatches, contentNeedle, Boolean(options.caseSensitive), options.signal)
          : rawMatches;

        const matches = filtered
          .filter(entry => this.typeAllowed(entry, options.type ?? 'all'))
          .slice(0, Math.max(1, options.maxResults ?? 500));

        return {
          rootPath: root,
          sourceId: resolved.location.sourceId,
          sourceKind: resolved.source.kind,
          entries: matches,
          visitedDirectories: 1,
          truncated: filtered.length > matches.length,
          warnings: [],
        };
      } catch {
        // Fall through to PanelSource traversal. This is important for aliased
        // or plugin-backed paths that look filesystem-ish but aren't.
      }
    }

    return this.searchPanelSource(root, needle, resolved.location.sourceId, resolved.source.kind, options);
  }

  private async searchPanelSource(
    root: string,
    query: string,
    sourceId: string,
    sourceKind: string,
    options: DiscoverySearchOptions,
  ): Promise<DiscoverySearchResult> {
    const recursive = options.recursive !== false;
    const maxResults = Math.max(1, options.maxResults ?? 100_000);
    const maxVisited = Math.max(1, options.maxVisitedDirectories ?? 100_000);
    const maxDepth = Math.max(0, options.maxDepth ?? 64);
    const batchSize = Math.max(64, Math.min(2048, options.batchSize ?? 512));
    const matcher = query ? this.matcher(query, Boolean(options.caseSensitive)) : () => true;
    const contentNeedle = (options.contentQuery ?? '').trim();

    // Streaming consumers own the result population. Do not also retain another
    // 100k-entry array in this service.
    const retainedMatches: DiscoverySearchEntry[] = [];
    const batch: DiscoverySearchEntry[] = [];
    const warnings: string[] = [];
    const visited = new Set<string>();
    const queue: Array<{ path: string; depth: number }> = [{ path: root, depth: 0 }];
    let matchedEntries = 0;
    let truncated = false;

    const progress = (currentPath: string): DiscoverySearchProgress => ({
      sourceId,
      visitedDirectories: visited.size,
      matchedEntries,
      currentPath,
    });

    const flush = (currentPath: string): void => {
      if (!batch.length) return;
      const out = batch.splice(0, batch.length);
      options.onBatch?.(out, progress(currentPath));
    };

    while (queue.length) {
      this.throwIfAborted(options.signal);

      if (visited.size >= maxVisited || matchedEntries >= maxResults) {
        truncated = true;
        break;
      }

      const current = queue.shift()!;
      const key = this.pathKey(current.path);
      if (visited.has(key)) continue;
      visited.add(key);

      let listing;
      try {
        listing = await this.work.schedule({
          key: `provider:list:${sourceId}:${current.path}:${options.showHidden ? 1 : 0}`,
          priority: 'interactive',
          kind: 'provider',
          run: async context => {
            this.throwIfAborted(options.signal);
            if (context.signal.aborted) {
              throw new DOMException('Provider listing cancelled.', 'AbortError');
            }
            return await this.panelSources.list(
              current.path,
              !!options.showHidden,
            );
          },
        }).promise;
      } catch (error) {
        warnings.push(`${current.path}: ${this.message(error)}`);
        continue;
      }

      for (const entry of listing.entries ?? []) {
        this.throwIfAborted(options.signal);
        if (!options.showHidden && entry.isHidden) continue;

        if (
          matcher(entry) &&
          this.typeAllowed(entry, options.type ?? 'all')
        ) {
          let copy: DiscoverySearchEntry | null = { ...entry };
          if (contentNeedle) {
            if (entry.type !== FileType.FILE || entry.size <= 0 || entry.size > 2 * 1024 * 1024) {
              copy = null;
            } else {
              const hit = await this.matchSingleFileContent(entry.path, contentNeedle, Boolean(options.caseSensitive));
              if (!hit) {
                copy = null;
              } else {
                copy = { ...entry, contentMatchCount: hit.count, contentSnippet: hit.snippet };
              }
            }
          }

          if (copy) {
            matchedEntries += 1;

            if (options.onBatch) {
              batch.push(copy);
              if (batch.length >= batchSize) flush(current.path);
            } else {
              retainedMatches.push(copy);
            }

            if (matchedEntries >= maxResults) {
              truncated = true;
              break;
            }
          }
        }

        if (
          recursive &&
          current.depth < maxDepth &&
          entry.type === FileType.DIRECTORY &&
          !entry.isSymlink
        ) {
          queue.push({ path: entry.path, depth: current.depth + 1 });
        }
      }

      flush(current.path);
      options.onProgress?.(progress(current.path));

      if (!recursive) break;

      // Let Angular paint result windows and process cancellation between
      // directory batches rather than monopolising the renderer.
      if (options.onBatch && queue.length) {
        await this.work.yield('interactive', options.signal);
      }
    }

    flush(root);

    return {
      rootPath: root,
      sourceId,
      sourceKind,
      entries: retainedMatches,
      visitedDirectories: visited.size,
      truncated,
      warnings,
    };
  }

  private async filterEntriesByContent(
    entries: readonly DiscoverySearchEntry[],
    contentNeedle: string,
    caseSensitive: boolean,
    signal?: AbortSignal,
  ): Promise<DiscoverySearchEntry[]> {
    const out: DiscoverySearchEntry[] = [];
    for (const entry of entries) {
      this.throwIfAborted(signal);
      if (entry.type !== FileType.FILE || entry.size <= 0 || entry.size > 2 * 1024 * 1024) continue;
      const hit = await this.matchSingleFileContent(entry.path, contentNeedle, caseSensitive);
      if (hit) {
        out.push({ ...entry, contentMatchCount: hit.count, contentSnippet: hit.snippet });
      }
    }
    return out;
  }

  private async matchSingleFileContent(
    path: string,
    contentNeedle: string,
    caseSensitive: boolean,
  ): Promise<{ count: number; snippet: string } | null> {
    try {
      const probe = await this.api.probeFile(path).catch(() => null);
      if (probe && !probe.isText) return null;
      const content = await this.api.readFile(path, 'utf8');
      const text = content.content ?? '';
      if (!text || text.includes('\u0000')) return null;
      const needle = caseSensitive ? contentNeedle : contentNeedle.toLowerCase();
      const lines = text.split(/\r?\n/);
      let count = 0;
      let snippet = '';
      for (let i = 0; i < lines.length; i++) {
        const line = lines[i];
        const hay = caseSensitive ? line : line.toLowerCase();
        let pos = hay.indexOf(needle);
        if (pos >= 0) {
          if (!snippet) {
            const trimmed = line.trim();
            snippet = `L${i + 1}: ${trimmed.length > 140 ? `${trimmed.slice(0, 140)}…` : trimmed}`;
          }
          while (pos >= 0) {
            count++;
            pos = hay.indexOf(needle, pos + Math.max(1, needle.length));
          }
        }
      }
      return count > 0 ? { count, snippet } : null;
    } catch {
      return null;
    }
  }

  private matcher(query: string, caseSensitive: boolean): (entry: FileInfo) => boolean {
    const normalized = caseSensitive ? query : query.toLocaleLowerCase();

    if (query.includes('*') || query.includes('?')) {
      const escaped = query.replace(/[.+^${}()|[\]\\]/g, '\\$&');
      const regex = new RegExp(
        '^' + escaped.replace(/\*/g, '.*').replace(/\?/g, '.') + '$',
        caseSensitive ? '' : 'i',
      );
      return entry => regex.test(entry.name ?? '');
    }

    return entry => {
      const name = caseSensitive ? (entry.name ?? '') : (entry.name ?? '').toLocaleLowerCase();
      const p = caseSensitive ? (entry.path ?? '') : (entry.path ?? '').toLocaleLowerCase();
      return name.includes(normalized) || p.includes(normalized);
    };
  }

  private typeAllowed(
    entry: FileInfo,
    type: NonNullable<DiscoverySearchOptions['type']>,
  ): boolean {
    if (type === 'all') return true;
    if (type === 'folders') return entry.type === FileType.DIRECTORY;
    if (type === 'archives') return entry.type === FileType.ARCHIVE;
    return entry.type === FileType.FILE;
  }

  private pathKey(path: string): string {
    return path.trim().replace(/\\/g, '/').replace(/\/+$/, '').toLocaleLowerCase();
  }

  private throwIfAborted(signal?: AbortSignal): void {
    if (signal?.aborted) {
      throw new DOMException('Search cancelled.', 'AbortError');
    }
  }

  private message(error: unknown): string {
    return error instanceof Error ? error.message : String(error ?? 'Unknown error');
  }
}