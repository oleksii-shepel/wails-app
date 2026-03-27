import { Injectable } from '@angular/core';
import { DirectorySourceService } from './directory-source.service';
import { PathSuggestion } from './path-history.service';
import { normalizePath } from '../components/address-bar.utils';
import { FileType, FileInfo } from '../shared/protocol';

/**
 * Provider interface for scheme-specific autocompletion.
 * Each scheme (drive, ftp, search, registry, etc.) can implement custom logic.
 */
export interface SchemeAutocompleteProvider {
  readonly scheme: string;

  /**
   * Determine if this provider should handle the given query.
   */
  canHandle(query: string): boolean;

  /**
   * Determine if the given path should fetch children for autocompletion.
   */
  shouldFetch(query: string): boolean;

  /**
   * Split a query into context (directory path) and prefix to match.
   */
  splitContext(query: string): { dirPath: string; prefix: string };

  /**
   * Fetch suggestions for the given directory and prefix.
   */
  fetchSuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]>;

  /**
   * Stream suggestions with batching support.
   */
  streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]>;

  /**
   * Check if this provider supports streaming.
   */
  supportsStreaming(path: string): boolean;

  /**
   * Get the root path.
   */
  getRootPath(path: string): string | null;

  /**
   * Join a directory path and child name.
   */
  joinChildPath(basePath: string, childName: string): string;
}

/**
 * Default filesystem provider (covers filesystem://, local paths, Windows paths).
 */
class FilesystemAutocompleteProvider implements SchemeAutocompleteProvider {
  scheme = 'filesystem';

  constructor(private readonly directorySource: DirectorySourceService) {}

  canHandle(query: string): boolean {
    const scheme = this.extractScheme(query);
    // Handle only filesystem and local (no explicit scheme)
    return !scheme || scheme === 'filesystem';
  }

  shouldFetch(query: string): boolean {
    const q = (query || '').trim();
    if (!q) return false;

    const n = normalizePath(q);
    if (!n) return false;

    // Avoid listing the full filesystem root on short inputs.
    if (n === '/') return false;
    if (n.toLowerCase() === 'filesystem://') return false;

    // Any slash-based path, or Windows drive prefix
    if (n.includes('/')) return true;
    if (/^[A-Za-z]:/.test(n)) return true;

    return false;
  }

  splitContext(query: string): { dirPath: string; prefix: string } {
    const q = normalizePath((query || '').replace(/\\/g, '/'));
    if (!q) return { dirPath: '/', prefix: '' };

    // Handle drive roots (e.g., "C:", "filesystem://X")
    const driveRootMatch = /^filesystem:\/\/[a-z]$/i.exec(q);
    if (driveRootMatch) return { dirPath: `${q}/`, prefix: '' };

    // If path ends with slash, it's a directory
    if (q.endsWith('/')) return { dirPath: q, prefix: '' };

    // Find the last slash to split directory and prefix
    const lastSlash = q.lastIndexOf('/');
    if (lastSlash >= 0) {
      return { dirPath: q.slice(0, lastSlash + 1), prefix: q.slice(lastSlash + 1) };
    }

    // Handle bare Windows drive letter (e.g., "C:")
    if (/^[A-Za-z]:$/.test(q)) return { dirPath: `${q}/`, prefix: '' };

    // Single path component – root context
    return { dirPath: '/', prefix: q };
  }

  async fetchSuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]> {
    try {
      const listing = await this.directorySource.listDirectory(dirPath, false);
      return this.filterAndFormatSuggestions(listing.entries, prefix);
    } catch {
      return [];
    }
  }

  async streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    const suggestions = await this.fetchSuggestions(dirPath, prefix);
    onBatch(suggestions);
    return suggestions;
  }

  supportsStreaming(path: string): boolean {
    return this.directorySource.supportsStreaming(path);
  }

  getRootPath(path: string): string | null {
    return this.directorySource.getRootPath(path);
  }

  joinChildPath(basePath: string, childName: string): string {
    const trimmed = (basePath || '/').replace(/\/+$/, '');
    const joined = `${trimmed}/${childName}`.replace(/\\/g, '/');
    if (joined.startsWith('//')) {
      return `//${joined.slice(2).replace(/\/{2,}/g, '/')}`;
    }
    return joined.replace(/\/{2,}/g, '/');
  }

  private filterAndFormatSuggestions(entries: FileInfo[] = [], prefix: string): PathSuggestion[] {
    const prefixLower = prefix.toLowerCase();
    const accumulated: PathSuggestion[] = [];
    const seenNames = new Set<string>();

    for (const entry of entries) {
      if (entry.type !== FileType.DIRECTORY) continue;
      if (prefixLower && !entry.name.toLowerCase().startsWith(prefixLower)) continue;
      if (seenNames.has(entry.name)) continue;
      seenNames.add(entry.name);

      accumulated.push({
        path: normalizePath(entry.path),
        displayName: entry.name,
        matchType: 'directory',
        score: 100,
        exists: true,
      });
    }

    accumulated.sort((a, b) => a.path.localeCompare(b.path));
    return accumulated;
  }

  private extractScheme(query: string): string | null {
    const normalized = normalizePath((query || '').trim().replace(/\\/g, '/'));
    const schemeMatch = /^([a-z][a-z0-9+.-]*):\/\//i.exec(normalized);
    return schemeMatch ? schemeMatch[1].toLowerCase() : null;
  }
}

