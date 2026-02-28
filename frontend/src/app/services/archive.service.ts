import { Injectable } from '@angular/core';
import { ArchiveListing } from '../shared/protocol-enhanced';
import { FileInfo, FileType } from '../shared/protocol';
import { PARENT_ENTRY_PATH } from '../components/file-entry.component';

/* ─────────────────────────────────────────────────────────────
   TYPES
───────────────────────────────────────────────────────────── */

export interface ArchiveFrame {
  fsPath: string;
  nestedChain: string[];
  innerPath: string;
  listing: ArchiveListing;
}

export interface ArchiveBrowseTarget {
  frame: ArchiveFrame;
  browseChain: string[];
  browseInnerPath: string;
}

export interface ArchiveEntriesRequest {
  fsPath: string;
  nestedChain: string[];
  entries: string[];
  destinationDir: string;
  flattenPaths: boolean;
}

export interface ArchiveEntriesResult {
  operationId: string;
  total: number;
  succeeded: number;
  failed: number;
  sourceRemoved: boolean;
  errors?: Array<{ source: string; error: string }>;
}

/* ─────────────────────────────────────────────────────────────
   HELPERS
───────────────────────────────────────────────────────────── */

export function isArchive(name: string): boolean {
  const l = name.toLowerCase();
  return (
    l.endsWith('.zip') ||
    l.endsWith('.tar') ||
    l.endsWith('.tgz') ||
    l.endsWith('.tar.gz') ||
    l.endsWith('.tar.bz2') ||
    l.endsWith('.tar.xz') ||
    l.endsWith('.gz') ||
    l.endsWith('.bz2') ||
    l.endsWith('.xz') ||
    l.endsWith('.rar') ||
    l.endsWith('.7z')
  );
}

export function buildArchiveDisplayPath(
  rootFsPath: string,
  stack: ArchiveFrame[]
): string {
  if (stack.length === 0) return rootFsPath;

  const frame = stack[stack.length - 1];
  const archiveFsPath = stack[0].fsPath.replace(/\\/g, '/');
  const parts = [archiveFsPath, ...frame.nestedChain];

  if (frame.innerPath)
    parts.push(frame.innerPath);

  return parts.join('/').replace(/\/+/g, '/');
}

export function buildArchiveTabTitle(
  rootFsPath: string,
  stack: ArchiveFrame[]
): string {

  if (stack.length === 0)
    return rootFsPath.split('/').filter(Boolean).pop() ?? rootFsPath;

  const frame = stack[stack.length - 1];

  if (frame.innerPath)
    return frame.innerPath.split('/').filter(Boolean).pop() ?? frame.innerPath;

  if (frame.nestedChain.length > 0)
    return frame.nestedChain[frame.nestedChain.length - 1];

  return rootFsPath.split('/').filter(Boolean).pop() ?? rootFsPath;
}

/* ─────────────────────────────────────────────────────────────
   WAILS BRIDGE
───────────────────────────────────────────────────────────── */

declare const window: any;

async function goBrowse(
  fsPath: string,
  nestedChain: string[],
  innerPath: string
): Promise<ArchiveListing> {

  const app = window.go?.main?.App;
  if (!app) throw new Error('Wails bridge not available');

  if (app.ArchiveBrowse)
    return app.ArchiveBrowse(fsPath, nestedChain, innerPath);

  if (nestedChain.length === 0)
    return app.ArchiveList(fsPath, innerPath);

  return app.ArchiveListNested(fsPath, nestedChain, innerPath);
}

/* ─────────────────────────────────────────────────────────────
   SERVICE
───────────────────────────────────────────────────────────── */

@Injectable({ providedIn: 'root' })
export class ArchiveService {

  /* ─────────────── OPEN ─────────────── */

  async openRootArchive(fsPath: string): Promise<ArchiveFrame> {
    const listing = await goBrowse(fsPath, [], '');
    return { fsPath, nestedChain: [], innerPath: '', listing };
  }

  async openNestedArchive(
    frame: ArchiveFrame,
    entryNestedChain: string[]
  ): Promise<ArchiveFrame> {

    const listing = await goBrowse(
      frame.fsPath,
      entryNestedChain,
      ''
    );

    return {
      fsPath: frame.fsPath,
      nestedChain: entryNestedChain,
      innerPath: '',
      listing
    };
  }

  /* ─────────────── NAVIGATION ─────────────── */

  async navigateInto(
    frame: ArchiveFrame,
    folderInnerPath: string
  ): Promise<ArchiveFrame> {

    const listing = await goBrowse(
      frame.fsPath,
      frame.nestedChain,
      folderInnerPath
    );

    return { ...frame, innerPath: folderInnerPath, listing };
  }

  async navigateUp(frame: ArchiveFrame): Promise<ArchiveFrame | null> {

    if (!frame.innerPath)
      return null;

    const parentInnerPath = frame.innerPath.includes('/')
      ? frame.innerPath.replace(/\/[^/]+$/, '')
      : '';

    const listing = await goBrowse(
      frame.fsPath,
      frame.nestedChain,
      parentInnerPath
    );

    return { ...frame, innerPath: parentInnerPath, listing };
  }

  /* ─────────────── UNIVERSAL RESOLVER ─────────────── */

