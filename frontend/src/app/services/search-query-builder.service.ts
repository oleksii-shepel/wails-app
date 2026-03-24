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

/**
 * Builder service for constructing search:// URLs with complex filters.
 * Eliminates the need for custom search dialogs by encoding filters in URL parameters.
 *
 * Usage:
 *   builder
 *     .setPath('C:/Users')
 *     .setPattern('*.log')
 *     .setMinSize(1024 * 1024) // 1MB
 *     .setModifiedAfter('2024-01-01')
 *     .build()
 *
 *   // Returns: search://?path=C%3A%2FUsers&pattern=*.log&minSize=1048576&modifiedAfter=2024-01-01
 */
@Injectable({ providedIn: 'root' })
export class SearchQueryBuilderService {
  private params: Partial<SearchQueryParams> = {};

  /**
   * Create a new builder (or reset existing).
   */
  reset(): SearchQueryBuilderService {
    this.params = {};
    return this;
  }

  /**
   * Set the base search path.
   */
  setPath(path: string): SearchQueryBuilderService {
    if (path) {
      this.params.path = normalizePath(path);
    }
    return this;
  }

  /**
   * Set the search pattern (glob-style or regex).
   */
  setPattern(pattern: string): SearchQueryBuilderService {
    if (pattern) {
      this.params.pattern = pattern;
    }
    return this;
  }

  /**
   * Set whether to search recursively.
   */
  setRecursive(recursive: boolean): SearchQueryBuilderService {
    this.params.recursive = recursive;
    return this;
  }

  /**
   * Enable regex pattern matching.
   */
  setRegex(useRegex: boolean): SearchQueryBuilderService {
    this.params.regex = useRegex;
    return this;
  }

  /**
   * Set case sensitivity.
   */
  setCaseSensitive(caseSensitive: boolean): SearchQueryBuilderService {
    this.params.caseSensitive = caseSensitive;
    return this;
  }

  /**
   * Set file types to include (file, directory, symlink).
   * Pass multiple: 'file,directory'
   */
  setFileTypes(types: string): SearchQueryBuilderService {
    if (types) {
      this.params.fileTypes = types;
    }
    return this;
  }

  /**
   * Set minimum file size in bytes.
   */
  setMinSize(bytes: number): SearchQueryBuilderService {
    if (bytes > 0) {
      this.params.minSize = bytes;
    }
    return this;
  }

  /**
   * Set maximum file size in bytes.
   */
  setMaxSize(bytes: number): SearchQueryBuilderService {
    if (bytes > 0) {
      this.params.maxSize = bytes;
    }
    return this;
  }

  /**
   * Set modified date lower bound (inclusive).
   * Format: ISO 8601 (2024-01-01 or 2024-01-01T12:00:00Z)
   */
  setModifiedAfter(date: string | Date): SearchQueryBuilderService {
    if (date) {
      this.params.modifiedAfter = this.formatDate(date);
    }
    return this;
  }

  /**
   * Set modified date upper bound (inclusive).
   * Format: ISO 8601 (2024-12-31 or 2024-12-31T23:59:59Z)
   */
  setModifiedBefore(date: string | Date): SearchQueryBuilderService {
    if (date) {
      this.params.modifiedBefore = this.formatDate(date);
    }
    return this;
  }

  /**
   * Include hidden files in search.
   */
  setSearchHidden(hidden: boolean): SearchQueryBuilderService {
    this.params.searchHidden = hidden;
    return this;
  }

  /**
   * Enable content search (search inside file contents).
   */
  setContentSearch(enabled: boolean): SearchQueryBuilderService {
    this.params.contentSearch = enabled;
    return this;
  }

  /**
   * Set file extensions to match (e.g., '.txt,.md,.log').
   */
  setExtensions(extensions: string): SearchQueryBuilderService {
    if (extensions) {
      this.params.extensions = extensions;
    }
    return this;
  }

  /**
   * Set custom metadata filter (backend-specific).
   */
  setMetadata(metadata: string): SearchQueryBuilderService {
    if (metadata) {
      this.params.metadata = metadata;
    }
    return this;
  }

  /**
   * Build the search:// URL.
   */
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

  /**
   * Load parameters from an existing search:// URL.
   */
  fromUrl(url: string): SearchQueryBuilderService {
    this.reset();
    try {
      const normalized = normalizePath((url || '').trim().replace(/\\/g, '/'));
      if (!normalized.startsWith('search://')) {
        return this;
      }

      const urlPart = normalized.slice('search://'.length);
      if (!urlPart.startsWith('?')) {
        return this;
      }

      const parsed = new URLSearchParams(urlPart.slice(1));
      this.params = {};

      parsed.forEach((value, key) => {
        if (key === 'recursive' || key === 'regex' || key === 'caseSensitive' || key === 'searchHidden' || key === 'contentSearch') {
          (this.params as any)[key] = value.toLowerCase() === 'true';
        } else if (key === 'minSize' || key === 'maxSize') {
          (this.params as any)[key] = parseInt(value, 10);
        } else {
          (this.params as any)[key] = value;
        }
      });
    } catch {
      this.reset();
    }

    return this;
  }

  /**
   * Convert URL to human-readable description.
   */
  toDescription(): string {
    const parts: string[] = [];

    if (this.params.pattern) {
      parts.push(`Pattern: ${this.params.pattern}`);
    }

    if (this.params.path) {
      parts.push(`In: ${this.params.path}`);
    }

    if (this.params.recursive === false) {
      parts.push('Non-recursive');
    }

    if (this.params.regex) {
      parts.push('Regex mode');
    }

    if (this.params.caseSensitive) {
      parts.push('Case-sensitive');
    }

    if (this.params.fileTypes) {
      parts.push(`Types: ${this.params.fileTypes}`);
    }

    if (this.params.extensions) {
      parts.push(`Extensions: ${this.params.extensions}`);
    }

    if (this.params.minSize) {
      parts.push(`Min size: ${this.formatBytes(this.params.minSize)}`);
    }

    if (this.params.maxSize) {
      parts.push(`Max size: ${this.formatBytes(this.params.maxSize)}`);
    }

    if (this.params.modifiedAfter) {
      parts.push(`Modified after: ${this.params.modifiedAfter}`);
    }

    if (this.params.modifiedBefore) {
      parts.push(`Modified before: ${this.params.modifiedBefore}`);
    }

    if (this.params.searchHidden) {
      parts.push('Include hidden');
    }

    if (this.params.contentSearch) {
      parts.push('Search content');
    }

    return parts.join(' | ');
  }

  /**
   * Get current parameters.
   */
  getParams(): Readonly<Partial<SearchQueryParams>> {
    return { ...this.params };
  }

  private formatDate(date: string | Date): string {
    if (typeof date === 'string') {
      return date;
    }
    return date.toISOString().split('T')[0];
  }

  private formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes}B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
    if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
    return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)}GB`;
  }
}
