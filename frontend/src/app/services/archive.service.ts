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
  stripPrefix?: string;
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

// FIX: added .tbz2 and .txz which the tar handler registers but were missing,
// causing those files to be treated as plain files and nested-chain parsing to
// break for paths containing them.
export function isArchive(name: string): boolean {
  const l = name.toLowerCase();
  return (
    l.endsWith('.zip') ||
    l.endsWith('.tar') ||
    l.endsWith('.tgz') ||
    l.endsWith('.tar.gz') ||
    l.endsWith('.tar.bz2') ||
    l.endsWith('.tbz2') ||    // added
    l.endsWith('.tar.xz') ||
    l.endsWith('.txz') ||     // added
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

  const app = window.go?.app?.App;
  if (!app) throw new Error('Wails bridge not available');

  return app.ArchiveBrowse(fsPath, nestedChain, innerPath);
}

/* ─────────────────────────────────────────────────────────────
   PATH UTILITIES
───────────────────────────────────────────────────────────── */

/**
 * Decomposes a global archive path like
 *   "filesystem://c/foo/bar.zip/nested.zip/dir/file.txt"
 * into the three components expected by the backend:
 *   fsPath      = "filesystem://c/foo/bar.zip"
 *   nestedChain = ["nested.zip"]
 *   innerPath   = "dir/file.txt"
 */
export function parseArchivePath(globalPath: string): { fsPath: string; nestedChain: string[]; innerPath: string } {
  const norm = globalPath.replace(/\\/g, '/');
  const parts = norm.split('/');

  const firstIdx = parts.findIndex(p => isArchive(p));
  if (firstIdx === -1) return { fsPath: globalPath, nestedChain: [], innerPath: '' };

  const fsPath = parts.slice(0, firstIdx + 1).join('/');
  const rest = parts.slice(firstIdx + 1);

  // Accumulate segments; each time we hit an archive name it closes a nestedChain entry.
  const nestedChain: string[] = [];
  const accum: string[] = [];
  for (const seg of rest) {
    accum.push(seg);
    if (isArchive(seg)) {
      nestedChain.push(accum.join('/'));
      accum.length = 0;
    }
  }
  return { fsPath, nestedChain, innerPath: accum.join('/') };
}

/**
 * Builds a global archive path from its three components.
 * Empty parts are omitted so trailing slashes are avoided.
 */
export function buildArchivePath(fsPath: string, nestedChain: string[], innerPath: string): string {
  return [fsPath.replace(/\\/g, '/'), ...nestedChain, innerPath]
    .filter(Boolean)
    .join('/')
    .replace(/\/+/g, '/');
}

/* ─────────────────────────────────────────────────────────────
   SERVICE
───────────────────────────────────────────────────────────── */

@Injectable({ providedIn: 'root' })
export class ArchiveService {
  /**
   * Extracts entries from an archive. If the user selects all top-level entries (the root),
   * it uses BatchExtract for efficiency. Otherwise, it uses BatchSelectiveExtract for a subset.
   * @param frame The current archive frame
   * @param entries The selected entry paths (relative to the archive root)
   * @param destDir The destination directory for extraction
   * @param flattenPaths Whether to flatten extracted paths
   */
  async extractEntries(
    frame: ArchiveFrame,
    entries: string[],
    destDir: string,
    flattenPaths = false
  ): Promise<any> {
    // 1. Filter out the ".." parent entry
    const filteredEntries = entries.filter(e => e !== PARENT_ENTRY_PATH);
    
    // 2. Get all entries currently visible in this frame
    const allInView = frame.listing.entries
      .map(e => e.path)
      .filter(e => e !== PARENT_ENTRY_PATH);

    // 3. Check if we are at the true root of the physical archive
    const isAtArchiveRoot = frame.nestedChain.length === 0 && !frame.innerPath;

    // 4. Check if everything in the current view is selected
    const isAllSelected = 
      filteredEntries.length > 0 &&
      filteredEntries.length === allInView.length &&
      filteredEntries.every(e => allInView.includes(e));

    // Fallback to BatchExtract ONLY if at the root and everything is selected
    if (isAtArchiveRoot && isAllSelected) {
      return window.go.app.App.BatchExtract({
        archives: [frame.fsPath],
        destinationDir: destDir,
        createSubdirectory: false,
        expandNestedArchives: false,
        flattenSingleRoot: flattenPaths,
      });
    } 
    
    // Otherwise, use Selective Extraction to ensure only the 
    // contents of the current folder/selection are processed
    return window.go.app.App.BatchSelectiveExtract({
      items: [
        {
          archivePath: frame.fsPath,
          nestedChain: frame.nestedChain,
          entries: filteredEntries,
          stripPrefix: frame.innerPath || '',
          destinationDir: destDir,
          flattenPaths,
        },
      ],
      destinationDir: destDir,
    });
  }

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

  copy(
    frame: ArchiveFrame,
    entries: string[],
    destDir: string,
    flattenPaths = false,
  ): Promise<ArchiveEntriesResult> {

    const req: ArchiveEntriesRequest = {
      fsPath: frame.fsPath,
      nestedChain: frame.nestedChain,
      entries,
      stripPrefix: frame.innerPath || '',
      destinationDir: destDir,
      flattenPaths,
    };

    return window.go.app.App.ArchiveCopyEntries(req);
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
      entries,
      stripPrefix: frame.innerPath || '',
      destinationDir: destDir,
      flattenPaths,
    };

    return window.go.app.App.ArchiveMoveEntries(req);
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
      entries,
      destinationDir: '',
      flattenPaths: false,
    };

    return window.go.app.App.ArchiveDeleteEntries(req);
  }

  copyByContext(
    archivePath: string,
    innerPaths: string[],
    destDir: string,
  ): Promise<ArchiveEntriesResult> {
    const { fsPath, nestedChain } = parseArchivePath(archivePath);
    const req: ArchiveEntriesRequest = {
      fsPath,
      nestedChain,
      entries: innerPaths,
      stripPrefix: '',
      destinationDir: destDir,
      flattenPaths: false,
    };
    return window.go.app.App.ArchiveCopyEntries(req);
  }

  moveByContext(
    archivePath: string,
    innerPaths: string[],
    destDir: string,
  ): Promise<ArchiveEntriesResult> {
    const { fsPath, nestedChain } = parseArchivePath(archivePath);
    const req: ArchiveEntriesRequest = {
      fsPath,
      nestedChain,
      entries: innerPaths,
      stripPrefix: '',
      destinationDir: destDir,
      flattenPaths: false,
    };
    return window.go.app.App.ArchiveMoveEntries(req);
  }

  /* ─────────────── UTILITIES ─────────────── */

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
        modified: e.modified, // already UnixMilli from backend
        created: e.modified,
        accessed: e.modified,
        permissions:
          e.type === FileType.DIRECTORY
            ? 'drwxr-xr-x'
            : '-rw-r--r--',
        isHidden: e.name.startsWith('.'),
        // FIX: nestedChain is now populated for archive entries by the backend
        // (convertVFSInfoWithChain / vfsEntryToFileInfo) for all formats.
        nestedChain: (e as any).nestedChain ?? undefined,
      }));

    return [dotdot, ...entries];
  }
}