import { Injectable } from '@angular/core';
import { ArchiveListing } from '../shared/protocol-enhanced';
import { FileInfo, FileType } from '../shared/protocol';

// ─── Types ───────────────────────────────────────────────────────────────────

export interface ArchiveFrame {
  /** Absolute path to the on-disk archive file. Never changes within a stack. */
  fsPath: string;
  /**
   * Chain of archive-entry names that leads to the *current* archive.
   * Empty for a top-level (filesystem) archive.
   * e.g. [] → top-level zip
   *      ['inner.zip'] → a zip nested one level deep
   *      ['a.tar.gz', 'b.zip'] → zip nested two levels deep
   */
  nestedChain: string[];
  /** Current sub-directory path inside the innermost archive. '' = root. */
  innerPath: string;
  /** Cached listing for this frame. */
  listing: ArchiveListing;
}

// ─── Pure helpers (exported so components can use them) ───────────────────────

/** Returns true if the file name looks like a supported archive format. */
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
    l.endsWith('.xz')
  );
}

/**
 * Build the full virtual path shown in the address bar and stored as the tab path.
 *
 * Examples:
 *   C:/Desktop.zip
 *   C:/Desktop.zip/src
 *   C:/Desktop.zip/progress.zip
 *   C:/Desktop.zip/progress.zip/docs
 */
export function buildArchiveDisplayPath(rootFsPath: string, stack: ArchiveFrame[]): string {
  if (stack.length === 0) return rootFsPath;
  const frame = stack[stack.length - 1];
  // rootFsPath already ends with the archive filename (e.g. C:/Desktop.zip)
  const parts = [rootFsPath, ...frame.nestedChain];
  if (frame.innerPath) parts.push(frame.innerPath);
  // Join with / — rootFsPath may already end with / on some platforms so normalise
  return parts.join('/').replace(/\/+/g, '/');
}

/**
 * Build the tab display title — just the last meaningful segment of the virtual path.
 *
 * Examples:
 *   C:/Desktop.zip            →  Desktop.zip
 *   C:/Desktop.zip/src        →  src
 *   C:/Desktop.zip/progress.zip          →  progress.zip
 *   C:/Desktop.zip/progress.zip/docs     →  docs
 */
export function buildArchiveTabTitle(rootFsPath: string, stack: ArchiveFrame[]): string {
  if (stack.length === 0) {
    return rootFsPath.split('/').filter(Boolean).pop() ?? rootFsPath;
  }
  const frame = stack[stack.length - 1];
  // Innermost sub-directory takes priority
  if (frame.innerPath) {
    return frame.innerPath.split('/').filter(Boolean).pop() ?? frame.innerPath;
  }
  // Deepest nested archive in the chain
  if (frame.nestedChain.length > 0) {
    return frame.nestedChain[frame.nestedChain.length - 1];
  }
  // At the root of the top-level archive
  return rootFsPath.split('/').filter(Boolean).pop() ?? rootFsPath;
}

// ─── Wails Go bridge ─────────────────────────────────────────────────────────

declare const window: any;

// Unified call that matches the new ArchiveBrowse method on the Go side.
// Falls back to the old split API so this works during migration too.
async function goBrowse(
  fsPath: string,
  nestedChain: string[],
  innerPath: string
): Promise<ArchiveListing> {
  const app = window.go?.main?.App;
  if (!app) throw new Error('Wails bridge not available');

  // Prefer the new unified method; fall back to old split API.
  if (app.ArchiveBrowse) {
    return app.ArchiveBrowse(fsPath, nestedChain, innerPath);
  }
  if (nestedChain.length === 0) {
    return app.ArchiveList(fsPath, innerPath);
  }
  return app.ArchiveListNested(fsPath, nestedChain, innerPath);
}

// ─── Service ─────────────────────────────────────────────────────────────────

@Injectable({ providedIn: 'root' })
export class ArchiveService {

  // ── Opening ────────────────────────────────────────────────────────────────

  /** Open a top-level (filesystem) archive. */
  async openRootArchive(fsPath: string): Promise<ArchiveFrame> {
    const listing = await goBrowse(fsPath, [], '');
    return { fsPath, nestedChain: [], innerPath: '', listing };
  }

  /**
   * Open a nested archive that lives *inside* the current frame.
   * `entry` must be a FileInfo whose type is ARCHIVE (or FILE with an archive name).
   * The entry's nestedChain from the Go response is the authoritative chain.
   */
  async openNestedArchive(
    frame: ArchiveFrame,
    entryNestedChain: string[]   // comes from FileInfo.nestedChain returned by Go
  ): Promise<ArchiveFrame> {
    const listing = await goBrowse(frame.fsPath, entryNestedChain, '');
    return { fsPath: frame.fsPath, nestedChain: entryNestedChain, innerPath: '', listing };
  }

  // ── Navigation inside an archive ──────────────────────────────────────────

  /**
   * Navigate into a sub-directory within the *same* archive level.
   * `folderInnerPath` is the full inner path of the target folder
   * (i.e. what was stored in FileInfo.path by the Go backend).
   */
  async navigateInto(frame: ArchiveFrame, folderInnerPath: string): Promise<ArchiveFrame> {
    const listing = await goBrowse(frame.fsPath, frame.nestedChain, folderInnerPath);
    return { ...frame, innerPath: folderInnerPath, listing };
  }

  /**
   * Navigate to the parent directory inside the current archive level.
   * Returns null if already at the archive root (caller should pop the stack).
   */
  async navigateUp(frame: ArchiveFrame): Promise<ArchiveFrame | null> {
    if (!frame.innerPath) return null;  // already at archive root

    const parentInnerPath = frame.innerPath.includes('/')
      ? frame.innerPath.replace(/\/[^/]+$/, '')
      : '';

    const listing = await goBrowse(frame.fsPath, frame.nestedChain, parentInnerPath);
    return { ...frame, innerPath: parentInnerPath, listing };
  }

  // ── Refresh ────────────────────────────────────────────────────────────────

  async refreshFrame(frame: ArchiveFrame): Promise<ArchiveFrame> {
    const listing = await goBrowse(frame.fsPath, frame.nestedChain, frame.innerPath);
    return { ...frame, listing };
  }

  // ── Conversion ────────────────────────────────────────────────────────────

  /**
   * Convert an ArchiveListing into the FileInfo array expected by FileListComponent.
   * Prepends a '..' entry so the user can navigate up.
   */
  listingToFileInfos(listing: ArchiveListing): FileInfo[] {
    const dotdot: FileInfo = {
      name: '..',
      path: '__archive_parent__',
      type: FileType.DIRECTORY,
      size: 0, modified: 0, created: 0, accessed: 0,
      permissions: 'drwxr-xr-x',
      isHidden: false,
    };

    const entries: FileInfo[] = listing.entries.map(e => ({
      name: e.name,
      path: e.path,           // Go returns "path", not "innerPath"
      type: e.type as FileType,
      size: e.size,
      modified: e.modified,
      created: e.modified,
      accessed: e.modified,
      permissions: e.type === FileType.DIRECTORY ? 'drwxr-xr-x' : '-rw-r--r--',
      isHidden: e.name.startsWith('.'),
      // Preserve the nestedChain so the pane can open nested archives correctly.
      nestedChain: (e as any).nestedChain ?? undefined,
    }));

    return [dotdot, ...entries];
  }
}