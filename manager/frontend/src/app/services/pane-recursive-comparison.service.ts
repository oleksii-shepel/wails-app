import { Injectable } from '@angular/core';
import type { ApiService } from './api.service';
import { FileType } from '../shared/protocol';
import type { FileInfo } from '../shared/protocol';
import { PaneComparisonService, type PaneComparisonItem, type PaneComparisonResult } from './pane-comparison.service';

export interface RecursivePaneComparisonOptions {
  showHidden?: boolean;
  maxDepth?: number;
  maxItems?: number;
}

export interface RecursivePaneComparisonResult {
  comparison: PaneComparisonResult;
  directoriesScanned: number;
  truncated: boolean;
  warnings: string[];
}

@Injectable({ providedIn: 'root' })
export class PaneRecursiveComparisonService {
  constructor(private readonly comparison: PaneComparisonService) {}

  async compare(
    api: ApiService,
    leftRoot: string,
    rightRoot: string,
    options: RecursivePaneComparisonOptions = {},
  ): Promise<RecursivePaneComparisonResult> {
    const maxDepth = Math.max(1, options.maxDepth ?? 64);
    const maxItems = Math.max(100, options.maxItems ?? 10000);
    const showHidden = options.showHidden ?? false;
    const items: PaneComparisonItem[] = [];
    const warnings: string[] = [];
    let directoriesScanned = 0;
    let truncated = false;

    const walk = async (leftDir: string, rightDir: string, relativeDir: string, depth: number): Promise<void> => {
      if (truncated || depth > maxDepth) {
        truncated = true;
        return;
      }

      let leftEntries: FileInfo[] = [];
      let rightEntries: FileInfo[] = [];
      try {
        [leftEntries, rightEntries] = await Promise.all([
          api.listDirectory(leftDir, showHidden).then(r => r.entries ?? []),
          api.listDirectory(rightDir, showHidden).then(r => r.entries ?? []),
        ]);
        directoriesScanned++;
      } catch (error) {
        warnings.push(`${relativeDir || '.'}: ${error instanceof Error ? error.message : String(error)}`);
        return;
      }

      const level = this.comparison.compare(leftEntries, rightEntries);
      for (const item of level.items) {
        if (item.name === '..' || item.left?.path === '..' || item.right?.path === '..') continue;
        const relativePath = relativeDir ? `${relativeDir}/${item.name}` : item.name;
        items.push({ ...item, key: relativePath.toLocaleLowerCase(), name: relativePath });
        if (items.length >= maxItems) {
          truncated = true;
          return;
        }

        const bothDirectories = item.left?.type === FileType.DIRECTORY && item.right?.type === FileType.DIRECTORY;
        const followsSymlink = !!item.left?.isSymlink || !!item.right?.isSymlink;
        if (bothDirectories && !followsSymlink) {
          await walk(item.left!.path, item.right!.path, relativePath, depth + 1);
          if (truncated) return;
        }
      }
    };

    await walk(leftRoot, rightRoot, '', 0);
    return {
      comparison: this.comparison.fromItems(items),
      directoriesScanned,
      truncated,
      warnings,
    };
  }
}