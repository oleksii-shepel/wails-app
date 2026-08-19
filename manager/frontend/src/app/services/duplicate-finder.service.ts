import { Injectable } from '@angular/core';
import { ApiService } from './api.service';
import { FileInfo, FileType } from '../shared/protocol';

export interface DuplicateScanOptions {
  includeHidden?: boolean;
  minSize?: number;
  maxFiles?: number;
  maxDepth?: number;
}

export interface DuplicateScanProgress {
  phase: 'enumerating' | 'hashing';
  discoveredFiles: number;
  candidateFiles: number;
  hashedFiles: number;
}

export interface DuplicateGroup {
  hash: string;
  size: number;
  files: FileInfo[];
  reclaimableBytes: number;
  suggestedKeeperPath: string;
}

export type DuplicateKeeperPolicy = 'oldest' | 'newest' | 'shortest-path';

export interface DuplicateScanResult {
  root: string;
  groups: DuplicateGroup[];
  scannedFiles: number;
  hashedFiles: number;
  reclaimableBytes: number;
  truncated: boolean;
  warnings: string[];
}

@Injectable({ providedIn: 'root' })
export class DuplicateFinderService {
  constructor(private readonly api: ApiService) {}

  async scan(
    root: string,
    options: DuplicateScanOptions = {},
    onProgress?: (progress: DuplicateScanProgress) => void,
  ): Promise<DuplicateScanResult> {
    const includeHidden = !!options.includeHidden;
    const minSize = Math.max(0, options.minSize ?? 1);
    const maxFiles = Math.max(1, options.maxFiles ?? 20_000);
    const maxDepth = Math.max(0, options.maxDepth ?? 64);
    const files: FileInfo[] = [];
    const warnings: string[] = [];
    let truncated = false;

    const walk = async (path: string, depth: number): Promise<void> => {
      if (files.length >= maxFiles) { truncated = true; return; }
      if (depth > maxDepth) { truncated = true; return; }
      let listing;
      try {
        listing = await this.api.listDirectory(path, includeHidden);
      } catch (error) {
        warnings.push(`${path}: ${this.messageOf(error)}`);
        return;
      }
      for (const entry of listing.entries ?? []) {
        if (files.length >= maxFiles) { truncated = true; return; }
        if (!includeHidden && entry.isHidden) continue;
        if (entry.type === FileType.DIRECTORY && !entry.isSymlink) {
          await walk(entry.path, depth + 1);
          continue;
        }
        if (entry.type !== FileType.FILE || entry.isSymlink || entry.size < minSize) continue;
        files.push(entry);
        onProgress?.({ phase: 'enumerating', discoveredFiles: files.length, candidateFiles: 0, hashedFiles: 0 });
      }
    };

    await walk(root, 0);

    const bySize = new Map<number, FileInfo[]>();
    for (const file of files) {
      const bucket = bySize.get(file.size) ?? [];
      bucket.push(file);
      bySize.set(file.size, bucket);
    }
    const candidates = Array.from(bySize.values()).filter(group => group.length > 1).flat();
    const byHash = new Map<string, FileInfo[]>();
    let hashedFiles = 0;
    for (const file of candidates) {
      try {
        const hash = await this.api.hashFile(file.path);
        const key = `${file.size}:${hash}`;
        const bucket = byHash.get(key) ?? [];
        bucket.push(file);
        byHash.set(key, bucket);
      } catch (error) {
        warnings.push(`${file.path}: ${this.messageOf(error)}`);
      } finally {
        hashedFiles++;
        onProgress?.({ phase: 'hashing', discoveredFiles: files.length, candidateFiles: candidates.length, hashedFiles });
      }
    }

    const groups = Array.from(byHash.entries())
      .filter(([, group]) => group.length > 1)
      .map(([key, group]) => {
        const [sizeText, hash] = key.split(':', 2);
        const size = Number(sizeText) || group[0]?.size || 0;
        const sorted = [...group].sort((a, b) =>
          (a.modified - b.modified) || a.path.length - b.path.length || a.path.localeCompare(b.path));
        return {
          hash,
          size,
          files: sorted,
          reclaimableBytes: size * (sorted.length - 1),
          suggestedKeeperPath: sorted[0].path,
        } satisfies DuplicateGroup;
      })
      .sort((a, b) => b.reclaimableBytes - a.reclaimableBytes || a.files[0].path.localeCompare(b.files[0].path));

    return {
      root,
      groups,
      scannedFiles: files.length,
      hashedFiles,
      reclaimableBytes: groups.reduce((sum, group) => sum + group.reclaimableBytes, 0),
      truncated,
      warnings,
    };
  }

  keeperPath(group: DuplicateGroup, policy: DuplicateKeeperPolicy = 'oldest'): string {
    if (!group.files.length) return '';
    const files = [...group.files];
    if (policy === 'newest') {
      files.sort((a, b) => (b.modified - a.modified) || a.path.length - b.path.length || a.path.localeCompare(b.path));
      return files[0].path;
    }
    if (policy === 'shortest-path') {
      files.sort((a, b) => a.path.length - b.path.length || (a.modified - b.modified) || a.path.localeCompare(b.path));
      return files[0].path;
    }
    files.sort((a, b) => (a.modified - b.modified) || a.path.length - b.path.length || a.path.localeCompare(b.path));
    return files[0].path;
  }

  redundantPaths(groups: readonly DuplicateGroup[], policy: DuplicateKeeperPolicy = 'oldest'): string[] {
    return groups.flatMap(group => {
      const keeper = this.keeperPath(group, policy);
      return group.files.filter(file => file.path !== keeper).map(file => file.path);
    });
  }

  suggestedRedundantPaths(groups: readonly DuplicateGroup[]): string[] {
    return this.redundantPaths(groups, 'oldest');
  }

  private messageOf(error: unknown): string {
    return error instanceof Error ? error.message : String(error ?? 'Unknown error');
  }
}