import { Injectable } from '@angular/core';

// ============================================================================
// Archive Response Types
// ============================================================================

export type ArchiveEntryType = 'FILE' | 'DIRECTORY';

export interface ArchiveEntry {
  name: string;
  innerPath: string;
  type: ArchiveEntryType;
  size: number;
  compressedSize: number;
  modified: number;
  compression: string;
}

export interface ArchiveListing {
  archivePath: string;
  innerPath: string;
  format: string;
  entries: ArchiveEntry[];
  totalSize: number;
}

export interface ReadArchiveFileResponse {
  path: string;
  content: string;
  encoding: 'utf8' | 'base64';
  size: number;
}

export interface ExtractArchiveResponse {
  success: boolean;
  message: string;
  affectedPaths: string[];
}

// ============================================================================
// Archive Format Detection
// ============================================================================

export type ArchiveFormat =
  | 'zip' | 'tar' | 'tar.gz' | 'tar.bz2' | 'tar.xz' | 'tar.zst'
  | 'gz' | 'bz2' | 'xz' | 'zst'
  | '7z' | 'rar' | 'cab' | 'arj' | 'lzh' | 'ace';

const ARCHIVE_EXTENSIONS = [
  '.zip', '.jar', '.war', '.ear', '.apk',
  '.docx', '.xlsx', '.pptx', '.odt', '.ods', '.odp',
  '.tar', '.tar.gz', '.tgz', '.tar.bz2', '.tbz2', '.tbz',
  '.tar.xz', '.txz', '.tar.zst', '.tar.zstd', '.tzst',
  '.gz', '.bz2', '.xz', '.zst', '.zstd',
  '.7z', '.rar', '.cab', '.arj', '.lzh', '.lha', '.ace',
];

export function isArchive(path: string): boolean {
  const lower = path.toLowerCase();
  return ARCHIVE_EXTENSIONS.some(ext => lower.endsWith(ext));
}

export function getArchiveFormat(path: string): ArchiveFormat | 'unknown' {
  const lower = path.toLowerCase();
  if (lower.endsWith('.tar.gz') || lower.endsWith('.tgz')) return 'tar.gz';
  if (lower.endsWith('.tar.bz2') || lower.endsWith('.tbz2') || lower.endsWith('.tbz')) return 'tar.bz2';
  if (lower.endsWith('.tar.xz') || lower.endsWith('.txz')) return 'tar.xz';
  if (lower.endsWith('.tar.zst') || lower.endsWith('.tar.zstd') || lower.endsWith('.tzst')) return 'tar.zst';
  if (lower.endsWith('.tar')) return 'tar';
  if (['.zip','.jar','.war','.ear','.apk','.docx','.xlsx','.pptx','.odt','.ods','.odp']
        .some(e => lower.endsWith(e))) return 'zip';
  if (lower.endsWith('.gz')) return 'gz';
  if (lower.endsWith('.bz2')) return 'bz2';
  if (lower.endsWith('.xz')) return 'xz';
  if (lower.endsWith('.zst') || lower.endsWith('.zstd')) return 'zst';
  if (lower.endsWith('.7z')) return '7z';
  if (lower.endsWith('.rar')) return 'rar';
  if (lower.endsWith('.cab')) return 'cab';
  if (lower.endsWith('.arj')) return 'arj';
  if (lower.endsWith('.lzh') || lower.endsWith('.lha')) return 'lzh';
  if (lower.endsWith('.ace')) return 'ace';
  return 'unknown';
}

export function getFormatDescription(format: ArchiveFormat | string): string {
  const descriptions: Record<string, string> = {
    'zip': 'ZIP Archive', 'tar': 'TAR Archive', 'tar.gz': 'GZIP-compressed TAR',
    'tar.bz2': 'BZIP2-compressed TAR', 'tar.xz': 'XZ-compressed TAR',
    'tar.zst': 'Zstandard-compressed TAR', 'gz': 'GZIP Compressed File',
    'bz2': 'BZIP2 Compressed File', 'xz': 'XZ Compressed File',
    'zst': 'Zstandard Compressed File', '7z': '7-Zip Archive',
    'rar': 'RAR Archive', 'cab': 'CAB Archive', 'arj': 'ARJ Archive',
    'lzh': 'LZH Archive', 'ace': 'ACE Archive',
  };
  return descriptions[format] ?? `Unknown (${format})`;
}