/**
 * Search scheme provider: supports comprehensive search queries via URL parameters.
 * Replaces need for custom search dialogs by encoding all filters in the URL.
 *
 * Supported parameters:
 * - path: Base directory to search from
 * - pattern: Search pattern (glob-style or regex)
 * - recursive: Whether to search recursively (default: true)
 * - regex: Use regex pattern matching (default: false)
 * - caseSensitive: Case-sensitive matching (default: false)
 * - fileTypes: Comma-separated file types to include (file,directory,symlink)
 * - minSize: Minimum file size in bytes
 * - maxSize: Maximum file size in bytes
 * - modifiedAfter: ISO 8601 date string (inclusive)
 * - modifiedBefore: ISO 8601 date string (inclusive)
 * - searchHidden: Include hidden files (default: false)
 * - contentSearch: Search inside file contents (default: false)
 * - extensions: Comma-separated file extensions to match (.txt,.md)
 *
 * Example: search://?path=filesystem://c&pattern=*.log&minSize=1000&modifiedAfter=2024-01-01
 */
class SearchAutocompleteProvider implements SchemeAutocompleteProvider {
  scheme = 'search';

  canHandle(query: string): boolean {
    const normalized = normalizePath((query || '').trim().replace(/\\/g, '/'));
    return /^search:\/\//i.test(normalized);
  }

  shouldFetch(query: string): boolean {
    const q = (query || '').trim();
    if (!q) return false;
    const n = normalizePath(q);
    // Don't fetch just for "search://" - need actual search params
    return /^search:\/\/\?/.test(n);
  }

  splitContext(query: string): { dirPath: string; prefix: string } {
    const q = normalizePath((query || '').replace(/\\/g, '/'));
    if (!q || q.toLowerCase() === 'search://') {
      return { dirPath: 'search://', prefix: '' };
    }

    // Validate search query parameters when present.
    try {
      const urlPart = q.slice('search://'.length);
      if (!urlPart.startsWith('?')) {
        return { dirPath: 'search://', prefix: '' };
      }

      void new URLSearchParams(urlPart.slice(1));
      
      // For search://?, we don't really have a "prefix" like filesystem
      // The entire query is the "context"
      return { dirPath: q, prefix: '' };
    } catch {
      return { dirPath: 'search://', prefix: '' };
    }
  }

  async fetchSuggestions(dirPath: string, _prefix: string): Promise<PathSuggestion[]> {
    // Search scheme doesn't provide directory listings in the traditional sense.
    // Suggestions could include:
    // 1. Common search filters (by extension, by size, etc.)
    // 2. Previously used search patterns
    // 3. Saved searches
    return this.getSearchFilterSuggestions(dirPath);
  }

