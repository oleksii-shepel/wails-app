import { Injectable } from '@angular/core';
import { FileInfo, FileType } from '../shared/protocol';
import { FilterQuery, FilterCriteria, FilterType } from '../shared/protocol-enhanced';

/**
 * A CriteriaGroup is one "segment" separated by OR in the filter string.
 * Within a group, same-type criteria (e.g. multiple EXTENSIONs) are OR-ed;
 * different types are AND-ed.
 * Between groups, the entry only needs to satisfy ONE group (OR semantics).
 */
export interface CriteriaGroup {
  criteria: FilterCriteria[];
  text?: string;
}

@Injectable({
  providedIn: 'root'
})
export class FilterService {

  /**
   * Filter file entries based on query.
   *
   * query.groups (new): OR-separated groups of criteria. Within each group,
   *   same-type criteria are OR-ed, different types are AND-ed.
   * query.criteria (legacy): flat list, all AND-ed (kept for back-compat).
   * query.text: plain text match, AND-ed on top of whichever group matched.
   */
  filterEntries(entries: FileInfo[], query: FilterQuery & { groups?: CriteriaGroup[] }): FileInfo[] {
    const groups: CriteriaGroup[] = query.groups ?? [];
    const hasGroups = groups.length > 0;
    const hasLegacyCriteria = !hasGroups && query.criteria && query.criteria.length > 0;

    if (!query.text && !hasGroups && !hasLegacyCriteria) {
      return entries;
    }

    return entries.filter(entry => {
      // ── Text filter (AND'd globally) ───────────────────────────────────
      if (query.text) {
        if (!this.matchesTextFilter(entry, query.text, query.caseSensitive)) {
          return false;
        }
      }

      // ── Group-based evaluation (new path) ─────────────────────────────
      if (hasGroups) {
        // Entry must satisfy AT LEAST ONE group (OR between groups)
        return groups.some(group => this.matchesGroup(entry, group, query.caseSensitive));
      }

      // ── Legacy flat criteria (all AND-ed, kept for back-compat) ───────
      if (hasLegacyCriteria) {
        for (const criterion of query.criteria!) {
          if (!this.matchesCriterion(entry, criterion, query.caseSensitive)) {
            return false;
          }
        }
      }

      return true;
    });
  }

  /**
   * Test whether an entry satisfies a single CriteriaGroup.
   *
   * Rules:
   *  - Same FilterType criteria → OR  (e.g. ext:jpg OR ext:png)
   *  - Different FilterType criteria → AND (e.g. ext:jpg AND size>1mb)
   *  - Group-level text → AND with the above
   */
  private matchesGroup(entry: FileInfo, group: CriteriaGroup, caseSensitive?: boolean): boolean {
    // Optional per-group text filter
    if (group.text) {
      if (!this.matchesTextFilter(entry, group.text, caseSensitive)) return false;
    }

    // Bucket criteria by type
    const buckets = new Map<FilterType, FilterCriteria[]>();
    for (const c of group.criteria) {
      if (!buckets.has(c.type)) buckets.set(c.type, []);
      buckets.get(c.type)!.push(c);
    }

    // Every bucket must be satisfied; within each bucket any one match suffices
    for (const [, bucket] of buckets) {
      const bucketMatch = bucket.some(c => this.matchesCriterion(entry, c, caseSensitive));
      if (!bucketMatch) return false;
    }

    return true;
  }

  /**
   * Parse filter text into a structured query with OR groups.
   *
   * Supports:
   * - OR separator:       "ext:pdf OR ext:png"     → two groups (any match)
   * - Plain text:         "document"
   * - Extension filter:   "ext:pdf"
   * - Type filter:        "type:file" or "type:dir"
   * - Size filter:        "size>1mb" or "size<100kb"
   * - Modified filter:    "modified:today" or "modified>2024-01-01"
   * - Glob pattern:       "*.pdf" or "doc*"
   *
   * Category presets emit e.g.:
   *   "ext:jpg ext:jpeg ext:png"          (single preset, OR within extensions)
   *   "ext:jpg ext:jpeg OR ext:js ext:ts" (two presets joined with OR)
   */
  parseFilterText(text: string): FilterQuery & { groups: CriteriaGroup[] } {
    const emptyQuery = (): FilterQuery & { groups: CriteriaGroup[] } => ({
      text: '',
      criteria: [],
      groups: [],
      caseSensitive: false,
    });

    if (!text || text.trim() === '') return emptyQuery();

    // Split on " OR " (case-insensitive, surrounded by whitespace)
    const segments = text.trim().split(/\s+OR\s+/i);

    if (segments.length === 1) {
      // No OR — parse as a single group, expose via both groups[] and legacy fields
      const group = this.parseSegment(segments[0]);
      return {
        text: group.text ?? '',
        criteria: group.criteria,   // legacy compat
        groups: [group],
        caseSensitive: false,
      };
    }

    // Multiple OR segments
    const groups = segments.map(seg => this.parseSegment(seg));
    return {
      text: '',       // per-group text lives inside each group
      criteria: [],   // legacy field unused when groups present
      groups,
      caseSensitive: false,
    };
  }

