import type { ApiService } from '../services/api.service';
import type { FileInfo } from '../shared/protocol';
import { FileType } from '../shared/protocol';
import type { ViewLayout } from './file-view-container.component';
import { PARENT_ENTRY_PATH } from './file-entry.component';
import { parentPath } from './address-bar.utils';
import { isArchive } from '../services/archive.service';

export interface FilePaneOpenHost {
  currentPath: string;
  currentView: ViewLayout;
  entries: FileInfo[];
  apiService: ApiService;
  isInArchive(): boolean;
  exitArchive(): void;
  clearArchiveState(): void;
  archiveNavigateInto(path: string): Promise<void>;
  openNestedArchive(chain: string[]): Promise<void>;
  loadDirectory(path: string): Promise<void>;
  openRootArchive(path: string): Promise<void>;
  emitPathChange(path: string): void;
  getEffectiveSelection(): Set<string>;
  openPathInNewTab(path: string): Promise<void>;
}

export class FilePaneOpenController {
  onNodeDoubleClick(host: FilePaneOpenHost, entry: FileInfo, navigateUp: () => void): void {
    if (entry.path === PARENT_ENTRY_PATH) {
      navigateUp();
      return;
    }

    if (host.currentView === 'tree' && entry.type !== FileType.DIRECTORY) {
      if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
        void host.openRootArchive(entry.path);
        return;
      }
      if (entry.name.toLowerCase().endsWith('.lnk') || entry.type === FileType.SYMLINK) {
        this.resolveAndOpen(host, entry.path);
        return;
      }
      host.apiService.openFile(entry.path).catch(err =>
        console.error('Failed to open file:', err)
      );
      return;
    }

    this.onNavigate(host, entry, navigateUp);
  }

  onNavigate(host: FilePaneOpenHost, entry: FileInfo, navigateUp: () => void): void {
    if (entry.path === PARENT_ENTRY_PATH) {
      navigateUp();
      return;
    }

    if (host.currentView === 'tree' && entry.type === FileType.DIRECTORY) {
      if (host.isInArchive()) {
        host.exitArchive();
        host.clearArchiveState();
      }
      host.currentPath = entry.path;
      host.emitPathChange(entry.path);
      return;
    }

    if (host.currentView === 'tree' && (entry.type === FileType.FILE || entry.type === FileType.SYMLINK || entry.type === FileType.ARCHIVE)) {
      const parent = parentPath(entry.path);
      host.currentPath = parent;
      host.emitPathChange(parent);
      return;
    }

    if (host.isInArchive()) {
      if (entry.type === FileType.DIRECTORY) {
        void host.archiveNavigateInto(entry.path);
      } else if (entry.type === FileType.ARCHIVE) {
        const chain: string[] = (entry as any).nestedChain;
        if (chain?.length) {
          void host.openNestedArchive(chain);
        }
      }
      return;
    }

    if (entry.type === FileType.DIRECTORY) {
      void host.loadDirectory(entry.path);
      return;
    }
    if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
      void host.openRootArchive(entry.path);
      return;
    }
    if (entry.name.toLowerCase().endsWith('.lnk') || entry.type === FileType.SYMLINK) {
      this.resolveAndOpen(host, entry.path);
      return;
    }

    host.apiService.openFile(entry.path).catch(err =>
      console.error('Failed to open file:', err)
    );
  }

  onOpenSelectedItem(host: FilePaneOpenHost, navigateUp: () => void): void {
    const selected = Array.from(host.getEffectiveSelection());
    if (selected.length !== 1) {
      return;
    }

    const entry = host.entries.find(candidate => candidate.path === selected[0]);
    if (entry) {
      this.onNavigate(host, entry, navigateUp);
    }
  }

  async onOpenInNewTab(host: FilePaneOpenHost, entry: FileInfo): Promise<void> {
    if (entry.path === PARENT_ENTRY_PATH) {
      const up = host.isInArchive() ? host.currentPath : parentPath(host.currentPath);
      await host.openPathInNewTab(up);
      return;
    }

    if (entry.type === FileType.DIRECTORY) {
      await host.openPathInNewTab(entry.path);
      return;
    }

    if (entry.type === FileType.ARCHIVE || isArchive(entry.name)) {
      await host.openPathInNewTab(entry.path);
    }
  }

  private resolveAndOpen(host: FilePaneOpenHost, path: string): void {
    host.apiService.resolveLink(path).then(result => {
      if (result.isDir) {
        void host.loadDirectory(result.target);
      } else {
        host.apiService.openFile(result.target).catch(err =>
          console.error('Failed to open link target:', err)
        );
      }
    }).catch(err => {
      console.warn('Could not resolve link, falling back to OS open:', err);
      host.apiService.openFile(path).catch(openErr =>
        console.error('Failed to open file:', openErr)
      );
    });
  }
}