  async streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    const suggestions = await this.fetchSuggestions(dirPath, prefix);
    onBatch(suggestions);
    return suggestions;
  }

  supportsStreaming(_path: string): boolean {
    return true;
  }

  getRootPath(_path: string): string | null {
    return 'search://';
  }

  joinChildPath(basePath: string, childName: string): string {
    const value = (childName || '').trim();
    if (!value) return basePath || 'search://';

    const normalized = normalizePath((basePath || 'search://').replace(/\\/g, '/'));
    if (!normalized.toLowerCase().startsWith('search://')) {
      return normalized;
    }

    const urlPart = normalized.slice('search://'.length);
    const params = new URLSearchParams(urlPart.startsWith('?') ? urlPart.slice(1) : '');
    params.set('pattern', value);
    return `search://?${params.toString()}`;
  }

  /**
   * Parse search:// URL parameters into a structured object.
   */
  private parseSearchParams(urlPath: string): Record<string, string | null> {
    try {
      const urlPart = urlPath.slice('search://'.length);
      if (!urlPart.startsWith('?')) return {};
      const params = new URLSearchParams(urlPart.slice(1));
      const result: Record<string, string | null> = {};
      params.forEach((value, key) => {
        result[key] = value;
      });
      return result;
    } catch {
      return {};
    }
  }

  /**
   * Build search query with filters for suggestion purposes.
   * Could suggest common filter combinations or saved searches.
   */
  private getSearchFilterSuggestions(dirPath: string): PathSuggestion[] {
    const suggestions: PathSuggestion[] = [];

    // Parse current query if provided
    const params = this.parseSearchParams(dirPath);
    const hasPattern = !!params['pattern'];
    const currentUrl = dirPath;

    // 1. Suggest search parameters if user is typing them
    if (currentUrl.endsWith('&') || currentUrl.endsWith('?')) {
      const paramSuggestions = [
        { key: 'pattern=', label: 'pattern (glob/regex)', score: 95 },
        { key: 'path=', label: 'path (base directory)', score: 90 },
        { key: 'minSize=', label: 'minSize (bytes)', score: 85 },
        { key: 'maxSize=', label: 'maxSize (bytes)', score: 85 },
        { key: 'extensions=', label: 'extensions (.txt,.md)', score: 80 },
        { key: 'modifiedAfter=', label: 'modifiedAfter (YYYY-MM-DD)', score: 75 },
        { key: 'fileTypes=', label: 'fileTypes (file,directory)', score: 70 },
        { key: 'recursive=', label: 'recursive (true/false)', score: 65 },
        { key: 'regex=', label: 'regex (true/false)', score: 60 },
      ];

      for (const p of paramSuggestions) {
        suggestions.push({
          path: currentUrl + p.key,
          displayName: `Param: ${p.label}`,
          matchType: 'directory',
          score: p.score,
          exists: true,
        });
      }
    }

    // Suggest common values for specific parameters being typed
    if (currentUrl.endsWith('fileTypes=')) {
      const types = ['file', 'directory', 'file,directory', 'symlink'];
      types.forEach((t, i) => {
        suggestions.push({
          path: currentUrl + t,
          displayName: `Type: ${t}`,
          matchType: 'directory',
          score: 100 - i,
          exists: true,
        });
      });
    }

    if (currentUrl.endsWith('recursive=') || currentUrl.endsWith('regex=') || currentUrl.endsWith('caseSensitive=') || currentUrl.endsWith('searchHidden=') || currentUrl.endsWith('contentSearch=')) {
      ['true', 'false'].forEach((v, i) => {
        suggestions.push({
          path: currentUrl + v,
          displayName: v,
          matchType: 'directory',
          score: 100 - i,
          exists: true,
        });
      });
    }

    // 2. Original filter logic
    // Suggest common filter combinations if user hasn't added them yet
    if (hasPattern && !params['extensions']) {
      suggestions.push({
        path: this.buildSearchURL(params, { extensions: '.log,.txt,.md' }),
        displayName: 'Common docs (.log, .txt, .md)',
        matchType: 'directory',
        score: 80,
        exists: true,
      });

      suggestions.push({
        path: this.buildSearchURL(params, { extensions: '.exe,.dll,.sys' }),
        displayName: 'Binary files (.exe, .dll, .sys)',
        matchType: 'directory',
        score: 75,
        exists: true,
      });

      suggestions.push({
        path: this.buildSearchURL(params, { extensions: '.jpg,.png,.gif,.bmp' }),
        displayName: 'Images (.jpg, .png, .gif, .bmp)',
        matchType: 'directory',
        score: 75,
        exists: true,
      });
    }

    // Suggest size filters if not present
    if (hasPattern && !params['minSize'] && !params['maxSize']) {
      suggestions.push({
        path: this.buildSearchURL(params, { minSize: '1048576' }), // 1MB
        displayName: 'Larger than 1MB',
        matchType: 'directory',
        score: 70,
        exists: true,
      });

      suggestions.push({
        path: this.buildSearchURL(params, { maxSize: '1024' }), // 1KB
        displayName: 'Smaller than 1KB',
        matchType: 'directory',
        score: 70,
        exists: true,
      });
    }

    return suggestions;
  }

  /**
   * Build a search:// URL with merged parameters.
   */
  private buildSearchURL(
    currentParams: Record<string, any>,
    updates: Record<string, string | number | boolean | null>,
  ): string {
    const merged = { ...currentParams, ...updates };
    const params = new URLSearchParams();

    for (const [key, value] of Object.entries(merged)) {
      if (value !== null && value !== undefined) {
        params.append(key, String(value));
      }
    }

    return `search://?${params.toString()}`;
  }
}

/**
 * Registry scheme provider: Windows registry paths.
 */
class RegistryAutocompleteProvider implements SchemeAutocompleteProvider {
  scheme = 'registry';

  constructor(private readonly directorySource: DirectorySourceService) {}

  canHandle(query: string): boolean {
    const normalized = normalizePath((query || '').trim().replace(/\\/g, '/'));
    return /^registry:\/\//i.test(normalized);
  }

  shouldFetch(query: string): boolean {
    const q = (query || '').trim();
    if (!q) return false;
    const n = normalizePath(q);
    return /^registry:\/\/(?:\/.*)?$/i.test(n) && n !== 'registry://';
  }

