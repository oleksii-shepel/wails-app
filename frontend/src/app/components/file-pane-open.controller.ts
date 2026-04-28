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
  defaultOpenAction: 'edit' | 'external';
  entries: FileInfo[];
  apiService: ApiService;
  selectionCursorPath: string | null;
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
  openPreviewPath(path: string): Promise<void>;
  openEditorPath(path: string): Promise<void>;
}

export class FilePaneOpenController {
  private openExternalFile(host: FilePaneOpenHost, path: string): void {
    host.apiService.openFile(path).catch(err =>
      console.error('Failed to open file:', err)
    );
  }

  private async openByPreference(host: FilePaneOpenHost, path: string): Promise<void> {
    switch (host.defaultOpenAction) {
      case 'edit':
        await host.openEditorPath(path);
        return;
      case 'external':
      default:
        this.openExternalFile(host, path);
        return;
    }
  }

  private async openArchiveEntry(host: FilePaneOpenHost, path: string): Promise<void> {
    if (host.defaultOpenAction === 'edit') {
      await host.openEditorPath(path);
      return;
    }

    await host.openPreviewPath(path);
  }

  private async openDetectedArchiveOrFile(host: FilePaneOpenHost, path: string): Promise<void> {
    try {
      const probe = await host.apiService.probeFile(path);
      if (probe?.isArchive) {
        await host.openRootArchive(path);
        return;
      }
    } catch {
      // fall back to the extension-based path below
    }

    await this.openByPreference(host, path);
  }

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
      void this.openDetectedArchiveOrFile(host, entry.path);
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
      } else {
        void this.openArchiveEntry(host, entry.path);
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

    void this.openDetectedArchiveOrFile(host, entry.path);
  }

  onOpenSelectedItem(host: FilePaneOpenHost, navigateUp: () => void): void {
    // Always use the entry at selectionCursorPath for keyboard navigation (Enter)
    const cursorPath = host.selectionCursorPath;
    if (!cursorPath) {
      return;
    }
    const entry = host.entries.find(candidate => candidate.path === cursorPath);
    if (entry) {
      if (host.currentView === 'tree' && entry.type !== FileType.DIRECTORY) {
        this.onNodeDoubleClick(host, entry, navigateUp);
        return;
      }
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
      return;
    }

    if (host.isInArchive()) {
      await host.openEditorPath(entry.path);
      return;
    }

    await this.openDetectedArchiveOrFile(host, entry.path);
  }

  private resolveAndOpen(host: FilePaneOpenHost, path: string): void {
    host.apiService.resolveLink(path).then(result => {
      if (result.isDir) {
        void host.loadDirectory(result.target);
      } else {
        void this.openByPreference(host, result.target);
      }
    }).catch(err => {
      console.warn('Could not resolve link, falling back to OS open:', err);
      this.openExternalFile(host, path);
    });
  }
}
