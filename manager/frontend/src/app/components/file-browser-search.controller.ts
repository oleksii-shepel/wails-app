import { FileType } from '../shared/protocol';
import type { PathHistoryService } from '../services/path-history.service';
import type { ApiService } from '../services/api.service';
import type { SearchResultItem } from '../services/global-search.service';
import { parentPath } from './address-bar.utils';

export interface FileBrowserSearchPane {
  currentPath: string;
  onPathChange(path: string): void;
  openSearchResult(result: SearchResultItem): Promise<void>;
  openSearchSurface(location: string, query: string): Promise<void>;
}

export interface FileBrowserSearchHost {
  pathHistoryService: PathHistoryService;
  apiService: ApiService;
  getActivePane(): FileBrowserSearchPane | undefined;
}

export class FileBrowserSearchController {
  onPanePathChange(host: FileBrowserSearchHost, paneId: string, path: string): void {
    host.pathHistoryService.addPath(path, paneId);
  }

  async openGlobalSearchAsFolder(host: FileBrowserSearchHost, query: string): Promise<void> {
    const pane = host.getActivePane();
    if (!pane) {
      return;
    }

    const trimmedQuery = query.trim();
    if (!trimmedQuery) {
      return;
    }

    const aliasPath = await host.apiService.buildSearchAlias(pane.currentPath, trimmedQuery, true);
    await pane.openSearchSurface(aliasPath, trimmedQuery);
  }

  async onGlobalSearchResult(host: FileBrowserSearchHost, result: SearchResultItem): Promise<void> {
    const pane = host.getActivePane();
    if (!pane) {
      return;
    }

    if (result.file.type === FileType.DIRECTORY) {
      await pane.openSearchResult(result);
      return;
    }

    const targetDir = parentPath(result.file.path);
    if (targetDir === result.file.path) {
      pane.onPathChange(targetDir);
      return;
    }

    await pane.openSearchResult(result);
  }
}