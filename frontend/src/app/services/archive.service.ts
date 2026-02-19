import { Injectable } from '@angular/core';

declare const window: any;

// ─── Archive format detection ────────────────────────────────────────────────

const SINGLE_EXT = new Set([
  'zip', 'jar', 'war', 'ear', 'apk', 'ipa', 'xpi', 'crx', 'whl',
  'rar', 'r00', 'r01',
  '7z',
  'gz', 'bz2', 'xz', 'zst', 'lz4', 'lzma', 'lzo', 'br',
  'tar',
  'iso', 'img', 'dmg',
  'cab', 'msi',
  'deb', 'rpm',
  'arj', 'lzh', 'lha', 'ace', 'arc', 'zoo',
  'zz', 'z',
]);

const COMPOUND_EXT: string[] = [
  'tar.gz', 'tar.bz2', 'tar.xz', 'tar.zst', 'tar.lz4',
  'tar.lzma', 'tar.lzo', 'tar.br', 'tar.z',
  'tar.sz', 'tar.zz',
];

export function isArchive(name: string): boolean {
  if (!name) return false;
  const lower = name.toLowerCase();
  for (const ext of COMPOUND_EXT) {
    if (lower.endsWith('.' + ext)) return true;
  }
  const dot = lower.lastIndexOf('.');
  return dot !== -1 && SINGLE_EXT.has(lower.slice(dot + 1));
}

export function archiveExtension(name: string): string {
  const lower = name.toLowerCase();
  for (const ext of COMPOUND_EXT) {
    if (lower.endsWith('.' + ext)) return ext;
  }
  const dot = lower.lastIndexOf('.');
  return dot === -1 ? '' : lower.slice(dot + 1);
}

// ─── Protocol types (must match Go structs) ──────────────────────────────────

/**
 * One entry returned by Go's ArchiveList / ArchiveListNested.
 * Must match the Go struct `ArchiveEntry`.
 */
export interface ArchiveEntry {
  name: string;       // basename, e.g. "README.md"
  innerPath: string;  // full path inside the archive, e.g. "src/lib/README.md"
  type: 'FILE' | 'DIRECTORY';
  size: number;
  modified: number;   // unix milliseconds
}

/**
 * Response from Go's ArchiveList / ArchiveListNested.
 * Must match the Go struct `ArchiveListing`.
 */
export interface ArchiveListing {
  archivePath: string;  // outermost filesystem path
  innerPath: string;    // folder currently listed
  entries: ArchiveEntry[];
}

// ─── Navigation stack type ───────────────────────────────────────────────────

/**
 * One level in the archive navigation stack.
 *
 * stack[0]  — root archive opened directly from the filesystem:
 *   archiveFsPath   = "C:/stuff/outer.zip"
 *   nestedEntryPath = null
 *   innerPath       = current folder, e.g. "" or "src/lib"
 *
 * stack[1+] — nested archive opened from inside the parent:
 *   archiveFsPath   = same root FS path as stack[0] (Go always needs it)
 *   nestedEntryPath = innerPath of this archive inside the root,
 *                     e.g. "libs/inner.tar.gz"
 *   innerPath       = current folder within this nested archive
 */
export interface ArchiveFrame {
  /** Filesystem path of the outermost archive — constant throughout the whole stack. */
  archiveFsPath: string;
  /**
   * Path of this archive entry within the root archive.
   * null only for stack[0] (the root archive itself).
   */
  nestedEntryPath: string | null;
  /** Current folder being browsed. "" = root of this archive level. */
  innerPath: string;
  /** Cached listing — restored instantly when navigating back without a network call. */
  listing: ArchiveListing;
  /** Display name used in the address bar breadcrumb. */
  archiveName: string;
}

// ─── Service ─────────────────────────────────────────────────────────────────

@Injectable({ providedIn: 'root' })
export class ArchiveService {

  /**
   * Direct Wails bridge call — same pattern as ApiService.call().
   * No dependency on ApiService needed since we call Go directly.
   */
  private call<T>(method: string, ...args: unknown[]): Promise<T> {
    const bridge = window?.go?.main?.App;
    if (!bridge) {
      return Promise.reject(new Error('Wails bridge (window.go.main.App) not available.'));
    }
    if (typeof bridge[method] !== 'function') {
      return Promise.reject(new Error(`Go method '${method}' not found on Wails bridge.`));
    }
    return bridge[method](...args) as Promise<T>;
  }