  splitContext(query: string): { dirPath: string; prefix: string } {
    const q = normalizePath((query || '').replace(/\\/g, '/'));
    if (!q || q.toLowerCase() === 'registry://') {
      return { dirPath: 'registry://', prefix: '' };
    }

    if (q.endsWith('/')) {
      return { dirPath: q, prefix: '' };
    }

    const lastSlash = q.lastIndexOf('/');
    if (lastSlash > 'registry://'.length - 1) {
      return { dirPath: q.slice(0, lastSlash + 1), prefix: q.slice(lastSlash + 1) };
    }

    return { dirPath: 'registry://', prefix: q.slice('registry://'.length) };
  }

  async fetchSuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]> {
    return this.getRegistrySuggestions(dirPath, prefix);
  }

  async streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    const suggestions = await this.fetchSuggestions(dirPath, prefix);
    onBatch(suggestions);
    return suggestions;
  }

  supportsStreaming(_path: string): boolean {
    return false;
  }

  getRootPath(_path: string): string | null {
    return 'registry://';
  }

  joinChildPath(basePath: string, childName: string): string {
    const trimmed = (basePath || 'registry://').replace(/\/+$/, '');
    if (trimmed === 'registry://') {
      return `${trimmed}${childName}`;
    }
    return `${trimmed}/${childName}`;
  }

  private async getRegistrySuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]> {
    try {
      const listing = await this.directorySource.listDirectory(dirPath, false);
      return this.filterAndFormatSuggestions(listing.entries, prefix);
    } catch {
      return [];
    }
  }

  private filterAndFormatSuggestions(entries: FileInfo[] = [], prefix: string): PathSuggestion[] {
    const prefixLower = prefix.toLowerCase();
    const accumulated: PathSuggestion[] = [];
    const seenNames = new Set<string>();

    for (const entry of entries) {
      // For registry, include both keys and values that are "directory-like"
      if (entry.type !== FileType.DIRECTORY) continue;
      if (prefixLower && !entry.name.toLowerCase().startsWith(prefixLower)) continue;
      if (seenNames.has(entry.name)) continue;
      seenNames.add(entry.name);

      accumulated.push({
        path: normalizePath(entry.path),
        displayName: entry.name,
        matchType: 'directory',
        score: 100,
        exists: true,
      });
    }

    accumulated.sort((a, b) => a.path.localeCompare(b.path));
    return accumulated;
  }
}

/**
 * FTP scheme provider: remote FTP connections.
 */
class FtpAutocompleteProvider implements SchemeAutocompleteProvider {
  scheme = 'ftp';

  constructor(private readonly directorySource: DirectorySourceService) {}

  canHandle(query: string): boolean {
    const normalized = normalizePath((query || '').trim().replace(/\\/g, '/'));
    return /^ftp:\/\//i.test(normalized);
  }

  shouldFetch(query: string): boolean {
    const q = (query || '').trim();
    if (!q) return false;
    const n = normalizePath(q);
    // Require a bit more context for FTP to avoid excessive requests
    return /^ftp:\/\/[^/]*(?:\/.*)?$/i.test(n) && n !== 'ftp://';
  }

  splitContext(query: string): { dirPath: string; prefix: string } {
    const q = normalizePath((query || '').replace(/\\/g, '/'));
    if (!q) return { dirPath: 'ftp://', prefix: '' };

    if (q.toLowerCase() === 'ftp://') {
      return { dirPath: 'ftp://', prefix: '' };
    }

    if (q.endsWith('/')) return { dirPath: q, prefix: '' };

    const lastSlash = q.lastIndexOf('/');
    if (lastSlash > 'ftp://'.length - 1) {
      return { dirPath: q.slice(0, lastSlash + 1), prefix: q.slice(lastSlash + 1) };
    }

    return { dirPath: 'ftp://', prefix: q.slice('ftp://'.length) };
  }

  async fetchSuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]> {
    try {
      const listing = await this.directorySource.listDirectory(dirPath, false);
      return this.filterAndFormatSuggestions(listing.entries, prefix);
    } catch {
      return [];
    }
  }

  async streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    const suggestions = await this.fetchSuggestions(dirPath, prefix);
    onBatch(suggestions);
    return suggestions;
  }

  supportsStreaming(path: string): boolean {
    return this.directorySource.supportsStreaming(path);
  }

  getRootPath(path: string): string | null {
    return this.directorySource.getRootPath(path);
  }

  joinChildPath(basePath: string, childName: string): string {
    const trimmed = (basePath || 'ftp://').replace(/\/+$/, '');
    return `${trimmed}/${childName}`;
  }

  private filterAndFormatSuggestions(entries: FileInfo[] = [], prefix: string): PathSuggestion[] {
    const prefixLower = prefix.toLowerCase();
    const accumulated: PathSuggestion[] = [];
    const seenNames = new Set<string>();

    for (const entry of entries) {
      if (entry.type !== FileType.DIRECTORY) continue;
      if (prefixLower && !entry.name.toLowerCase().startsWith(prefixLower)) continue;
      if (seenNames.has(entry.name)) continue;
      seenNames.add(entry.name);

      accumulated.push({
        path: normalizePath(entry.path),
        displayName: entry.name,
        matchType: 'directory',
        score: 100,
        exists: true,
      });
    }

    accumulated.sort((a, b) => a.path.localeCompare(b.path));
    return accumulated;
  }
}