  /**
   * Parse one whitespace-separated segment into a CriteriaGroup.
   * Multiple ext: tokens within the segment become sibling EXTENSION criteria
   * that are OR-ed at evaluation time (same bucket logic in matchesGroup).
   */
  private parseSegment(segment: string): CriteriaGroup {
    const group: CriteriaGroup = { criteria: [] };
    const plainTextTokens: string[] = [];

    const tokens = segment.trim().split(/\s+/);

    for (const token of tokens) {
      if (!token) continue;

      if (token.includes(':')) {
        const colonIdx = token.indexOf(':');
        const prefix = token.slice(0, colonIdx);
        const value  = token.slice(colonIdx + 1);
        const criterion = this.parseFilterToken(prefix, value);
        if (criterion) {
          group.criteria.push(criterion);
          continue;
        }
      }

      // Size comparisons: size>1mb
      const sizeMatch = token.match(/^size([><=]+)(.+)$/i);
      if (sizeMatch) {
        const criterion = this.parseSizeFilter(sizeMatch[1], sizeMatch[2]);
        if (criterion) { group.criteria.push(criterion); continue; }
      }

      // Date comparisons: modified>2024-01-01
      const dateMatch = token.match(/^modified([><=]+)(.+)$/i);
      if (dateMatch) {
        const criterion = this.parseDateFilter(dateMatch[1], dateMatch[2]);
        if (criterion) { group.criteria.push(criterion); continue; }
      }

      // Glob pattern
      if (token.includes('*') || token.includes('?')) {
        group.criteria.push({ type: FilterType.GLOB, value: token, operator: 'contains' });
        continue;
      }

      plainTextTokens.push(token);
    }

    if (plainTextTokens.length > 0) {
      group.text = plainTextTokens.join(' ');
    }

    return group;
  }

  /**
   * Parse a filter token (prefix:value format)
   */
  private parseFilterToken(prefix: string, value: string): FilterCriteria | null {
    switch (prefix.toLowerCase()) {
      case 'ext':
      case 'extension':
        return {
          type: FilterType.EXTENSION,
          value: value.startsWith('.') ? value : `.${value}`,
          operator: 'equals',
        };

      case 'type': {
        const fileType = this.parseFileType(value);
        if (fileType) return { type: FilterType.TYPE, value: fileType, operator: 'equals' };
        break;
      }

      case 'name':
        return { type: FilterType.NAME, value, operator: 'contains' };

      case 'modified':
        return this.parseDateFilter('=', value);
    }

    return null;
  }

  private parseFileType(value: string): string | null {
    const lower = value.toLowerCase();
    if (lower === 'file' || lower === 'f') return FileType.FILE;
    if (lower === 'directory' || lower === 'dir' || lower === 'd' || lower === 'folder') return FileType.DIRECTORY;
    if (lower === 'symlink' || lower === 'link' || lower === 'l') return FileType.SYMLINK;
    return null;
  }

  private parseSizeFilter(operator: string, value: string): FilterCriteria | null {
    const sizeBytes = this.parseSizeValue(value);
    if (sizeBytes === null) return null;
    return { type: FilterType.SIZE, value: sizeBytes.toString(), operator: this.normalizeOperator(operator) };
  }

  private parseDateFilter(operator: string, value: string): FilterCriteria | null {
    const timestamp = this.parseDateValue(value);
    if (timestamp === null) return null;
    return { type: FilterType.MODIFIED, value: timestamp.toString(), operator: this.normalizeOperator(operator) };
  }

  private parseSizeValue(value: string): number | null {
    const match = value.match(/^(\d+(?:\.\d+)?)(b|kb|mb|gb)?$/i);
    if (!match) return null;
    const numValue = parseFloat(match[1]);
    switch ((match[2] || 'b').toLowerCase()) {
      case 'b':  return numValue;
      case 'kb': return numValue * 1024;
      case 'mb': return numValue * 1024 * 1024;
      case 'gb': return numValue * 1024 * 1024 * 1024;
      default:   return numValue;
    }
  }