  // ── Public API ─────────────────────────────────────────────────────────────

  /**
   * Open a root (filesystem) archive and return the initial frame.
   *
   * Go signature:
   *   func (a *App) ArchiveList(fsPath string, innerPath string) (ArchiveListing, error)
   */
  async openRootArchive(fsPath: string): Promise<ArchiveFrame> {
    const listing = await this.call<ArchiveListing>('ArchiveList', fsPath, '');
    return {
      archiveFsPath: fsPath,
      nestedEntryPath: null,
      innerPath: '',
      listing,
      archiveName: basename(fsPath),
    };
  }

  /**
   * Navigate into a sub-folder within the current archive level.
   * Fetches the listing for targetInnerPath and returns an updated frame.
   *
   * Uses ArchiveList for root archives, ArchiveListNested for nested ones.
   */
  async navigateInto(frame: ArchiveFrame, targetInnerPath: string): Promise<ArchiveFrame> {
    const listing = await this.fetchListing(
      frame.archiveFsPath,
      frame.nestedEntryPath,
      targetInnerPath,
    );
    return { ...frame, innerPath: targetInnerPath, listing };
  }

  /**
   * Open a nested archive entry that lives inside the current frame.
   *
   * Go signature:
   *   func (a *App) ArchiveListNested(
   *     fsPath string,        // outermost archive on disk
   *     nestedEntry string,   // path of the nested archive within fsPath
   *     innerPath string,     // folder to list within the nested archive
   *   ) (ArchiveListing, error)
   *
   * @param parentFrame     Frame that contains the nested archive entry.
   * @param entryInnerPath  innerPath of the nested archive within parentFrame.
   * @param entryName       Display name, e.g. "inner.tar.gz".
   */
  async openNestedArchive(
    parentFrame: ArchiveFrame,
    entryInnerPath: string,
    entryName: string,
  ): Promise<ArchiveFrame> {
    const listing = await this.call<ArchiveListing>(
      'ArchiveListNested',
      parentFrame.archiveFsPath,
      entryInnerPath,
      '',
    );
    return {
      archiveFsPath: parentFrame.archiveFsPath,
      nestedEntryPath: entryInnerPath,
      innerPath: '',
      listing,
      archiveName: entryName,
    };
  }

  /** Re-fetch the listing for an existing frame (refresh). */
  async refreshFrame(frame: ArchiveFrame): Promise<ArchiveFrame> {
    const listing = await this.fetchListing(
      frame.archiveFsPath,
      frame.nestedEntryPath,
      frame.innerPath,
    );
    return { ...frame, listing };
  }

  // ── Private ────────────────────────────────────────────────────────────────

  private fetchListing(
    archiveFsPath: string,
    nestedEntryPath: string | null,
    innerPath: string,
  ): Promise<ArchiveListing> {
    if (nestedEntryPath) {
      // Inside a nested archive — Go needs to extract it from the root first
      return this.call<ArchiveListing>(
        'ArchiveListNested',
        archiveFsPath,
        nestedEntryPath,
        innerPath,
      );
    }
    // Inside the root archive
    return this.call<ArchiveListing>('ArchiveList', archiveFsPath, innerPath);
  }
}

// ─── Utilities ────────────────────────────────────────────────────────────────

function basename(path: string): string {
  return path.replace(/\\/g, '/').split('/').pop() ?? path;
}

/** Join two innerPaths, avoiding double slashes. */
export function joinInnerPath(base: string, segment: string): string {
  if (!base) return segment;
  if (!segment) return base;
  return base.replace(/\/$/, '') + '/' + segment.replace(/^\//, '');
}

/**
 * Build a virtual path string for the address bar when inside an archive stack.
 * e.g. "C:/stuff/outer.zip :: libs/inner.tar.gz / src"
 */
export function buildArchiveDisplayPath(fsDir: string, stack: ArchiveFrame[]): string {
  if (!stack.length) return fsDir;
  let result = fsDir;
  for (const frame of stack) {
    result += ' :: ' + frame.archiveName;
    if (frame.innerPath) result += ' / ' + frame.innerPath;
  }
  return result;
}