import { PathSegment } from './address-bar.types';
import { ArchiveFrame, buildArchiveDisplayPath, isArchive } from '../services/archive.service';
import { REGISTRY_ROOT } from '../services/registry.service';

// ─── Path utilities ───────────────────────────────────────────────────────────

export interface SearchAliasQuery {
  path: string;
  pattern: string;
  recursive: boolean;
}

function canonicalDriveReference(p: string): string | null {
  const normalized = (p || '').trim().replace(/\\/g, '/');
  const driveRef = /^drive:\/\/([a-z])(\/.*)?$/i.exec(normalized);
  if (driveRef) {
    const drive = driveRef[1].toLowerCase();
    const rest = (driveRef[2] || '').replace(/^\/+/, '').replace(/\/+/, '/');
    return rest ? `drive://${drive}/${rest}` : `drive://${drive}`;
  }
  const windowsRef = /^([a-z]):(\/.*)?$/i.exec(normalized);
  if (windowsRef) {
    const drive = windowsRef[1].toLowerCase();
    const rest = (windowsRef[2] || '').replace(/^\/+/, '').replace(/\/+/g, '/');
    return rest ? `drive://${drive}/${rest}` : `drive://${drive}`;
  }
  return null;
}

function isDriveReference(p: string): boolean {
  return canonicalDriveReference(p) !== null;
}

function toShortDriveDisplayPath(p: string): string {
  const canonical = canonicalDriveReference(p);
  if (!canonical) return p;
  const rest = canonical.slice('drive://'.length);
  const drive = rest.slice(0, 1).toUpperCase();
  const suffix = rest.slice(1).replace(/^\/+/, '');
  return suffix ? `${drive}:/${suffix}` : `${drive}:/`;
}

function aliasSchemeFor(p: string): string | null {
  const normalized = (p || '').trim().replace(/\\/g, '/').toLowerCase();
  const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec(normalized);
  if (!match) return null;
  const scheme = `${match[1].toLowerCase()}://`;
  if (scheme === 'drive://') return null;
  return scheme;
}

function normalizeAliasReference(p: string, scheme: string): string {
  const normalized = (p || '').trim().replace(/\\/g, '/');
  if (scheme === 'search://' && normalized.slice(scheme.length).startsWith('?')) {
    return normalizeSearchAliasReference(normalized);
  }
  const rest = normalized.slice(scheme.length).replace(/^\/+/, '').replace(/\/+/g, '/');
  return rest ? `${scheme}${rest}` : scheme;
}

function normalizeSearchAliasReference(p: string): string {
  const normalized = (p || '').trim().replace(/\\/g, '/');
  const raw = normalized.slice('search://'.length);
  if (!raw || raw === '?') return 'search://';
  if (!raw.startsWith('?')) return normalized;

  const params = new URLSearchParams(raw.slice(1));
  const path = normalizePath(params.get('path') || '/');
  const pattern = (params.get('pattern') || '').trim();
  const recursiveRaw = (params.get('recursive') || '').trim().toLowerCase();
  const recursive = recursiveRaw === '' || recursiveRaw === '1' || recursiveRaw === 'true';

  if (!pattern) {
    return 'search://';
  }

  const next = new URLSearchParams();
  void next;
  return buildReadableSearchAlias(path, pattern, recursive);
}

function buildReadableSearchAlias(path: string, pattern: string, recursive: boolean): string {
  const parts = [
    `path=${readableSearchValue(path)}`,
    `pattern=${readableSearchValue(pattern)}`,
    `recursive=${recursive ? 'true' : 'false'}`,
  ];
  return `search://?${parts.join('&')}`;
}

function readableSearchValue(value: string): string {
  return encodeURIComponent(value)
    .replace(/%2F/gi, '/')
    .replace(/%3A/gi, ':')
    .replace(/%2A/gi, '*');
}

export function parseSearchAliasQuery(p: string): SearchAliasQuery | null {
  const normalized = normalizePath(p);
  if (!normalized.toLowerCase().startsWith('search://?')) {
    return null;
  }

  const params = new URLSearchParams(normalized.slice('search://?'.length));
  const path = params.get('path');
  const pattern = params.get('pattern');
  if (!path || !pattern) {
    return null;
  }

  const recursiveRaw = (params.get('recursive') || '').trim().toLowerCase();
  return {
    path: normalizePath(path),
    pattern,
    recursive: recursiveRaw === '' || recursiveRaw === '1' || recursiveRaw === 'true',
  };
}

export function isAliasPath(p: string): boolean {
  return aliasSchemeFor(p) !== null;
}

function isRegistryReference(p: string): boolean {
  return /^registry:\/\//i.test((p || '').trim().replace(/\\/g, '/'));
}