/**
 * SMB scheme provider: Windows file sharing (SMB/CIFS).
 */
class SmbAutocompleteProvider implements SchemeAutocompleteProvider {
  scheme = 'smb';

  constructor(private readonly directorySource: DirectorySourceService) {}

  canHandle(query: string): boolean {
    const normalized = normalizePath((query || '').trim().replace(/\\/g, '/'));
    return /^smb:\/\//i.test(normalized);
  }

  shouldFetch(query: string): boolean {
    const q = (query || '').trim();
    if (!q) return false;
    const n = normalizePath(q);
    return /^smb:\/\/[^/]*(?:\/.*)?$/i.test(n) && n !== 'smb://';
  }

  splitContext(query: string): { dirPath: string; prefix: string } {
    const q = normalizePath((query || '').replace(/\\/g, '/'));
    if (!q) return { dirPath: 'smb://', prefix: '' };

    if (q.toLowerCase() === 'smb://') {
      return { dirPath: 'smb://', prefix: '' };
    }

    if (q.endsWith('/')) return { dirPath: q, prefix: '' };

    const lastSlash = q.lastIndexOf('/');
    if (lastSlash > 'smb://'.length - 1) {
      return { dirPath: q.slice(0, lastSlash + 1), prefix: q.slice(lastSlash + 1) };
    }

    return { dirPath: 'smb://', prefix: q.slice('smb://'.length) };
  }

  async fetchSuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]> {
    try {
      const listing = await this.directorySource.listDirectory(dirPath, false);
      return this.filterAndFormatSuggestions(listing.entries, prefix);
    } catch {
      return [];
    }
  }

  async streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    const suggestions = await this.fetchSuggestions(dirPath, prefix);
    onBatch(suggestions);
    return suggestions;
  }

  supportsStreaming(path: string): boolean {
    return this.directorySource.supportsStreaming(path);
  }

  getRootPath(path: string): string | null {
    return this.directorySource.getRootPath(path);
  }

  joinChildPath(basePath: string, childName: string): string {
    const trimmed = (basePath || 'smb://').replace(/\/+$/, '');
    return `${trimmed}/${childName}`;
  }

  private filterAndFormatSuggestions(entries: FileInfo[] = [], prefix: string): PathSuggestion[] {
    const prefixLower = prefix.toLowerCase();
    const accumulated: PathSuggestion[] = [];
    const seenNames = new Set<string>();

    for (const entry of entries) {
      if (entry.type !== FileType.DIRECTORY) continue;
      if (prefixLower && !entry.name.toLowerCase().startsWith(prefixLower)) continue;
      if (seenNames.has(entry.name)) continue;
      seenNames.add(entry.name);

      accumulated.push({
        path: normalizePath(entry.path),
        displayName: entry.name,
        matchType: 'directory',
        score: 100,
        exists: true,
      });
    }

    accumulated.sort((a, b) => a.path.localeCompare(b.path));
    return accumulated;
  }
}

/**
 * SFTP scheme provider: SSH file transfer protocol.
 */
class SftpAutocompleteProvider implements SchemeAutocompleteProvider {
  scheme = 'sftp';

  constructor(private readonly directorySource: DirectorySourceService) {}

  canHandle(query: string): boolean {
    const normalized = normalizePath((query || '').trim().replace(/\\/g, '/'));
    return /^sftp:\/\//i.test(normalized);
  }

  shouldFetch(query: string): boolean {
    const q = (query || '').trim();
    if (!q) return false;
    const n = normalizePath(q);
    return /^sftp:\/\/[^/]*(?:\/.*)?$/i.test(n) && n !== 'sftp://';
  }

  splitContext(query: string): { dirPath: string; prefix: string } {
    const q = normalizePath((query || '').replace(/\\/g, '/'));
    if (!q) return { dirPath: 'sftp://', prefix: '' };

    if (q.toLowerCase() === 'sftp://') {
      return { dirPath: 'sftp://', prefix: '' };
    }

    if (q.endsWith('/')) return { dirPath: q, prefix: '' };

    const lastSlash = q.lastIndexOf('/');
    if (lastSlash > 'sftp://'.length - 1) {
      return { dirPath: q.slice(0, lastSlash + 1), prefix: q.slice(lastSlash + 1) };
    }

    return { dirPath: 'sftp://', prefix: q.slice('sftp://'.length) };
  }