  resolveBrowseTarget(
    seg: { name: string; fullPath: string },
    stack: ArchiveFrame[]
  ): ArchiveBrowseTarget | null {

    if (!stack?.length)
      return null;

    const archiveBase = stack[0].fsPath.replace(/\\/g, '/');
    const vNorm = seg.fullPath.replace(/\\/g, '/');

    const afterArchive = vNorm.startsWith(archiveBase)
      ? vNorm.slice(archiveBase.length).replace(/^\/+/, '')
      : '';

    const parts = afterArchive.split('/').filter(Boolean);

    let owningFrame = stack[0];

    for (const frame of stack) {

      const chain = frame.nestedChain;

      const matches =
        chain.length <= parts.length &&
        chain.every((c, i) => c === parts[i]);

      if (matches && chain.length >= owningFrame.nestedChain.length)
        owningFrame = frame;
    }

    const chain = owningFrame.nestedChain;
    let innerParts = parts.slice(chain.length);

    if (isArchive(seg.name))
      innerParts = innerParts.slice(1);

    return {
      frame: owningFrame,
      browseChain: chain,
      browseInnerPath: innerParts.join('/')
    };
  }

  /* ─────────────── OPERATIONS ─────────────── */

  /**
   * Ensures every entry path is a full inner path from the archive root.
   *
   * When browsing inside a subdirectory (frame.innerPath = "some/dir"), the
   * listing entries carry short paths like "file.txt" rather than the full
   * "some/dir/file.txt" the backend needs for matchesEntries() to locate them.
   * We prepend frame.innerPath when it is not already present.
   */
  private resolveEntries(frame: ArchiveFrame, entries: string[]): string[] {
    const prefix = frame.innerPath ? frame.innerPath + '/' : '';
    if (!prefix) return entries;
    return entries.map(e => e.startsWith(prefix) ? e : prefix + e);
  }

  copy(
    frame: ArchiveFrame,
    entries: string[],
    destDir: string,
    flattenPaths = false,
  ): Promise<ArchiveEntriesResult> {

    const req: ArchiveEntriesRequest = {
      fsPath: frame.fsPath,
      nestedChain: frame.nestedChain,
      entries: this.resolveEntries(frame, entries),
      destinationDir: destDir,
      flattenPaths,
    };

    return window.go.main.App.ArchiveCopyEntries(req);
  }

  move(
    frame: ArchiveFrame,
    entries: string[],
    destDir: string,
    flattenPaths = false,
  ): Promise<ArchiveEntriesResult> {

    const req: ArchiveEntriesRequest = {
      fsPath: frame.fsPath,
      nestedChain: frame.nestedChain,
      entries: this.resolveEntries(frame, entries),
      destinationDir: destDir,
      flattenPaths,
    };

    return window.go.main.App.ArchiveMoveEntries(req);
  }

  delete(
    frame: ArchiveFrame,
    entries: string[],
  ): Promise<ArchiveEntriesResult> {

    if (frame.nestedChain.length > 0) {
      return Promise.reject(new Error(
        'Deleting entries from nested archives is not supported. ' +
        'Navigate to the top-level archive first.'
      ));
    }

    const req: ArchiveEntriesRequest = {
      fsPath: frame.fsPath,
      nestedChain: [],
      entries: this.resolveEntries(frame, entries),
      destinationDir: '',
      flattenPaths: false,
    };

    return window.go.main.App.ArchiveDeleteEntries(req);
  }

  /* ─────────────── UTILITIES ─────────────── */

  parseVirtualPath(path: string): { fsPath: string; chain: string[] } {

    const normalized = path.replace(/\\/g, '/');
    const parts = normalized.split('/');

    const archiveIndex =
      parts.findIndex(p => /\.(zip|tar|rar|7z)$/i.test(p));

    if (archiveIndex === -1)
      return { fsPath: normalized, chain: [] };

    return {
      fsPath: parts.slice(0, archiveIndex + 1).join('/'),
      chain: parts.slice(archiveIndex + 1)
    };
  }

  isArchiveVirtualPath(path: string): boolean {
    return /\.zip\//i.test(path)
        || /\.tar\//i.test(path)
        || /\.rar\//i.test(path)
        || /\.7z\//i.test(path);
  }

  async refreshFrame(frame: ArchiveFrame): Promise<ArchiveFrame> {

    const listing = await goBrowse(
      frame.fsPath,
      frame.nestedChain,
      frame.innerPath
    );

    return { ...frame, listing };
  }

  listingToFileInfos(listing: ArchiveListing): FileInfo[] {

    const dotdot: FileInfo = {
      name: '..',
      path: PARENT_ENTRY_PATH,
      type: FileType.DIRECTORY,
      size: 0,
      modified: 0,
      created: 0,
      accessed: 0,
      permissions: 'drwxr-xr-x',
      isHidden: false,
    };

    const entries: FileInfo[] =
      listing.entries.map(e => ({
        name: e.name,
        path: e.path,
        type: e.type as FileType,
        size: e.size,
        modified: e.modified,
        created: e.modified,
        accessed: e.modified,
        permissions:
          e.type === FileType.DIRECTORY
            ? 'drwxr-xr-x'
            : '-rw-r--r--',
        isHidden: e.name.startsWith('.'),
        nestedChain: (e as any).nestedChain ?? undefined,
      }));

    return [dotdot, ...entries];
  }
}