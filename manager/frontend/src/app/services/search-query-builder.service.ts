import { Injectable } from '@angular/core';
import { normalizePath } from '../components/address-bar.utils';

/**
 * Comprehensive search query parameters.
 * Replaces the need for custom search dialogs by encoding everything in the URL.
 */
export interface SearchQueryParams {
  /** Base directory to search from */
  path?: string;

  /** Search pattern (glob-style by default, regex if regex=true) */
  pattern?: string;

  /** Whether to search recursively (default: true) */
  recursive?: boolean;

  /** Use regex pattern matching instead of glob (default: false) */
  regex?: boolean;

  /** Case-sensitive pattern matching (default: false) */
  caseSensitive?: boolean;

  /** Comma-separated file types to include: file, directory, symlink (default: file,directory) */
  fileTypes?: string;

  /** Minimum file size in bytes */
  minSize?: number;

  /** Maximum file size in bytes */
  maxSize?: number;

  /** Modified date (inclusive), ISO 8601 format: 2024-01-01 or 2024-01-01T12:00:00Z */
  modifiedAfter?: string;

  /** Modified date (inclusive), ISO 8601 format: 2024-12-31 or 2024-12-31T23:59:59Z */
  modifiedBefore?: string;

  /** Include hidden files (default: false) */
  searchHidden?: boolean;

  /** Search inside file contents (requires backend support) (default: false) */
  contentSearch?: boolean;

  /** Comma-separated file extensions to match: .txt,.md,.log */
  extensions?: string;

  /** Custom metadata filter (backend-specific) */
  metadata?: string;
}

const BOOLEAN_KEYS: ReadonlySet<keyof SearchQueryParams> = new Set([
  'recursive',
  'regex',
  'caseSensitive',
  'searchHidden',
  'contentSearch',
]);

const NUMBER_KEYS: ReadonlySet<keyof SearchQueryParams> = new Set([
  'minSize',
  'maxSize',
]);

/**
 * Per-use fluent builder for constructing search:// URLs with complex filters.
 */
export class SearchQueryBuilder {
  private params: Partial<SearchQueryParams>;

  constructor(initial: Partial<SearchQueryParams> = {}) {
    this.params = { ...initial };
  }

  /** Create a new builder state. */
  reset(): SearchQueryBuilder {
    this.params = {};
    return this;
  }

  /** Set the base search path. */
  setPath(path: string): SearchQueryBuilder {
    if (path) {
      this.params.path = normalizePath(path);
    }
    return this;
  }

  /** Set the search pattern (glob-style or regex). */
  setPattern(pattern: string): SearchQueryBuilder {
    if (pattern) {
      this.params.pattern = pattern;
    }
    return this;
  }

  /** Set whether to search recursively. */
  setRecursive(recursive: boolean): SearchQueryBuilder {
    this.params.recursive = recursive;
    return this;
  }

  /** Enable regex pattern matching. */
  setRegex(useRegex: boolean): SearchQueryBuilder {
    this.params.regex = useRegex;
    return this;
  }

  /** Set case sensitivity. */
  setCaseSensitive(caseSensitive: boolean): SearchQueryBuilder {
    this.params.caseSensitive = caseSensitive;
    return this;
  }

  /** Set file types to include (file, directory, symlink). */
  setFileTypes(types: string): SearchQueryBuilder {
    if (types) {
      this.params.fileTypes = types;
    }
    return this;
  }

  /** Set minimum file size in bytes. */
  setMinSize(bytes: number): SearchQueryBuilder {
    if (bytes > 0) {
      this.params.minSize = bytes;
    }
    return this;
  }

  /** Set maximum file size in bytes. */
  setMaxSize(bytes: number): SearchQueryBuilder {
    if (bytes > 0) {
      this.params.maxSize = bytes;
    }
    return this;
  }

  /** Set modified date lower bound (inclusive). */
  setModifiedAfter(date: string | Date): SearchQueryBuilder {
    if (date) {
      this.params.modifiedAfter = this.formatDate(date);
    }
    return this;
  }

  /** Set modified date upper bound (inclusive). */
  setModifiedBefore(date: string | Date): SearchQueryBuilder {
    if (date) {
      this.params.modifiedBefore = this.formatDate(date);
    }
    return this;
  }

  /** Include hidden files in search. */
  setSearchHidden(hidden: boolean): SearchQueryBuilder {
    this.params.searchHidden = hidden;
    return this;
  }

  /** Enable content search (search inside file contents). */
  setContentSearch(enabled: boolean): SearchQueryBuilder {
    this.params.contentSearch = enabled;
    return this;
  }