function normalizeRegistryReference(p: string): string {
  const normalized = (p || '').trim().replace(/\\/g, '/');
  const match = /^registry:\/\/(.*)?$/i.exec(normalized);
  if (!match) return normalized;
  const rest = (match[1] || '').replace(/^\/+/, '').replace(/\/+/g, '/');
  return rest ? `${REGISTRY_ROOT}${rest}` : REGISTRY_ROOT;
}

export function canonicalizePathReference(p: string): string {
  if (!p) return p;

  const driveReference = canonicalDriveReference(p);
  if (driveReference) {
    return driveReference;
  }

  const n = p.replace(/\\/g, '/');

  const registryMatch = /^registry:\/\/(.*)?$/i.exec(n);
  if (registryMatch) {
    const rest = (registryMatch[1] || '').replace(/^\/+/, '').replace(/\/+/g, '/');
    return rest ? `${REGISTRY_ROOT}${rest}` : REGISTRY_ROOT;
  }

  const aliasScheme = aliasSchemeFor(n);
  if (aliasScheme) {
    return normalizeAliasReference(n, aliasScheme);
  }

  return n;
}

export function isWindowsPath(p: string): boolean {
  return isDriveReference(canonicalizePathReference(p));
}

export function normalizePath(p: string): string {
  if (!p) return '/';
  let n = canonicalizePathReference(p);
  if (isRegistryReference(n)) {
    return normalizeRegistryReference(n);
  }
  const aliasScheme = aliasSchemeFor(n);
  if (aliasScheme) {
    return normalizeAliasReference(n, aliasScheme);
  }
  if (isDriveReference(n)) {
    return canonicalDriveReference(n) || n;
  } else {
    n = n.replace(/\/+/g, '/');
  }
  if (n.length > 1 && n.endsWith('/')) {
    n = n.slice(0, -1);
  }
  return n || '/';
}

export function pathIdentityKey(p: string): string {
  const normalized = normalizePath(p);
  const searchQuery = parseSearchAliasQuery(normalized);
  if (searchQuery) {
    return buildReadableSearchAlias(pathIdentityKey(searchQuery.path), searchQuery.pattern, searchQuery.recursive);
  }
  if (isRegistryReference(normalized)) {
    return normalized.toLowerCase();
  }
  if (isDriveReference(normalized) || normalized.startsWith('//')) {
    return normalized.toLowerCase();
  }
  return normalized;
}

export function samePath(left: string | null | undefined, right: string | null | undefined): boolean {
  if (!left && !right) {
    return true;
  }
  if (!left || !right) {
    return false;
  }
  return pathIdentityKey(left) === pathIdentityKey(right);
}

export function parentPath(p: string): string {
  const n = normalizePath(p);
  const searchQuery = parseSearchAliasQuery(n);
  if (searchQuery) {
    return searchQuery.path;
  }
  if (isRegistryReference(n)) {
    if (n === REGISTRY_ROOT) return n;
    const rel = n.slice(REGISTRY_ROOT.length);
    const idx = rel.lastIndexOf('/');
    return idx < 0 ? REGISTRY_ROOT : `${REGISTRY_ROOT}${rel.slice(0, idx)}`;
  }
  const aliasScheme = aliasSchemeFor(n);
  if (aliasScheme) {
    const rel = n.slice(aliasScheme.length);
    if (!rel) return aliasScheme;
    const idx = rel.lastIndexOf('/');
    return idx < 0 ? aliasScheme : `${aliasScheme}${rel.slice(0, idx)}`;
  }
  const driveReference = canonicalDriveReference(n);
  if (driveReference) {
    const rest = driveReference.slice('drive://'.length);
    const drive = rest.slice(0, 1).toLowerCase();
    const rel = rest.slice(1).replace(/^\/+/, '');
    if (!rel) return `drive://${drive}`;
    const idx = rel.lastIndexOf('/');
    return idx < 0 ? `drive://${drive}` : `drive://${drive}/${rel.slice(0, idx)}`;
  }
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
  p = normalizePath(p);
  const searchQuery = parseSearchAliasQuery(p);
  if (searchQuery) {
    return `search://${searchQuery.pattern} @ ${shortenPath(searchQuery.path)}`;
  }
  if (isRegistryReference(p)) {
    const rel = p.slice(REGISTRY_ROOT.length);
    if (!rel) return 'registry:';
    if (rel.length <= 28) return rel;
    const parts = rel.split('/').filter(Boolean);
    if (parts.length <= 2) return rel;
    return '…/' + parts.slice(-2).join('/');
  }
  const aliasScheme = aliasSchemeFor(p);
  if (aliasScheme) {
    const rel = p.slice(aliasScheme.length);
    if (!rel) return aliasScheme.slice(0, -2);
    if (rel.length <= 28) return `${aliasScheme}${rel}`;
    const parts = rel.split('/').filter(Boolean);
    if (parts.length <= 2) return `${aliasScheme}${rel}`;
    return `${aliasScheme}…/${parts.slice(-2).join('/')}`;
  }
  const driveReference = canonicalDriveReference(p);
  if (driveReference) {
    return toShortDriveDisplayPath(driveReference);
  }
  if (p.length <= 28) return p;
  const parts = p.split('/').filter(Boolean);
  if (parts.length <= 2) return p;
  return '…/' + parts.slice(-2).join('/');
}

