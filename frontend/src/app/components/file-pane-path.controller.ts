import { FileType, type FileInfo } from '../shared/protocol';
import type { ArchiveFrame } from '../services/archive.service';
import type { FileViewDirectorySource } from '../services/directory-source.service';
import { PARENT_ENTRY_PATH } from './file-entry.component';

export interface FilePanePathHost {
  get currentPath(): string;
  get archiveStack(): ArchiveFrame[];
  directorySource: FileViewDirectorySource;
  isInArchive(): boolean;
  loadDirectory(path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void>;
  restoreArchiveFromPath(path: string): Promise<void>;
  renderArchiveFrame(frame: ArchiveFrame): void;
}

export class FilePanePathController {
  withParentEntry(raw: FileInfo[], inArchive: boolean, fsPath: string, directorySource: FileViewDirectorySource): FileInfo[] {
    if (raw.some(entry => entry.path === PARENT_ENTRY_PATH)) {
      return raw;
    }
    if (inArchive || !directorySource.isRootPath(fsPath)) {
      return [this.makeParentEntry(), ...raw];
    }
    return raw;
  }

  async loadPathTransparently(host: FilePanePathHost, path: string): Promise<void> {
    if (host.directorySource.isPathInsideArchive(path)) {
      await host.restoreArchiveFromPath(path);
      return;
    }

    await host.loadDirectory(path);
  }

  async refreshCurrentPathTransparently(host: FilePanePathHost): Promise<void> {
    if (!host.currentPath) {
      return;
    }

    if (host.isInArchive()) {
      const currentFrame = host.archiveStack[host.archiveStack.length - 1];
      if (currentFrame) {
        host.renderArchiveFrame(currentFrame);
        return;
      }
    }

    if (host.directorySource.isPathInsideArchive(host.currentPath)) {
      await host.restoreArchiveFromPath(host.currentPath);
      return;
    }

    await host.loadDirectory(host.currentPath, { preserveUI: true });
  }

  private makeParentEntry(): FileInfo {
    return {
      name: '..',
      path: PARENT_ENTRY_PATH,
      type: FileType.DIRECTORY,
      size: 0,
      modified: 0,
    } as FileInfo;
  }
}