  /** Set file extensions to match (e.g., '.txt,.md,.log'). */
  setExtensions(extensions: string): SearchQueryBuilder {
    if (extensions) {
      this.params.extensions = extensions;
    }
    return this;
  }

  /** Set custom metadata filter (backend-specific). */
  setMetadata(metadata: string): SearchQueryBuilder {
    if (metadata) {
      this.params.metadata = metadata;
    }
    return this;
  }

  /** Build the search:// URL. */
  build(): string {
    if (!this.params.pattern) {
      return 'search://';
    }

    const params = new URLSearchParams();
    for (const [key, value] of Object.entries(this.params)) {
      if (value !== null && value !== undefined) {
        params.append(key, String(value));
      }
    }

    return `search://?${params.toString()}`;
  }

  /** Load parameters from an existing search:// URL. */
  fromUrl(url: string): SearchQueryBuilder {
    this.reset();
    const parsed = SearchQueryBuilder.parseParams(url);
    this.params = parsed;
    return this;
  }

  /** Convert current state to human-readable description. */
  toDescription(): string {
    return SearchQueryBuilder.describeParams(this.params);
  }

  /** Get current parameters. */
  getParams(): Readonly<Partial<SearchQueryParams>> {
    return { ...this.params };
  }

  static parseParams(url: string): Partial<SearchQueryParams> {
    try {
      const normalized = normalizePath((url || '').trim().replace(/\\/g, '/'));
      if (!normalized.startsWith('search://')) {
        return {};
      }

      const urlPart = normalized.slice('search://'.length);
      if (!urlPart.startsWith('?')) {
        return {};
      }

      const parsed = new URLSearchParams(urlPart.slice(1));
      const params: Partial<SearchQueryParams> = {};

      parsed.forEach((value, key) => {
        const typedKey = key as keyof SearchQueryParams;

        if (BOOLEAN_KEYS.has(typedKey)) {
          (params[typedKey] as boolean | undefined) = value.toLowerCase() === 'true';
          return;
        }

        if (NUMBER_KEYS.has(typedKey)) {
          const numberValue = Number.parseInt(value, 10);
          if (Number.isFinite(numberValue)) {
            (params[typedKey] as number | undefined) = numberValue;
          }
          return;
        }

        (params[typedKey] as string | undefined) = value;
      });

      return params;
    } catch {
      return {};
    }
  }

  static describeParams(params: Partial<SearchQueryParams>): string {
    const parts: string[] = [];

    if (params.pattern) parts.push(`Pattern: ${params.pattern}`);
    if (params.path) parts.push(`In: ${params.path}`);
    if (params.recursive === false) parts.push('Non-recursive');
    if (params.regex) parts.push('Regex mode');
    if (params.caseSensitive) parts.push('Case-sensitive');
    if (params.fileTypes) parts.push(`Types: ${params.fileTypes}`);
    if (params.extensions) parts.push(`Extensions: ${params.extensions}`);
    if (params.minSize) parts.push(`Min size: ${SearchQueryBuilder.formatBytes(params.minSize)}`);
    if (params.maxSize) parts.push(`Max size: ${SearchQueryBuilder.formatBytes(params.maxSize)}`);
    if (params.modifiedAfter) parts.push(`Modified after: ${params.modifiedAfter}`);
    if (params.modifiedBefore) parts.push(`Modified before: ${params.modifiedBefore}`);
    if (params.searchHidden) parts.push('Include hidden');
    if (params.contentSearch) parts.push('Search content');

    return parts.join(' | ');
  }

  private formatDate(date: string | Date): string {
    if (typeof date === 'string') {
      return date;
    }
    return date.toISOString().split('T')[0];
  }

  private static formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes}B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)}GB`;
  }
}

/**
 * Stateless service facade that creates isolated builder instances.
 */
@Injectable({ providedIn: 'root' })
export class SearchQueryBuilderService {
  create(initial: Partial<SearchQueryParams> = {}): SearchQueryBuilder {
    return new SearchQueryBuilder(initial);
  }

  fromUrl(url: string): SearchQueryBuilder {
    return new SearchQueryBuilder().fromUrl(url);
  }

  build(params: Partial<SearchQueryParams>): string {
    return new SearchQueryBuilder(params).build();
  }

  parse(url: string): Partial<SearchQueryParams> {
    return SearchQueryBuilder.parseParams(url);
  }

  toDescription(params: Partial<SearchQueryParams>): string {
    return SearchQueryBuilder.describeParams(params);
  }
}