  async fetchSuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]> {
    try {
      const listing = await this.directorySource.listDirectory(dirPath, false);
      return this.filterAndFormatSuggestions(listing.entries, prefix);
    } catch {
      return [];
    }
  }

  async streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    const suggestions = await this.fetchSuggestions(dirPath, prefix);
    onBatch(suggestions);
    return suggestions;
  }

  supportsStreaming(path: string): boolean {
    return this.directorySource.supportsStreaming(path);
  }

  getRootPath(path: string): string | null {
    return this.directorySource.getRootPath(path);
  }

  joinChildPath(basePath: string, childName: string): string {
    const trimmed = (basePath || 'sftp://').replace(/\/+$/, '');
    return `${trimmed}/${childName}`;
  }

  private filterAndFormatSuggestions(entries: FileInfo[] = [], prefix: string): PathSuggestion[] {
    const prefixLower = prefix.toLowerCase();
    const accumulated: PathSuggestion[] = [];
    const seenNames = new Set<string>();

    for (const entry of entries) {
      if (entry.type !== FileType.DIRECTORY) continue;
      if (prefixLower && !entry.name.toLowerCase().startsWith(prefixLower)) continue;
      if (seenNames.has(entry.name)) continue;
      seenNames.add(entry.name);

      accumulated.push({
        path: normalizePath(entry.path),
        displayName: entry.name,
        matchType: 'directory',
        score: 100,
        exists: true,
      });
    }

    accumulated.sort((a, b) => a.path.localeCompare(b.path));
    return accumulated;
  }
}

/**
 * WebDAV scheme provider: WebDAV protocol.
 */
class WebdavAutocompleteProvider implements SchemeAutocompleteProvider {
  scheme = 'webdav';

  constructor(private readonly directorySource: DirectorySourceService) {}

  canHandle(query: string): boolean {
    const normalized = normalizePath((query || '').trim().replace(/\\/g, '/'));
    return /^webdav:\/\//i.test(normalized);
  }

  shouldFetch(query: string): boolean {
    const q = (query || '').trim();
    if (!q) return false;
    const n = normalizePath(q);
    return /^webdav:\/\/[^/]*(?:\/.*)?$/i.test(n) && n !== 'webdav://';
  }

  splitContext(query: string): { dirPath: string; prefix: string } {
    const q = normalizePath((query || '').replace(/\\/g, '/'));
    if (!q) return { dirPath: 'webdav://', prefix: '' };

    if (q.toLowerCase() === 'webdav://') {
      return { dirPath: 'webdav://', prefix: '' };
    }

    if (q.endsWith('/')) return { dirPath: q, prefix: '' };

    const lastSlash = q.lastIndexOf('/');
    if (lastSlash > 'webdav://'.length - 1) {
      return { dirPath: q.slice(0, lastSlash + 1), prefix: q.slice(lastSlash + 1) };
    }

    return { dirPath: 'webdav://', prefix: q.slice('webdav://'.length) };
  }

  async fetchSuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]> {
    try {
      const listing = await this.directorySource.listDirectory(dirPath, false);
      return this.filterAndFormatSuggestions(listing.entries, prefix);
    } catch {
      return [];
    }
  }

  async streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    const suggestions = await this.fetchSuggestions(dirPath, prefix);
    onBatch(suggestions);
    return suggestions;
  }

  supportsStreaming(path: string): boolean {
    return this.directorySource.supportsStreaming(path);
  }

  getRootPath(path: string): string | null {
    return this.directorySource.getRootPath(path);
  }

  joinChildPath(basePath: string, childName: string): string {
    const trimmed = (basePath || 'webdav://').replace(/\/+$/, '');
    return `${trimmed}/${childName}`;
  }

  private filterAndFormatSuggestions(entries: FileInfo[] = [], prefix: string): PathSuggestion[] {
    const prefixLower = prefix.toLowerCase();
    const accumulated: PathSuggestion[] = [];
    const seenNames = new Set<string>();

    for (const entry of entries) {
      if (entry.type !== FileType.DIRECTORY) continue;
      if (prefixLower && !entry.name.toLowerCase().startsWith(prefixLower)) continue;
      if (seenNames.has(entry.name)) continue;
      seenNames.add(entry.name);

      accumulated.push({
        path: normalizePath(entry.path),
        displayName: entry.name,
        matchType: 'directory',
        score: 100,
        exists: true,
      });
    }

    accumulated.sort((a, b) => a.path.localeCompare(b.path));
    return accumulated;
  }
}

/**
 * Archive scheme provider: browsing inside archives.
 */
class ArchiveAutocompleteProvider implements SchemeAutocompleteProvider {
  scheme = 'archive';

  constructor(private readonly directorySource: DirectorySourceService) {}

  canHandle(query: string): boolean {
    const normalized = normalizePath((query || '').trim().replace(/\\/g, '/'));
    return /^archive:\/\//i.test(normalized);
  }

