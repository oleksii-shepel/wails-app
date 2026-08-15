import { Injectable } from '@angular/core';
import { FileInfo, FileType } from '../shared/protocol';

export type PaneComparisonStatus =
  | 'only-left'
  | 'only-right'
  | 'left-newer'
  | 'right-newer'
  | 'size-different'
  | 'type-different'
  | 'same';

export interface PaneComparisonItem {
  key: string;
  name: string;
  status: PaneComparisonStatus;
  left: FileInfo | null;
  right: FileInfo | null;
}

export interface PaneComparisonSummary {
  total: number;
  same: number;
  onlyLeft: number;
  onlyRight: number;
  leftNewer: number;
  rightNewer: number;
  sizeDifferent: number;
  typeDifferent: number;
  different: number;
}

export interface PaneComparisonResult {
  items: PaneComparisonItem[];
  summary: PaneComparisonSummary;
}

export interface PaneComparisonOptions {
  caseSensitive?: boolean;
  modifiedToleranceSeconds?: number;
}

const DEFAULT_MODIFIED_TOLERANCE_SECONDS = 2;

@Injectable({ providedIn: 'root' })
export class PaneComparisonService {
  compare(
    leftEntries: readonly FileInfo[],
    rightEntries: readonly FileInfo[],
    options: PaneComparisonOptions = {},
  ): PaneComparisonResult {
    const caseSensitive = options.caseSensitive ?? false;
    const tolerance = Math.max(0, options.modifiedToleranceSeconds ?? DEFAULT_MODIFIED_TOLERANCE_SECONDS);
    const left = this.index(leftEntries, caseSensitive);
    const right = this.index(rightEntries, caseSensitive);
    const keys = new Set([...left.keys(), ...right.keys()]);
    const items: PaneComparisonItem[] = [];

    for (const key of keys) {
      const leftEntry = left.get(key) ?? null;
      const rightEntry = right.get(key) ?? null;
      const entry = leftEntry ?? rightEntry;
      if (!entry) continue;

      items.push({
        key,
        name: entry.name,
        left: leftEntry,
        right: rightEntry,
        status: this.status(leftEntry, rightEntry, tolerance),
      });
    }

    items.sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }));

    return this.fromItems(items);
  }

  fromItems(items: readonly PaneComparisonItem[]): PaneComparisonResult {
    const normalized = [...items];
    const summary: PaneComparisonSummary = {
      total: normalized.length, same: 0, onlyLeft: 0, onlyRight: 0,
      leftNewer: 0, rightNewer: 0, sizeDifferent: 0, typeDifferent: 0, different: 0,
    };
    for (const item of normalized) {
      switch (item.status) {
        case 'same': summary.same++; break;
        case 'only-left': summary.onlyLeft++; break;
        case 'only-right': summary.onlyRight++; break;
        case 'left-newer': summary.leftNewer++; break;
        case 'right-newer': summary.rightNewer++; break;
        case 'size-different': summary.sizeDifferent++; break;
        case 'type-different': summary.typeDifferent++; break;
      }
    }
    summary.different = summary.total - summary.same;
    return { items: normalized, summary };
  }

  leftDifferencePaths(result: PaneComparisonResult): string[] {
    return result.items
      .filter(item => item.left && item.status !== 'same' && item.status !== 'only-right')
      .map(item => item.left!.path);
  }

  rightDifferencePaths(result: PaneComparisonResult): string[] {
    return result.items
      .filter(item => item.right && item.status !== 'same' && item.status !== 'only-left')
      .map(item => item.right!.path);
  }

  missingOnRightPaths(result: PaneComparisonResult): string[] {
    return result.items
      .filter(item => item.status === 'only-left' && item.left)
      .map(item => item.left!.path);
  }

  missingOnLeftPaths(result: PaneComparisonResult): string[] {
    return result.items
      .filter(item => item.status === 'only-right' && item.right)
      .map(item => item.right!.path);
  }

  private index(entries: readonly FileInfo[], caseSensitive: boolean): Map<string, FileInfo> {
    const result = new Map<string, FileInfo>();
    for (const entry of entries ?? []) {
      if (!entry?.name || entry.path === '..') continue;
      const key = caseSensitive ? entry.name : entry.name.toLocaleLowerCase();
      if (!result.has(key)) result.set(key, entry);
    }
    return result;
  }

  private status(left: FileInfo | null, right: FileInfo | null, tolerance: number): PaneComparisonStatus {
    if (!left) return 'only-right';
    if (!right) return 'only-left';
    if (left.type !== right.type) return 'type-different';

    // Directory contents are intentionally not recursively compared in the
    // shallow pane comparison. Presence/type parity is enough for this level.
    if (left.type === FileType.DIRECTORY && right.type === FileType.DIRECTORY) {
      return 'same';
    }

    if (left.size !== right.size) return 'size-different';

    const delta = (left.modified || 0) - (right.modified || 0);
    if (Math.abs(delta) <= tolerance) return 'same';
    return delta > 0 ? 'left-newer' : 'right-newer';
  }
}