export function getFormatExtension(format: ArchiveFormat): string {
  const extensions: Record<ArchiveFormat, string> = {
    'zip': '.zip', 'tar': '.tar', 'tar.gz': '.tar.gz', 'tar.bz2': '.tar.bz2',
    'tar.xz': '.tar.xz', 'tar.zst': '.tar.zst', 'gz': '.gz', 'bz2': '.bz2',
    'xz': '.xz', 'zst': '.zst', '7z': '.7z', 'rar': '.rar', 'cab': '.cab',
    'arj': '.arj', 'lzh': '.lzh', 'ace': '.ace',
  };
  return extensions[format] ?? '.unknown';
}

// ============================================================================
// Service – calls Wails backend directly (no HTTP command bus)
// ============================================================================

@Injectable({ providedIn: 'root' })
export class ArchiveService {

  /** Direct call to the Wails Go bridge. */
  private call<T>(method: string, ...args: unknown[]): Promise<T> {
    const bridge = (window as any)?.go?.main?.App;
    if (!bridge?.[method]) {
      return Promise.reject(new Error(`Wails bridge method '${method}' not found.`));
    }
    return bridge[method](...args) as Promise<T>;
  }

  /**
   * List entries inside an archive.
   * @param archivePath  Absolute path to the archive on disk.
   * @param innerPath    Sub-directory inside the archive ('' = root).
   */
  async listArchive(archivePath: string, innerPath = ''): Promise<ArchiveListing> {
    return this.call<ArchiveListing>('ListArchive', archivePath, innerPath);
  }

  /**
   * Read a single file from inside an archive.
   */
  async readArchiveFile(
    archivePath: string,
    innerPath: string,
    encoding: 'utf8' | 'base64' = 'utf8',
  ): Promise<ReadArchiveFileResponse> {
    return this.call<ReadArchiveFileResponse>('ReadArchiveFile', archivePath, innerPath, encoding);
  }

  /**
   * Extract entries from an archive to a destination directory.
   * @param innerPaths  Specific inner paths to extract; empty = all.
   */
  async extractArchive(
    archivePath: string,
    destination: string,
    innerPaths: string[] = [],
  ): Promise<ExtractArchiveResponse> {
    return this.call<ExtractArchiveResponse>('ExtractArchive', archivePath, destination, innerPaths);
  }

  // ---------------------------------------------------------------------------
  // Pure-TS helpers (unchanged from original)
  // ---------------------------------------------------------------------------

  compressionRatio(entry: ArchiveEntry): string {
    if (entry.size === 0 || entry.compressedSize === 0) return '—';
    return `${Math.round((1 - entry.compressedSize / entry.size) * 100)}%`;
  }

  isArchivePath(path: string): boolean { return isArchive(path); }
  getArchiveFormat(path: string): ArchiveFormat | 'unknown' { return getArchiveFormat(path); }
  getFormatDescription(format: ArchiveFormat | string): string { return getFormatDescription(format); }
  getFormatExtension(format: ArchiveFormat): string { return getFormatExtension(format); }

  isFormatSupported(format: string): boolean {
    return (['zip','tar','tar.gz','tar.bz2','tar.xz','tar.zst','gz','bz2','xz','zst',
             '7z','rar','cab','arj','lzh','ace'] as string[]).includes(format);
  }

  getSupportedFormats(): ArchiveFormat[] {
    return ['zip','tar','tar.gz','tar.bz2','tar.xz','tar.zst',
            'gz','bz2','xz','zst','7z','rar','cab','arj','lzh','ace'];
  }

  getArchiveIcon(format: ArchiveFormat | string): string {
    const icons: Record<string, string> = {
      'zip':'📦','tar':'📦','tar.gz':'📦','tar.bz2':'📦','tar.xz':'📦','tar.zst':'📦',
      'gz':'🗜️','bz2':'🗜️','xz':'🗜️','zst':'🗜️','7z':'🗜️',
      'rar':'📚','cab':'📦','arj':'📦','lzh':'📦','ace':'📦',
    };
    return icons[format] ?? '📄';
  }
}

export { getArchiveFormat as archiveFormat };