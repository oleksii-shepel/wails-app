import { FileType, type FileInfo } from '../shared/protocol';
import { getPrimaryFileCategoryForExtension } from '../shared/file-categories';

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

const EXECUTABLE_EXT_SET = new Set([
  'apk', 'appimage', 'bat', 'cmd', 'com', 'dll', 'exe', 'jar', 'msi', 'scr',
]);

function getNormalizedExtension(name: string | null | undefined): string {
  const value = (name ?? '').trim().toLowerCase();
  const lastDot = value.lastIndexOf('.');
  return lastDot >= 0 ? value.slice(lastDot + 1) : '';
}

function getCategoryFromExtension(name: string | null | undefined): FileMimeColorKind | null {
  const ext = getNormalizedExtension(name);
  if (!ext) return null;
  if (EXECUTABLE_EXT_SET.has(ext)) return 'executable';
  return (getPrimaryFileCategoryForExtension(ext)?.id as FileMimeColorKind | undefined) ?? null;
}

export function getResolvedCategory(entry: FileInfo, resolvedCategoryId?: string | null): FileMimeColorKind | null {
  if (resolvedCategoryId) {
    return resolvedCategoryId as FileMimeColorKind;
  }

  const extKind = getCategoryFromExtension(entry.name);
  return extKind;
}

export function getFileMimeColorKind(entry: FileInfo, colorizeByMime: boolean, resolvedCategoryId?: string | null): FileMimeColorKind {
  if (!entry) return 'default';
  if (!colorizeByMime) return 'default';
  if (entry.type === FileType.SYMLINK || entry.isSymlink) return 'symlink';
  if (entry.type === FileType.DIRECTORY) return 'directory';
  if (entry.type === FileType.ARCHIVE) return 'archive';

  const kind = getResolvedCategory(entry, resolvedCategoryId);
  if (kind) return kind;

  return 'default';
}