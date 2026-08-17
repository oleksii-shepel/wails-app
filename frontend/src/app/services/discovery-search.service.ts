import { Injectable } from '@angular/core';

import { ApiService } from './api.service';
import { PanelSourceRegistryService } from './panel-source-registry.service';
import { FileInfo, FileType } from '../shared/protocol';

export interface DiscoverySearchOptions {
  readonly recursive?: boolean;
  readonly showHidden?: boolean;
  readonly maxResults?: number;
  readonly maxVisitedDirectories?: number;
  readonly maxDepth?: number;
  readonly type?: 'all' | 'files' | 'folders' | 'archives';
  readonly signal?: AbortSignal;
  readonly onProgress?: (progress: DiscoverySearchProgress) => void;
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
  readonly entries: readonly FileInfo[];
  readonly visitedDirectories: number;
  readonly truncated: boolean;
  readonly warnings: readonly string[];
}

@Injectable({ providedIn: 'root' })
export class DiscoverySearchService {
  constructor(
    private readonly api: ApiService,
    private readonly panelSources: PanelSourceRegistryService,
  ) {}

  async search(
    rootPath: string,
    query: string,
    options: DiscoverySearchOptions = {},
  ): Promise<DiscoverySearchResult> {
    const root = rootPath.trim();
    const needle = query.trim();

    if (!root || !needle) {
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
    if (resolved.source.kind === 'filesystem' &&
        /^([a-z]:[\\/]|\/|\\\\)/i.test(root)) {
      try {
        const result = await this.api.search(
          root,
          needle,
          options.recursive !== false,
        );

        const matches = (result.matches ?? [])
          .filter(entry => this.typeAllowed(entry, options.type ?? 'all'))
          .slice(0, Math.max(1, options.maxResults ?? 500));

        return {
          rootPath: root,
          sourceId: resolved.location.sourceId,
          sourceKind: resolved.source.kind,
          entries: matches,
          visitedDirectories: 1,
          truncated: (result.matches?.length ?? 0) > matches.length,
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
    const maxResults = Math.max(1, options.maxResults ?? 500);
    const maxVisited = Math.max(1, options.maxVisitedDirectories ?? 5000);
    const maxDepth = Math.max(0, options.maxDepth ?? 64);
    const matcher = this.matcher(query);
    const matches: FileInfo[] = [];
    const warnings: string[] = [];
    const visited = new Set<string>();
    const queue: Array<{ path: string; depth: number }> = [{ path: root, depth: 0 }];
    let truncated = false;

    while (queue.length) {
      this.throwIfAborted(options.signal);

      if (visited.size >= maxVisited || matches.length >= maxResults) {
        truncated = true;
        break;
      }

      const current = queue.shift()!;
      const key = this.pathKey(current.path);
      if (visited.has(key)) continue;
      visited.add(key);

      let listing;
      try {
        listing = await this.panelSources.list(
          current.path,
          !!options.showHidden,
        );
      } catch (error) {
        warnings.push(`${current.path}: ${this.message(error)}`);
        continue;
      }

      for (const entry of listing.entries ?? []) {
        this.throwIfAborted(options.signal);
        if (!options.showHidden && entry.isHidden) continue;

        if (matcher(entry) &&
            this.typeAllowed(entry, options.type ?? 'all')) {
          matches.push({ ...entry });
          if (matches.length >= maxResults) {
            truncated = true;
            break;
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

      options.onProgress?.({
        sourceId,
        visitedDirectories: visited.size,
        matchedEntries: matches.length,
        currentPath: current.path,
      });

      if (!recursive) break;
    }

    return {
      rootPath: root,
      sourceId,
      sourceKind,
      entries: matches,
      visitedDirectories: visited.size,
      truncated,
      warnings,
    };
  }

  private matcher(query: string): (entry: FileInfo) => boolean {
    const normalized = query.toLocaleLowerCase();

    if (query.includes('*') || query.includes('?')) {
      const escaped = query.replace(/[.+^${}()|[\]\\]/g, '\\$&');
      const regex = new RegExp(
        '^' + escaped.replace(/\*/g, '.*').replace(/\?/g, '.') + '$',
        'i',
      );
      return entry => regex.test(entry.name ?? '');
    }

    return entry =>
      (entry.name ?? '').toLocaleLowerCase().includes(normalized) ||
      (entry.path ?? '').toLocaleLowerCase().includes(normalized);
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
