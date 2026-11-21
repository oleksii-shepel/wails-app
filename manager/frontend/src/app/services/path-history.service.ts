import { Injectable } from '@angular/core';
import { createBehaviorSubject, type Stream } from '@epikodelabs/streamix';
import { ApiService } from './api.service';
import { FileType } from '../shared/protocol';
import {
  canonicalizeLocation,
  describeLocation,
  locationIdentity,
  locationLabel,
  type LocationKind,
} from './location-reference';

export interface LocationHistoryEntry {
  location: string;
  scheme: string;
  kind: LocationKind;
  timestamp: number;
  accessCount: number;
  paneId: string;
}

/** @deprecated Prefer LocationHistoryEntry. */
export type PathHistoryEntry = LocationHistoryEntry;

export interface PathSuggestion {
  path: string;
  displayName: string;
  matchType: 'history' | 'directory' | 'fuzzy';
  score: number;
  exists: boolean;
}

interface LegacyPathHistoryEntry {
  path?: string;
  location?: string;
  timestamp?: number;
  accessCount?: number;
  paneId?: string;
}

@Injectable({ providedIn: 'root' })
export class PathHistoryService {
  private readonly STORAGE_KEY = 'filemanager:path-history';
  private readonly MAX_HISTORY_ENTRIES = 100;

  private history = new Map<string, LocationHistoryEntry>();
  private historySubject = createBehaviorSubject<LocationHistoryEntry[]>([]);

  constructor(private apiService: ApiService) {
    this.loadHistory();
  }

  get history$(): Stream<LocationHistoryEntry[]> {
    return this.historySubject;
  }

  addLocation(location: string, paneId: string): void {
    const descriptor = describeLocation(location);
    if (!descriptor) return;

    const id = locationIdentity(descriptor.location);
    const existing = this.history.get(id);
    if (existing) {
      existing.timestamp = Date.now();
      existing.accessCount++;
      existing.paneId = paneId || existing.paneId;
    } else {
      this.history.set(id, {
        ...descriptor,
        timestamp: Date.now(),
        accessCount: 1,
        paneId,
      });
    }

    this.pruneHistory();
    this.saveHistory();
    this.emitHistory();
  }

  /** Compatibility alias while callers move from path to location terminology. */
  addPath(path: string, paneId: string): void {
    this.addLocation(path, paneId);
  }

  getRecentLocations(limit = 10): LocationHistoryEntry[] {
    return this.sortedHistory((a, b) => b.timestamp - a.timestamp).slice(0, limit);
  }

  getFrequentLocations(limit = 10, minimumAccessCount = 2): LocationHistoryEntry[] {
    return this.sortedHistory((a, b) =>
      b.accessCount - a.accessCount || b.timestamp - a.timestamp,
    ).filter(entry => entry.accessCount >= minimumAccessCount).slice(0, limit);
  }

  getRelatedLocations(location: string, limit = 10): LocationHistoryEntry[] {
    const descriptor = describeLocation(location);
    if (!descriptor || descriptor.scheme === 'filesystem') return [];
    const activeId = locationIdentity(descriptor.location);
    return this.getRecentLocations(this.MAX_HISTORY_ENTRIES)
      .filter(entry => entry.scheme === descriptor.scheme && locationIdentity(entry.location) !== activeId)
      .slice(0, limit);
  }

  async getSuggestions(query: string, currentPath: string): Promise<PathSuggestion[]> {
    query = query.trim();
    if (!query) {
      return this.getRecentLocations(10).map(entry => this.toHistorySuggestion(entry));
    }

    const suggestions: PathSuggestion[] = [];
    suggestions.push(...this.getHistoryMatches(query));

    if (this.canUseDirectoryAutocomplete(query)) {
      suggestions.push(...await this.getDirectorySuggestions(query));
    }

    suggestions.push(...this.getFuzzyMatches(query));

    suggestions.sort((a, b) => b.score - a.score);
    const uniqueLocations = new Set<string>();
    const uniqueSuggestions = suggestions.filter(suggestion => {
      const id = locationIdentity(suggestion.path);
      if (!id || uniqueLocations.has(id)) return false;
      uniqueLocations.add(id);
      return true;
    });

    return uniqueSuggestions.slice(0, 10);
  }

  private getHistoryMatches(query: string): PathSuggestion[] {
    const lowerQuery = query.toLowerCase();
    const matches: PathSuggestion[] = [];

    for (const entry of this.history.values()) {
      const lowerLocation = entry.location.toLowerCase();
      const lowerLabel = locationLabel(entry.location).toLowerCase();
      const prefix = lowerLocation.startsWith(lowerQuery) || lowerLabel.startsWith(lowerQuery);
      const contains = lowerLocation.includes(lowerQuery) || lowerLabel.includes(lowerQuery);
      if (!prefix && !contains) continue;

      matches.push({
        path: entry.location,
        displayName: locationLabel(entry.location),
        matchType: 'history',
        score: (prefix ? 100 : 80) + (entry.accessCount * (prefix ? 5 : 3)),
        exists: true,
      });
    }

    return matches;
  }

