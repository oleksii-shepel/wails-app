import { FileType, type FileInfo } from '../shared/protocol';
import { getFileMimeColorKind, type FileMimeColorKind } from './file-entry-color.util';
import { DEFAULT_FILE_CATEGORIES } from '../shared/file-categories';

export interface FileEntryPresentation {
  iconEmoji: string;
  mimeColorKind: FileMimeColorKind;
  isSymlink: boolean;
  isParent: boolean;
}

const presentationCache = new Map<string, FileEntryPresentation>();

export function getFileEntryPresentation(entry: FileInfo | undefined, colorizeByMime: boolean): FileEntryPresentation {
  if (!entry) {
    return {
      iconEmoji: '📄',
      mimeColorKind: 'default',
      isSymlink: false,
      isParent: false,
    };
  }

  const isParent = entry.path === '__parent__';
  const isSymlink = entry.type === FileType.SYMLINK || !!entry.isSymlink;
  const cacheKey = [
    colorizeByMime ? '1' : '0',
    isParent ? '1' : '0',
    isSymlink ? '1' : '0',
    entry.type,
    entry.name ?? '',
    entry.mimeType ?? '',
    entry.registryKind ?? '',
    entry.path.startsWith('registry://') ? 'registry' : '',
    entry.path === 'registry://' ? 'root' : '',
  ].join('|');

  const cached = presentationCache.get(cacheKey);
  if (cached) return cached;

  const presentation: FileEntryPresentation = {
    iconEmoji: resolveIconEmoji(entry, isParent),
    mimeColorKind: getFileMimeColorKind(entry, colorizeByMime),
    isSymlink,
    isParent,
  };
  presentationCache.set(cacheKey, presentation);
  return presentation;
}

function resolveIconEmoji(entry: FileInfo, isParent: boolean): string {
  if (isParent) return '📄';
  if (entry.path.startsWith('registry://')) {
    const kind = (entry.registryKind || '').toLowerCase();
    if (entry.path === 'registry://' || kind === 'registry') return '🧭';
    if (kind === 'key' || entry.type === FileType.DIRECTORY) return '🗂️';
    return '📝';
  }

  switch (entry.type) {
    case FileType.DIRECTORY:
      return '📁';
    case FileType.SYMLINK:
      return '📄';
    default:
      return extIcon(entry.name);
  }
}

function extIcon(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase();
  if (!ext) return '📄';
  const category = DEFAULT_FILE_CATEGORIES.find((candidate) => candidate.extensions.includes(ext));
  return category?.icon || '📄';
}