  private parseDateValue(value: string): number | null {
    const lower = value.toLowerCase();
    const now = Date.now();
    const day = 24 * 60 * 60 * 1000;

    if (lower === 'today')     return now - day;
    if (lower === 'yesterday') return now - (2 * day);
    if (lower === 'week' || lower === 'thisweek') return now - (7 * day);
    if (lower === 'month' || lower === 'thismonth') return now - (30 * day);

    const date = new Date(value);
    return isNaN(date.getTime()) ? null : date.getTime();
  }

  private normalizeOperator(operator: string): FilterCriteria['operator'] {
    switch (operator) {
      case '>':  return 'gt';
      case '<':  return 'lt';
      case '>=': return 'gte';
      case '<=': return 'lte';
      case '=':
      case '==': return 'equals';
      default:   return 'equals';
    }
  }

  private matchesTextFilter(entry: FileInfo, text: string, caseSensitive?: boolean): boolean {
    const searchText = caseSensitive ? text : text.toLowerCase();
    const entryName  = caseSensitive ? entry.name : entry.name.toLowerCase();
    return entryName.includes(searchText);
  }

  private matchesCriterion(entry: FileInfo, criterion: FilterCriteria, caseSensitive?: boolean): boolean {
    switch (criterion.type) {
      case FilterType.NAME:      return this.matchesNameCriterion(entry, criterion, caseSensitive);
      case FilterType.EXTENSION: return this.matchesExtensionCriterion(entry, criterion);
      case FilterType.TYPE:
        // type:symlink matches both file-symlinks (SYMLINK) and
        // dir-symlinks (DIRECTORY + isSymlink).
        if (criterion.value === FileType.SYMLINK) {
          return entry.type === FileType.SYMLINK || !!(entry as any).isSymlink;
        }
        return entry.type === criterion.value;
      case FilterType.SIZE:      return this.matchesSizeCriterion(entry, criterion);
      case FilterType.MODIFIED:  return this.matchesModifiedCriterion(entry, criterion);
      case FilterType.GLOB:      return this.matchesGlobPattern(entry.name, criterion.value);
      default:                   return true;
    }
  }

  private matchesNameCriterion(entry: FileInfo, criterion: FilterCriteria, caseSensitive?: boolean): boolean {
    const name  = caseSensitive ? entry.name : entry.name.toLowerCase();
    const value = caseSensitive ? criterion.value : criterion.value.toLowerCase();
    switch (criterion.operator) {
      case 'equals':     return name === value;
      case 'contains':   return name.includes(value);
      case 'startsWith': return name.startsWith(value);
      case 'endsWith':   return name.endsWith(value);
      default:           return name.includes(value);
    }
  }

  private matchesExtensionCriterion(entry: FileInfo, criterion: FilterCriteria): boolean {
    if (entry.type !== FileType.FILE) return false;
    return this.getFileExtension(entry.name) === criterion.value;
  }

  private matchesSizeCriterion(entry: FileInfo, criterion: FilterCriteria): boolean {
    const targetSize = parseInt(criterion.value, 10);
    const entrySize  = entry.size;
    switch (criterion.operator) {
      case 'equals': return entrySize === targetSize;
      case 'gt':     return entrySize >  targetSize;
      case 'lt':     return entrySize <  targetSize;
      case 'gte':    return entrySize >= targetSize;
      case 'lte':    return entrySize <= targetSize;
      default:       return entrySize === targetSize;
    }
  }

  private matchesModifiedCriterion(entry: FileInfo, criterion: FilterCriteria): boolean {
    const targetTime = parseInt(criterion.value, 10);
    const entryTime  = entry.modified * 1000;
    switch (criterion.operator) {
      case 'equals': return this.isSameDay(entryTime, targetTime);
      case 'gt':     return entryTime >  targetTime;
      case 'lt':     return entryTime <  targetTime;
      case 'gte':    return entryTime >= targetTime;
      case 'lte':    return entryTime <= targetTime;
      default:       return this.isSameDay(entryTime, targetTime);
    }
  }

  private matchesGlobPattern(name: string, pattern: string): boolean {
    const regexPattern = pattern
      .replace(/\./g, '\\.')
      .replace(/\*/g, '.*')
      .replace(/\?/g, '.');
    return new RegExp(`^${regexPattern}$`, 'i').test(name);
  }

  private getFileExtension(filename: string): string {
    const lastDot = filename.lastIndexOf('.');
    if (lastDot === -1 || lastDot === 0) return '';
    return filename.substring(lastDot);
  }

  private isSameDay(time1: number, time2: number): boolean {
    const d1 = new Date(time1), d2 = new Date(time2);
    return d1.getFullYear() === d2.getFullYear() &&
           d1.getMonth()    === d2.getMonth()    &&
           d1.getDate()     === d2.getDate();
  }
}