  private async getDirectorySuggestions(query: string): Promise<PathSuggestion[]> {
    try {
      const normalizedQuery = canonicalizeLocation(query);
      const lastSlash = normalizedQuery.lastIndexOf('/');
      if (lastSlash < 0) return [];

      const directoryInput = /[\\/]$/.test(query.replace(/\\/g, '/'));
      const dirPath = directoryInput ? normalizedQuery : normalizedQuery.substring(0, lastSlash + 1) || '/';
      const prefix = directoryInput ? '' : normalizedQuery.substring(lastSlash + 1);
      const listing = await this.apiService.listDirectory(dirPath, false);
      const suggestions: PathSuggestion[] = [];

      for (const entry of listing.entries) {
        if (entry.type !== FileType.DIRECTORY) continue;
        if (!entry.name.toLowerCase().startsWith(prefix.toLowerCase())) continue;
        const fullPath = canonicalizeLocation(entry.path || `${dirPath.replace(/\/+$/, '')}/${entry.name}`);
        suggestions.push({
          path: fullPath,
          displayName: entry.name,
          matchType: 'directory',
          score: 90,
          exists: true,
        });
      }

      return suggestions;
    } catch {
      return [];
    }
  }

  private canUseDirectoryAutocomplete(query: string): boolean {
    const normalized = canonicalizeLocation(query);
    return (
      normalized.startsWith('/') ||
      normalized.startsWith('./') ||
      normalized.startsWith('../') ||
      /^filesystem:\/\/[a-z](?:\/.*)?$/i.test(normalized) ||
      normalized.startsWith('//')
    );
  }

  private getFuzzyMatches(query: string): PathSuggestion[] {
    const matches: PathSuggestion[] = [];
    const queryTokens = query.toLowerCase().split(/[\s\-_\/]/).filter(Boolean);
    if (queryTokens.length === 0) return matches;

    for (const entry of this.history.values()) {
      const searchable = `${locationLabel(entry.location)} ${entry.location}`.toLowerCase();
      const locationTokens = searchable.split(/[\s\-_\/]/).filter(Boolean);
      let matchCount = 0;

      for (const queryToken of queryTokens) {
        if (locationTokens.some(token => token.includes(queryToken))) matchCount++;
      }

      if (matchCount === 0) continue;
      const score = (matchCount / queryTokens.length) * 50 + (entry.accessCount * 2);
      if (score <= 20) continue;
      matches.push({
        path: entry.location,
        displayName: locationLabel(entry.location),
        matchType: 'fuzzy',
        score,
        exists: true,
      });
    }

    return matches;
  }

  private toHistorySuggestion(entry: LocationHistoryEntry): PathSuggestion {
    return {
      path: entry.location,
      displayName: locationLabel(entry.location),
      matchType: 'history',
      score: 50 + (entry.accessCount * 5),
      exists: true,
    };
  }

  private pruneHistory(): void {
    if (this.history.size <= this.MAX_HISTORY_ENTRIES) return;
    const entries = this.sortedHistory((a, b) => a.timestamp - b.timestamp);
    const toRemove = entries.slice(0, this.history.size - this.MAX_HISTORY_ENTRIES);
    for (const entry of toRemove) this.history.delete(locationIdentity(entry.location));
  }

  private loadHistory(): void {
    try {
      const stored = localStorage.getItem(this.STORAGE_KEY);
      if (!stored) return;
      const entries: unknown = JSON.parse(stored);
      if (!Array.isArray(entries)) return;
      for (const value of entries) {
        if (!value || typeof value !== 'object') continue;
        const storedEntry = value as LegacyPathHistoryEntry;
        const rawLocation = typeof storedEntry.location === 'string' ? storedEntry.location : storedEntry.path;
        if (typeof rawLocation !== 'string') continue;
        const descriptor = describeLocation(rawLocation);
        if (!descriptor) continue;
        const id = locationIdentity(descriptor.location);
        const recordedAt = Number(storedEntry.timestamp);
        const timestamp = Number.isFinite(recordedAt) && recordedAt > 0 ? recordedAt : Date.now();
        const count = Number(storedEntry.accessCount);
        const accessCount = Number.isSafeInteger(count) && count > 0 ? count : 1;
        const paneId = typeof storedEntry.paneId === 'string' && storedEntry.paneId ? storedEntry.paneId : 'left';
        const existing = this.history.get(id);

        if (existing) {
          if (timestamp >= existing.timestamp) {
            existing.timestamp = timestamp;
            existing.paneId = paneId;
          }
          existing.accessCount += accessCount;
        } else {
          this.history.set(id, { ...descriptor, timestamp, accessCount, paneId });
        }
      }
      this.pruneHistory();
      this.saveHistory(); // Persist the location-aware shape after legacy reads.
      this.emitHistory();
    } catch (error) {
      console.error('Failed to load location history:', error);
    }
  }

  private saveHistory(): void {
    try {
      localStorage.setItem(this.STORAGE_KEY, JSON.stringify(this.getRecentLocations(this.MAX_HISTORY_ENTRIES)));
    } catch (error) {
      console.error('Failed to save location history:', error);
    }
  }

  private emitHistory(): void {
    this.historySubject.next(this.getRecentLocations(this.MAX_HISTORY_ENTRIES));
  }

  private sortedHistory(compare: (a: LocationHistoryEntry, b: LocationHistoryEntry) => number): LocationHistoryEntry[] {
    return Array.from(this.history.values()).sort(compare).map(entry => ({ ...entry }));
  }

  clearHistory(): void {
    this.history.clear();
    this.saveHistory();
    this.emitHistory();
  }

  getAllHistory(): LocationHistoryEntry[] {
    return this.getRecentLocations(this.MAX_HISTORY_ENTRIES);
  }

  removeLocation(location: string): void {
    const id = locationIdentity(location);
    if (!id) return;
    this.history.delete(id);
    this.saveHistory();
    this.emitHistory();
  }

  /** Compatibility alias while callers move from path to location terminology. */
  removePath(path: string): void {
    this.removeLocation(path);
  }
}