export function formatPathForDisplay(path: string): string {
  const normalized = normalizePath(path);
  const driveReference = canonicalDriveReference(normalized);
  if (driveReference) {
    return toShortDriveDisplayPath(driveReference);
  }
  return normalized;
}

export function formatPathsInText(text: string): string {
  if (!text) return text;

  return text.replace(/drive:\/\/[a-z](?:\/[^:\n\r]*)?/gi, (match) => formatPathForDisplay(match));
}

// ─── Segment builders ─────────────────────────────────────────────────────────

export function buildSegments(p: string): PathSegment[] {
  p = normalizePath(p);
  if (!p) return [];

  const searchQuery = parseSearchAliasQuery(p);
  if (searchQuery) {
    return [
      {
        label: 'search:',
        fullPath: 'search://',
        isRoot: true,
        rootLabel: 'search:',
      },
      {
        label: searchQuery.pattern,
        fullPath: p,
        isRoot: false,
      },
    ];
  }

  if (isRegistryReference(p)) {
    const rel = p.slice(REGISTRY_ROOT.length);
    const parts = rel ? rel.split('/').filter(Boolean) : [];
    const segs: PathSegment[] = [{
      label: 'registry:',
      fullPath: REGISTRY_ROOT,
      isRoot: true,
      rootLabel: 'registry:',
    }];

    parts.forEach((part, i) => {
      segs.push({
        label: part,
        fullPath: `${REGISTRY_ROOT}${parts.slice(0, i + 1).join('/')}`,
        isRoot: false,
      });
    });
    return segs;
  }

  const aliasScheme = aliasSchemeFor(p);
  if (aliasScheme) {
    const rel = p.slice(aliasScheme.length);
    const parts = rel ? rel.split('/').filter(Boolean) : [];
    const rootLabel = aliasScheme.slice(0, -2);
    const segs: PathSegment[] = [{
      label: rootLabel,
      fullPath: aliasScheme,
      isRoot: true,
      rootLabel,
    }];

    parts.forEach((part, i) => {
      segs.push({
        label: part,
        fullPath: `${aliasScheme}${parts.slice(0, i + 1).join('/')}`,
        isRoot: false,
      });
    });
    return segs;
  }

  const driveReference = canonicalDriveReference(p);
  if (driveReference) {
    const rest = driveReference.slice('drive://'.length);
    const drive = rest.slice(0, 1).toUpperCase() + ':';
    const rel = rest.slice(1).replace(/^\/+/, '');
    const parts = rel ? rel.split('/').filter(Boolean) : [];

    const segs: PathSegment[] = [{
      label: drive,
      fullPath: `drive://${drive[0].toLowerCase()}`,
      isRoot: true,
      rootLabel: drive,
    }];

    parts.forEach((part, i) => {
      segs.push({
        label: part,
        fullPath: `drive://${drive[0].toLowerCase()}/${parts.slice(0, i + 1).join('/')}`,
        isRoot: false,
      });
    });
    return segs;
  }

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
  path = normalizePath(path);
  const ARCHIVE_RE = /\.(zip|tar|tgz|tar\.gz|tar\.bz2|tar\.xz|gz|bz2|xz|rar|7z)$/i;
  const driveReference = canonicalDriveReference(path);
  const withoutDrive = driveReference ? driveReference.slice('drive://'.length) : '';
  const driveLetter = driveReference ? withoutDrive.slice(0, 1).toLowerCase() : '';
  const driveRemainder = driveReference ? withoutDrive.slice(1).replace(/^\/+/, '') : '';
  const parts = driveReference
    ? (driveRemainder ? driveRemainder.split('/').filter(Boolean) : [])
    : path.replace(/\\/g, '/').split('/').filter(Boolean);

  const isWin = !!driveReference || path.startsWith('//') || path.startsWith('\\');

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
  if (driveReference) {
    fsDirPath = `drive://${driveLetter}` + (fsParts.length ? `/${fsParts.join('/')}` : '');
  } else if (isWin && /^[A-Za-z]:/.test(parts[0])) {
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
