import { Injectable } from '@angular/core';
import { FileInfo, FileType } from '../shared/protocol';
import type { ApiService } from './api.service';

export type PaneComparisonPolicy = 'size' | 'date' | 'size-date' | 'hash' | 'content';

export type PaneComparisonStatus =
  | 'only-left'
  | 'only-right'
  | 'left-newer'
  | 'right-newer'
  | 'size-different'
  | 'hash-different'
  | 'content-different'
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
  hashDifferent: number;
  contentDifferent: number;
  typeDifferent: number;
  different: number;
}

export interface PaneComparisonResult {
  items: PaneComparisonItem[];
  summary: PaneComparisonSummary;
  policy: PaneComparisonPolicy;
}

export interface PaneComparisonOptions {
  caseSensitive?: boolean;
  modifiedToleranceSeconds?: number;
  policy?: PaneComparisonPolicy;
}

const DEFAULT_MODIFIED_TOLERANCE_SECONDS = 2;
const CONTENT_COMPARE_CHUNK_SIZE = 256 * 1024;

@Injectable({ providedIn: 'root' })
export class PaneComparisonService {
  compare(
    leftEntries: readonly FileInfo[],
    rightEntries: readonly FileInfo[],
    options: PaneComparisonOptions = {},
  ): PaneComparisonResult {
    const caseSensitive = options.caseSensitive ?? false;
    const tolerance = Math.max(0, options.modifiedToleranceSeconds ?? DEFAULT_MODIFIED_TOLERANCE_SECONDS);
    const policy = options.policy ?? 'size-date';
    const metadataPolicy = policy === 'hash' || policy === 'content' ? 'size' : policy;
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
        status: this.status(leftEntry, rightEntry, tolerance, metadataPolicy),
      });
    }

    items.sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: 'base' }));
    return this.fromItems(items, policy);
  }

  async verify(
    api: Pick<ApiService, 'hashFile' | 'readFileRange'>,
    result: PaneComparisonResult,
    policy: PaneComparisonPolicy = result.policy,
  ): Promise<PaneComparisonResult> {
    if (policy !== 'hash' && policy !== 'content') {
      return this.fromItems(result.items, policy);
    }

    const items = await Promise.all(result.items.map(async item => {
      if (!item.left || !item.right || item.left.type !== item.right.type) return { ...item };
      if (item.left.type === FileType.DIRECTORY) return { ...item, status: 'same' as PaneComparisonStatus };
      if (item.left.size !== item.right.size) return { ...item, status: 'size-different' as PaneComparisonStatus };

      if (policy === 'hash') {
        const [leftHash, rightHash] = await Promise.all([
          api.hashFile(item.left.path),
          api.hashFile(item.right.path),
        ]);
        return { ...item, status: leftHash === rightHash ? 'same' as const : 'hash-different' as const };
      }

      const same = await this.contentEqual(api, item.left.path, item.right.path, item.left.size);
      return { ...item, status: same ? 'same' as const : 'content-different' as const };
    }));

    return this.fromItems(items, policy);
  }

  fromItems(items: readonly PaneComparisonItem[], policy: PaneComparisonPolicy = 'size-date'): PaneComparisonResult {
    const normalized = [...items];
    const summary: PaneComparisonSummary = {
      total: normalized.length, same: 0, onlyLeft: 0, onlyRight: 0,
      leftNewer: 0, rightNewer: 0, sizeDifferent: 0, hashDifferent: 0,
      contentDifferent: 0, typeDifferent: 0, different: 0,
    };
    for (const item of normalized) {
      switch (item.status) {
        case 'same': summary.same++; break;
        case 'only-left': summary.onlyLeft++; break;
        case 'only-right': summary.onlyRight++; break;
        case 'left-newer': summary.leftNewer++; break;
        case 'right-newer': summary.rightNewer++; break;
        case 'size-different': summary.sizeDifferent++; break;
        case 'hash-different': summary.hashDifferent++; break;
        case 'content-different': summary.contentDifferent++; break;
        case 'type-different': summary.typeDifferent++; break;
      }
    }
    summary.different = summary.total - summary.same;
    return { items: normalized, summary, policy };
  }

  leftDifferencePaths(result: PaneComparisonResult): string[] {
    return result.items.filter(item => item.left && item.status !== 'same' && item.status !== 'only-right').map(item => item.left!.path);
  }
  rightDifferencePaths(result: PaneComparisonResult): string[] {
    return result.items.filter(item => item.right && item.status !== 'same' && item.status !== 'only-left').map(item => item.right!.path);
  }
  missingOnRightPaths(result: PaneComparisonResult): string[] {
    return result.items.filter(item => item.status === 'only-left' && item.left).map(item => item.left!.path);
  }
  missingOnLeftPaths(result: PaneComparisonResult): string[] {
    return result.items.filter(item => item.status === 'only-right' && item.right).map(item => item.right!.path);
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

  private status(left: FileInfo | null, right: FileInfo | null, tolerance: number, policy: 'size' | 'date' | 'size-date'): PaneComparisonStatus {
    if (!left) return 'only-right';
    if (!right) return 'only-left';
    if (left.type !== right.type) return 'type-different';
    if (left.type === FileType.DIRECTORY && right.type === FileType.DIRECTORY) return 'same';

    if ((policy === 'size' || policy === 'size-date') && left.size !== right.size) return 'size-different';
    if (policy === 'size') return 'same';

    const delta = (left.modified || 0) - (right.modified || 0);
    if (Math.abs(delta) <= tolerance) return 'same';
    return delta > 0 ? 'left-newer' : 'right-newer';
  }

  private async contentEqual(
    api: Pick<ApiService, 'readFileRange'>,
    leftPath: string,
    rightPath: string,
    size: number,
  ): Promise<boolean> {
    for (let offset = 0; offset < size; offset += CONTENT_COMPARE_CHUNK_SIZE) {
      const length = Math.min(CONTENT_COMPARE_CHUNK_SIZE, size - offset);
      const [left, right] = await Promise.all([
        api.readFileRange(leftPath, offset, length),
        api.readFileRange(rightPath, offset, length),
      ]);
      if (left.content !== right.content) return false;
    }
    return true;
  }
}
