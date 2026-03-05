import { PathSegment } from './address-bar.types';
import { ArchiveFrame, buildArchiveDisplayPath, isArchive } from '../services/archive.service';

// ─── Path utilities ───────────────────────────────────────────────────────────

export function isWindowsPath(p: string): boolean {
  return /^[A-Za-z]:/.test(p);
}

export function normalizePath(p: string): string {
  if (!p) return '/';
  let n = p.replace(/\\/g, '/');
  if (isWindowsPath(n)) {
    const drive = n.slice(0, 2);
    const rest = n.slice(2).replace(/\/+/g, '/');
    n = drive + (rest.startsWith('/') ? rest : '/' + rest);
  } else {
    n = n.replace(/\/+/g, '/');
  }
  if (n.length > 1 && n.endsWith('/') && !(/^[A-Za-z]:\//.test(n) && n.length === 3)) {
    n = n.slice(0, -1);
  }
  return n || '/';
}

export function parentPath(p: string): string {
  const n = normalizePath(p);
  if (/^[A-Za-z]:\/$/.test(n) || /^[A-Za-z]:$/.test(n)) return n;
  if (n === '/') return '/';
  const parts = n.split('/').filter(Boolean);
  if (parts.length <= 1) {
    return isWindowsPath(n) ? parts[0] + '/' : '/';
  }
  parts.pop();
  if (isWindowsPath(n)) {
    const drive = parts[0];
    const rest = parts.slice(1);
    return rest.length ? drive + '/' + rest.join('/') : drive + '/';
  }
  return '/' + parts.join('/');
}

export function shortenPath(p: string): string {
  if (p.length <= 28) return p;
  const parts = p.split('/').filter(Boolean);
  if (parts.length <= 2) return p;
  return '…/' + parts.slice(-2).join('/');
}

// ─── Segment builders ─────────────────────────────────────────────────────────

export function buildSegments(p: string): PathSegment[] {
  if (!p) return [];

  if (isWindowsPath(p)) {
    const drive = p.slice(0, 2).toUpperCase();
    const rest = p.slice(3);
    const parts = rest ? rest.split('/').filter(Boolean) : [];

    const segs: PathSegment[] = [{
      label: drive,
      fullPath: drive + '/',
      isRoot: true,
      rootLabel: drive,
    }];

    parts.forEach((part, i) => {
      segs.push({
        label: part,
        fullPath: drive + '/' + parts.slice(0, i + 1).join('/'),
        isRoot: false,
      });
    });
    return segs;
  }

  const parts = p.split('/').filter(Boolean);
  const segs: PathSegment[] = [{
    label: '/',
    fullPath: '/',
    isRoot: true,
    rootLabel: '/',
  }];
  parts.forEach((part, i) => {
    segs.push({
      label: part,
      fullPath: '/' + parts.slice(0, i + 1).join('/'),
      isRoot: false,
    });
  });
  return segs;
}

export function buildArchiveSegments(fsDirPath: string, stack: ArchiveFrame[]): PathSegment[] {
  if (stack.length === 0) return buildSegments(fsDirPath);

  const virtualPath = buildArchiveDisplayPath(fsDirPath, stack);
  const allSegs = buildSegments(virtualPath);
  const fsDepth = buildSegments(fsDirPath).length;

  allSegs.forEach((seg, i) => {
    if (i === fsDepth) {
      // This is the archive file segment itself
      seg.isArchiveRoot = true;
    } else if (i > fsDepth) {
    const label = seg.label.toLowerCase();
    if (label.endsWith('.zip') || label.endsWith('.tar') || label.endsWith('.tgz') ||
        label.endsWith('.tar.gz') || label.endsWith('.tar.bz2') || label.endsWith('.tar.xz') ||
        label.endsWith('.rar') || label.endsWith('.7z')) {
      seg.isArchiveRoot = true;
      seg.isArchiveInner = true;
    } else {
        seg.isArchiveInner = true;
      }
    }
    // i < fsDepth → plain FS segment, no archive flags
  });

  return allSegs;
}

// ─── Highlight match for autocomplete ────────────────────────────────────────

export function highlightMatch(text: string, query: string): string {
  const t = text ?? '';
  const q = query ?? '';
  if (!q) return escapeHtml(t);

  const idx = t.toLowerCase().indexOf(q.toLowerCase());
  if (idx < 0) return escapeHtml(t);

  const before = escapeHtml(t.slice(0, idx));
  const match = escapeHtml(t.slice(idx, idx + q.length));
  const after = escapeHtml(t.slice(idx + q.length));
  return before + '<span class="ab-ac-match">' + match + '</span>' + after;
}

function escapeHtml(s: string): string {
  return (s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

/**
 * Splits a path into (fsDirPath, archiveSegments) by detecting the first
 * archive extension boundary, enabling the address bar to self-parse any
 * virtual path without needing an externally-provided archiveStack.
 *
 * Examples:
 *   /home/user/archive.zip/folder/nested.tar/deep
 *   → { fsDirPath: '/home/user/archive.zip', archiveInnerParts: ['folder','nested.tar','deep'] }
 *
 *   /home/user/docs
 *   → { fsDirPath: '/home/user/docs', archiveInnerParts: [] }
 */
export function parsePathWithArchives(path: string): {
  fsDirPath: string;
  archiveInnerParts: string[];
} {
  const ARCHIVE_RE = /\.(zip|tar|tgz|tar\.gz|tar\.bz2|tar\.xz|gz|bz2|xz|rar|7z)$/i;
  const sep = path.includes('\\') ? '\\' : '/';
  const parts = path.replace(/\\/g, '/').split('/').filter(Boolean);

  // Windows UNC or drive-letter prefix reconstruction
  const isWin = /^[A-Za-z]:/.test(path) || path.startsWith('//') || path.startsWith('\\\\');

  let archiveBoundary = -1;
  for (let i = 0; i < parts.length; i++) {
    if (ARCHIVE_RE.test(parts[i])) { archiveBoundary = i; break; }
  }

  if (archiveBoundary === -1) {
    return { fsDirPath: path, archiveInnerParts: [] };
  }

  const fsParts = parts.slice(0, archiveBoundary + 1);
  const innerParts = parts.slice(archiveBoundary + 1);

  // Reconstruct the FS prefix correctly for Windows vs POSIX
  let fsDirPath: string;
  if (isWin && /^[A-Za-z]:/.test(parts[0])) {
    fsDirPath = fsParts.join('/');
  } else if (path.startsWith('//') || path.startsWith('\\\\')) {
    fsDirPath = '//' + fsParts.join('/');
  } else {
    fsDirPath = '/' + fsParts.join('/');
  }

  return { fsDirPath, archiveInnerParts: innerParts };
}

/**
 * Builds PathSegment[] for any path, archive-aware, without needing
 * an ArchiveFrame stack. Used as fallback in address-bar when no
 * external archiveStack is provided but the path itself contains archives.
 */
export function buildSegmentsFromParsedPath(
  fsDirPath: string,
  archiveInnerParts: string[]
): PathSegment[] {
  const segments: PathSegment[] = [];
  
  // Build the filesystem path segments first
  const fsSegments = buildSegments(fsDirPath);
  segments.push(...fsSegments);
  
  // Track the cumulative path
  let cumulativePath = fsDirPath;
  
  // Add archive inner parts, marking EVERY archive file in the path
  for (let i = 0; i < archiveInnerParts.length; i++) {
    const part = archiveInnerParts[i];
    cumulativePath += '/' + part;
    
    // Check if this part is an archive file
    const isArchiveFile = isArchive(part);
    const isLast = i === archiveInnerParts.length - 1;
    
    segments.push({
      label: part,
      fullPath: cumulativePath,
      isRoot: false,
      isLast,
      // Mark as archive root if it's an archive file
      isArchiveRoot: isArchiveFile,
      // Mark as archive inner only if it's NOT an archive file AND we're inside an archive
      isArchiveInner: !isArchiveFile
    });
  }
  
  return segments;
}
