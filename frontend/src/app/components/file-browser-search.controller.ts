import { FileType } from '../shared/protocol';
import type { PathHistoryService } from '../services/path-history.service';

export interface FileBrowserSearchPane {
  onPathChange(path: string): void;
}

export interface FileBrowserSearchHost {
  pathHistoryService: PathHistoryService;
  getActivePane(): FileBrowserSearchPane | undefined;
}

export class FileBrowserSearchController {
  onPanePathChange(host: FileBrowserSearchHost, paneId: string, path: string): void {
    host.pathHistoryService.addPath(path, paneId);
  }

  onGlobalSearchResult(host: FileBrowserSearchHost, result: any): void {
    const pane = host.getActivePane();
    if (!pane) {
      return;
    }

    if (result.file.type === FileType.DIRECTORY) {
      pane.onPathChange(result.file.path);
      return;
    }

    pane.onPathChange(this.getParentPath(result.file.path));
  }

  private getParentPath(path: string): string {
    if (path === '/' || path === '') {
      return '/';
    }

    const parts = path.split('/').filter(part => part.length > 0);
    if (parts.length === 0) {
      return '/';
    }

    parts.pop();
    return '/' + parts.join('/');
  }
}