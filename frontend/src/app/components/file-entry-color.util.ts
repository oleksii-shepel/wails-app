import { FileType, type FileInfo } from '../shared/protocol';
import { DEFAULT_FILE_CATEGORIES } from '../shared/file-categories';

export type FileMimeColorKind =
  | 'default'
  | 'directory'
  | 'archive'
  | 'symlink'
  | 'executable'
  | 'images'
  | 'documents'
  | 'spreadsheets'
  | 'presentations'
  | 'code'
  | 'web'
  | 'data'
  | 'archives'
  | 'audio'
  | 'video'
  | 'fonts'
  | 'markdown'
  | 'scripts'
  | '3d'
  | 'design';

const categoryByExtension = new Map<string, FileMimeColorKind>();
for (const category of DEFAULT_FILE_CATEGORIES) {
  for (const ext of category.extensions) {
    categoryByExtension.set(ext.toLowerCase(), category.id as FileMimeColorKind);
  }
}

const EXECUTABLE_MIME_SET = new Set([
  'application/java-archive',
  'application/vnd.microsoft.portable-executable',
  'application/x-appimage',
  'application/x-deb',
  'application/x-dosexec',
  'application/x-executable',
  'application/x-mach-binary',
  'application/x-msdownload',
  'application/x-msi',
  'application/x-rpm',
]);

const EXECUTABLE_EXT_SET = new Set([
  'apk', 'appimage', 'bat', 'cmd', 'com', 'dll', 'exe', 'jar', 'msi', 'scr',
]);

function normalizeMime(mime: string | null | undefined): string {
  return (mime ?? '').split(';', 1)[0].trim().toLowerCase();
}

function getNormalizedExtension(name: string | null | undefined): string {
  const value = (name ?? '').trim().toLowerCase();
  const lastDot = value.lastIndexOf('.');
  return lastDot >= 0 ? value.slice(lastDot + 1) : '';
}

function getCategoryFromExtension(name: string | null | undefined): FileMimeColorKind | null {
  const ext = getNormalizedExtension(name);
  if (!ext) return null;
  if (EXECUTABLE_EXT_SET.has(ext)) return 'executable';
  return categoryByExtension.get(ext) ?? null;
}

function getCategoryFromMime(mime: string): FileMimeColorKind | null {
  if (!mime) return null;
  if (EXECUTABLE_MIME_SET.has(mime)) return 'executable';

  if (mime === 'text/markdown') return 'markdown';
  if (mime.startsWith('image/')) return 'images';
  if (mime.startsWith('video/')) return 'video';
  if (mime.startsWith('audio/')) return 'audio';
  if (mime.startsWith('font/')) return 'fonts';

  if (
    mime === 'text/html' ||
    mime === 'text/css' ||
    mime.includes('xml') ||
    mime.includes('svg')
  ) {
    return 'web';
  }

  if (
    mime.includes('json') ||
    mime.includes('yaml') ||
    mime.includes('toml') ||
    mime.includes('sql') ||
    mime === 'text/csv' ||
    mime === 'text/tab-separated-values'
  ) {
    return 'data';
  }

  if (
    mime.includes('javascript') ||
    mime.includes('typescript') ||
    mime.includes('shellscript') ||
    mime.includes('x-sh') ||
    mime.includes('python') ||
    mime.includes('ruby') ||
    mime.includes('rust') ||
    mime.includes('java') ||
    mime.includes('kotlin') ||
    mime.includes('swift') ||
    mime.includes('php')
  ) {
    return 'code';
  }

  if (
    mime === 'application/pdf' ||
    mime === 'application/msword' ||
    mime === 'application/rtf' ||
    mime === 'application/postscript' ||
    mime.includes('wordprocessingml') ||
    mime.includes('opendocument.text') ||
    mime === 'text/plain' ||
    mime === 'text/rtf' ||
    mime === 'application/epub+zip'
  ) {
    return 'documents';
  }

  if (
    mime.includes('excel') ||
    mime.includes('spreadsheetml') ||
    mime.includes('opendocument.spreadsheet')
  ) {
    return 'spreadsheets';
  }

  if (
    mime.includes('powerpoint') ||
    mime.includes('presentationml') ||
    mime.includes('opendocument.presentation') ||
    mime === 'application/vnd.apple.keynote'
  ) {
    return 'presentations';
  }

  if (
    mime.includes('zip') ||
    mime.includes('rar') ||
    mime.includes('7z') ||
    mime.includes('tar') ||
    mime.includes('gzip') ||
    mime.includes('bzip') ||
    mime.includes('xz') ||
    mime.includes('dmg') ||
    mime.includes('iso')
  ) {
    return 'archives';
  }

  return null;
}

export function getFileMimeColorKind(entry: FileInfo, colorizeByMime: boolean): FileMimeColorKind {
  if (!entry) return 'default';
  if (!colorizeByMime) return 'default';
  if (entry.type === FileType.SYMLINK || entry.isSymlink) return 'symlink';
  if (entry.type === FileType.DIRECTORY) return 'directory';
  if (entry.type === FileType.ARCHIVE) return 'archive';

  const extKind = getCategoryFromExtension(entry.name);
  if (extKind) return extKind;

  const mimeKind = getCategoryFromMime(normalizeMime(entry.mimeType));
  if (mimeKind) return mimeKind;

  return 'default';
}
