import { Injectable } from '@angular/core';

import { DiscoverySearchService } from './discovery-search.service';
import { DuplicateFinderService } from './duplicate-finder.service';
import { RecentFilesService } from './recent-files.service';
import {
  ResultsPanelService,
  type ResultsPanelDefinition,
} from './results-panel.service';
import type { FileInfo } from '../shared/protocol-enhanced';

@Injectable({ providedIn: 'root' })
export class SmartResultsService {
  constructor(
    private readonly panels: ResultsPanelService,
    private readonly discovery: DiscoverySearchService,
    private readonly recentFiles: RecentFilesService,
    private readonly duplicates: DuplicateFinderService,
  ) {}

  attachBuiltInRefresh(path: string): boolean {
    const panel = this.panels.get(path);
    if (!panel || this.panels.canRefresh(path)) {
      return !!panel && this.panels.canRefresh(path);
    }

    const kind = panel.metadata.kind ?? '';
    const origin = panel.metadata.origin ?? '';

    if (origin === 'global-search' ||
        kind === 'search' ||
        kind === 'recursive-search') {
      const root = panel.metadata.rootPath?.trim() ?? '';
      const query = panel.metadata.query?.trim() ?? '';
      if (!root || !query) return false;

      this.panels.registerRefresher(path, async () => {
        const result = await this.discovery.search(root, query, {
          recursive: kind === 'recursive-search',
          maxResults: 500,
        });
        return {
          entries: result.entries,
          metadata: {
            ...panel.metadata,
            refreshedAt: String(Date.now()),
            refreshWarnings: result.warnings.join('\n'),
          },
        };
      });
      return true;
    }

    if (origin === 'recent-files' || kind === 'recent-files') {
      this.panels.registerRefresher(path, async () => ({
        entries: this.recentFiles.recent(100),
      }));
      return true;
    }

    if (origin === 'duplicate-finder' || kind === 'duplicates') {
      const root = panel.metadata.rootPath?.trim() ?? '';
      if (!root) return false;

      this.panels.registerRefresher(path, async () => {
        const result = await this.duplicates.scan(root, {
          includeHidden: false,
        });

        return {
          entries: this.flattenDuplicateGroups(result.groups),
          metadata: {
            ...panel.metadata,
            reclaimableBytes: String(result.reclaimableBytes),
            duplicateGroups: String(result.groups.length),
            refreshWarnings: result.warnings.join('\n'),
          },
        };
      });
      return true;
    }

    return false;
  }

  ensureRefreshability(path: string): boolean {
    return this.panels.canRefresh(path) || this.attachBuiltInRefresh(path);
  }

  async refresh(path: string): Promise<ResultsPanelDefinition | null> {
    if (!this.ensureRefreshability(path)) return null;
    return this.panels.refresh(path);
  }

  private flattenDuplicateGroups(
    groups: readonly { files: readonly FileInfo[] }[],
  ): FileInfo[] {
    const seen = new Set<string>();
    const entries: FileInfo[] = [];

    for (const group of groups) {
      for (const entry of group.files) {
        const key = entry.path.replace(/\\/g, '/').toLocaleLowerCase();
        if (seen.has(key)) continue;
        seen.add(key);
        entries.push({ ...entry });
      }
    }

    return entries;
  }
}