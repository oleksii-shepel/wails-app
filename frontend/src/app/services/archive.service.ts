import { Injectable } from '@angular/core';
import { ArchiveListing, ArchiveEntry } from '../shared/protocol-enhanced';
import { FileInfo, FileType } from '../shared/protocol';

export interface ArchiveFrame {
  fsPath: string;           // Physical path on disk
  innerPath: string;        // Current path inside the archive
  listing: ArchiveListing;  // Current entries
}

/** Checks if a file is an archive (zip, tar, tgz, etc.) */
export function isArchive(name: string): boolean {
  const l = name.toLowerCase();
  return l.endsWith('.zip') || l.endsWith('.tar') || l.endsWith('.tgz') ||
         l.endsWith('.tar.gz') || l.endsWith('.tar.bz2') || l.endsWith('.tar.xz');
}

/** Builds a virtual display path for nested archives */
export function buildArchiveDisplayPath(rootFsPath: string, stack: ArchiveFrame[]): string {
  const parts: string[] = [rootFsPath];
  for (const frame of stack) {
    if (frame.innerPath) parts.push(frame.innerPath);
  }
  return parts.join('/');
}

declare const window: any;

@Injectable({ providedIn: 'root' })
export class ArchiveService {

  constructor() {}

  /** Open a top-level archive on the filesystem */
  async openRootArchive(fsPath: string): Promise<ArchiveFrame> {
    try {
      const listing: ArchiveListing = await window.go.main.App.ArchiveList(fsPath, '');
      return { fsPath, innerPath: '', listing };
    } catch (err: any) {
      throw new Error(err?.message || 'Failed to open archive');
    }
  }

  /** Open a nested archive inside a current frame */
  async openNestedArchive(
    frame: ArchiveFrame,
    entryInnerPath: string,
    entryName: string
  ): Promise<ArchiveFrame> {
    try {
      const listing: ArchiveListing = await window.go.main.App.ArchiveListNested(
        frame.fsPath,
        entryInnerPath,
        ''
      );
      return {
        fsPath: frame.fsPath,
        innerPath: entryInnerPath,
        listing
      };
    } catch (err: any) {
      throw new Error(err?.message || `Failed to open nested archive: ${entryName}`);
    }
  }

  /** Navigate into a folder inside the current archive frame */
  async navigateInto(frame: ArchiveFrame, folderName: string): Promise<ArchiveFrame> {
    const targetPath = frame.innerPath
      ? `${frame.innerPath}/${folderName}`
      : folderName;
    try {
      const listing: ArchiveListing = await window.go.main.App.ArchiveListNested(
        frame.fsPath,
        targetPath,
        ''
      );
      return { ...frame, innerPath: targetPath, listing };
    } catch (err: any) {
      throw new Error(err?.message || `Failed to navigate into folder: ${folderName}`);
    }
  }

  /** Refresh the current archive frame (re-read contents from backend) */
  async refreshFrame(frame: ArchiveFrame): Promise<ArchiveFrame> {
    try {
      const listing: ArchiveListing = frame.innerPath
        ? await window.go.main.App.ArchiveListNested(frame.fsPath, frame.innerPath, '')
        : await window.go.main.App.ArchiveList(frame.fsPath, '');
      return { ...frame, listing };
    } catch (err: any) {
      throw new Error(err?.message || 'Failed to refresh archive');
    }
  }

  /** Convert ArchiveListing entries to FileInfo array for Angular pane */
  public listingToFileInfos(listing: ArchiveListing): FileInfo[] {
    const files: FileInfo[] = listing.entries.map((e: ArchiveEntry) => ({
      name: e.name,
      path: e.innerPath,
      type: e.type === FileType.DIRECTORY ? FileType.DIRECTORY : FileType.FILE,
      size: e.size,
      modified: e.modified,
      permissions: e.type === FileType.DIRECTORY ? 'drwxr-xr-x' : '-rw-r--r--',
      isHidden: e.name.startsWith('.'),
      created: e.modified,
      accessed: e.modified
    }));
    // Prepend '..' for navigating up in archive
    return [{
      name: '..',
      path: '__archive_parent__',
      type: FileType.DIRECTORY,
      size: 0,
      modified: 0,
      permissions: 'drwxr-xr-x',
      isHidden: false,
      created: 0,
      accessed: 0
    }, ...files];
  }
}