  shouldFetch(query: string): boolean {
    const q = (query || '').trim();
    if (!q) return false;
    const n = normalizePath(q);
    return /^archive:\/\/\.\/[^/]*(?:\/.*)?$/i.test(n);
  }

  splitContext(query: string): { dirPath: string; prefix: string } {
    const q = normalizePath((query || '').replace(/\\/g, '/'));
    if (!q) return { dirPath: 'archive://', prefix: '' };

    if (q.toLowerCase() === 'archive://') {
      return { dirPath: 'archive://', prefix: '' };
    }

    if (q.endsWith('/')) return { dirPath: q, prefix: '' };

    const lastSlash = q.lastIndexOf('/');
    if (lastSlash > 'archive://'.length - 1) {
      return { dirPath: q.slice(0, lastSlash + 1), prefix: q.slice(lastSlash + 1) };
    }

    return { dirPath: 'archive://', prefix: q.slice('archive://'.length) };
  }

  async fetchSuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]> {
    try {
      const listing = await this.directorySource.listDirectory(dirPath, false);
      return this.filterAndFormatSuggestions(listing.entries, prefix);
    } catch {
      return [];
    }
  }

  async streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    const suggestions = await this.fetchSuggestions(dirPath, prefix);
    onBatch(suggestions);
    return suggestions;
  }

  supportsStreaming(_path: string): boolean {
    return false;
  }

  getRootPath(path: string): string | null {
    return this.directorySource.getRootPath(path);
  }

  joinChildPath(basePath: string, childName: string): string {
    const trimmed = (basePath || 'archive://').replace(/\/+$/, '');
    return `${trimmed}/${childName}`;
  }

  private filterAndFormatSuggestions(entries: FileInfo[] = [], prefix: string): PathSuggestion[] {
    const prefixLower = prefix.toLowerCase();
    const accumulated: PathSuggestion[] = [];
    const seenNames = new Set<string>();

    for (const entry of entries) {
      if (entry.type !== FileType.DIRECTORY) continue;
      if (prefixLower && !entry.name.toLowerCase().startsWith(prefixLower)) continue;
      if (seenNames.has(entry.name)) continue;
      seenNames.add(entry.name);

      accumulated.push({
        path: normalizePath(entry.path),
        displayName: entry.name,
        matchType: 'directory',
        score: 100,
        exists: true,
      });
    }

    accumulated.sort((a, b) => a.path.localeCompare(b.path));
    return accumulated;
  }
}

/**
 * Generic fallback provider for unknown schemes.
 */
class GenericSchemeAutocompleteProvider implements SchemeAutocompleteProvider {
  scheme: string;

  constructor(private readonly directorySource: DirectorySourceService, scheme: string) {
    this.scheme = scheme;
  }

  canHandle(query: string): boolean {
    const normalized = normalizePath((query || '').trim().replace(/\\/g, '/'));
    const schemeMatch = /^([a-z][a-z0-9+.-]*):\/\//i.exec(normalized);
    return schemeMatch ? schemeMatch[1].toLowerCase() === this.scheme : false;
  }

  shouldFetch(query: string): boolean {
    const q = (query || '').trim();
    if (!q) return false;
    const n = normalizePath(q);
    return n.includes('/') || n.endsWith('://');
  }

  splitContext(query: string): { dirPath: string; prefix: string } {
    const q = normalizePath((query || '').replace(/\\/g, '/'));
    if (!q) return { dirPath: `${this.scheme}://`, prefix: '' };

    // Scheme-only (e.g., "ftp://")
    const schemeOnlyMatch = /^([a-z][a-z0-9+.-]*):\/\/$/.exec(q);
    if (schemeOnlyMatch) {
      return { dirPath: q, prefix: '' };
    }

    // Path ends with slash
    if (q.endsWith('/')) return { dirPath: q, prefix: '' };

    // Split on last slash
    const lastSlash = q.lastIndexOf('/');
    if (lastSlash >= 0) {
      return { dirPath: q.slice(0, lastSlash + 1), prefix: q.slice(lastSlash + 1) };
    }

    return { dirPath: `${this.scheme}://`, prefix: q.slice(`${this.scheme}://`.length) };
  }

  async fetchSuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]> {
    try {
      const listing = await this.directorySource.listDirectory(dirPath, false);
      return this.filterAndFormatSuggestions(listing.entries, prefix);
    } catch {
      return [];
    }
  }

  async streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    const suggestions = await this.fetchSuggestions(dirPath, prefix);
    onBatch(suggestions);
    return suggestions;
  }

  supportsStreaming(path: string): boolean {
    return this.directorySource.supportsStreaming(path);
  }

  getRootPath(path: string): string | null {
    return this.directorySource.getRootPath(path);
  }

  joinChildPath(basePath: string, childName: string): string {
    const trimmed = (basePath || `${this.scheme}://`).replace(/\/+$/, '');
    return `${trimmed}/${childName}`;
  }

  private filterAndFormatSuggestions(entries: FileInfo[] = [], prefix: string): PathSuggestion[] {
    const prefixLower = prefix.toLowerCase();
    const accumulated: PathSuggestion[] = [];
    const seenNames = new Set<string>();

    for (const entry of entries) {
      if (entry.type !== FileType.DIRECTORY) continue;
      if (prefixLower && !entry.name.toLowerCase().startsWith(prefixLower)) continue;
      if (seenNames.has(entry.name)) continue;
      seenNames.add(entry.name);

      accumulated.push({
        path: normalizePath(entry.path),
        displayName: entry.name,
        matchType: 'directory',
        score: 100,
        exists: true,
      });
    }

    accumulated.sort((a, b) => a.path.localeCompare(b.path));
    return accumulated;
  }
}

/**
 * Scheme-oriented autocompletion service.
 * Routes autocompletion to scheme-specific providers.
 */
@Injectable({ providedIn: 'root' })
export class SchemeAutocompleteService {
  private readonly providers: SchemeAutocompleteProvider[];
  private readonly filesystemProvider: SchemeAutocompleteProvider;
  private readonly providerCache = new Map<string, SchemeAutocompleteProvider>();

  constructor(
    private readonly directorySource: DirectorySourceService,
  ) {
    this.filesystemProvider = new FilesystemAutocompleteProvider(directorySource);
    this.providers = [
      // Specialized providers for specific schemes (order matters for canHandle matching)
      new SearchAutocompleteProvider(),
      new RegistryAutocompleteProvider(directorySource),
      new FtpAutocompleteProvider(directorySource),
      new SmbAutocompleteProvider(directorySource),
      new SftpAutocompleteProvider(directorySource),
      new WebdavAutocompleteProvider(directorySource),
      new ArchiveAutocompleteProvider(directorySource),
      // Generic filesystem provider (catch-all for filesystem://, local paths, etc.)
      this.filesystemProvider,
    ];
  }

  /**
   * Extract the scheme from a path.
   * Returns the scheme portion (e.g., "drive", "ftp", "search") or null.
   */
  getScheme(path: string): string | null {
    const normalized = normalizePath((path || '').trim().replace(/\\/g, '/'));
    const schemeMatch = /^([a-z][a-z0-9+.-]*):\/\//i.exec(normalized);
    return schemeMatch ? schemeMatch[1].toLowerCase() : null;
  }

  /**
   * Get or create the appropriate provider for a query.
   */
  private getProvider(query: string): SchemeAutocompleteProvider {
    // Try registered providers first
    for (const provider of this.providers) {
      if (provider.canHandle(query)) {
        return provider;
      }
    }

    // For unknown schemes, create or reuse a generic provider
    const scheme = this.getScheme(query);
    if (scheme) {
      if (!this.providerCache.has(scheme)) {
        this.providerCache.set(
          scheme,
          new GenericSchemeAutocompleteProvider(this.directorySource, scheme),
        );
      }
      return this.providerCache.get(scheme)!;
    }

    // Fallback to filesystem provider for no-scheme address bar values.
    return this.filesystemProvider;
  }

  /**
   * Determine if the given path should fetch children for autocompletion.
   */
  shouldFetchSuggestions(query: string): boolean {
    const provider = this.getProvider(query);
    return provider.shouldFetch(query);
  }

  /**
   * Split a query into the directory context and the prefix to match.
   */
  splitQueryContext(query: string): { dirPath: string; prefix: string } {
    const provider = this.getProvider(query);
    return provider.splitContext(query);
  }

  /**
   * Fetch suggestions for a directory and prefix.
   */
  async fetchSuggestions(dirPath: string, prefix: string): Promise<PathSuggestion[]> {
    const provider = this.getProvider(dirPath);
    return provider.fetchSuggestions(dirPath, prefix);
  }

  /**
   * Stream suggestions with batching support.
   */
  async streamSuggestions(
    dirPath: string,
    prefix: string,
    onBatch: (suggestions: PathSuggestion[]) => void,
  ): Promise<PathSuggestion[]> {
    const provider = this.getProvider(dirPath);
    return provider.streamSuggestions(dirPath, prefix, onBatch);
  }

  /**
   * Check if a given scheme supports streaming suggestions.
   */
  supportsStreaming(path: string): boolean {
    const provider = this.getProvider(path);
    return provider.supportsStreaming(path);
  }

  /**
   * Get the root path for a given path.
   */
  getRootPath(path: string): string | null {
    const provider = this.getProvider(path);
    return provider.getRootPath(path);
  }

  /**
   * Join a directory path and child name.
   */
  joinChildPath(basePath: string, childName: string): string {
    const provider = this.getProvider(basePath);
    return provider.joinChildPath(basePath, childName);
  }
}
