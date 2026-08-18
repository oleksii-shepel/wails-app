import { FileInfo, FileType } from '../shared/protocol';

export type EntrySortField = 'name' | 'size' | 'modified' | 'type';
export type EntrySortDirection = 'asc' | 'desc';

export interface EntryProjectionOptions {
  sortField: EntrySortField;
  sortDirection: EntrySortDirection;
  directoriesFirst: boolean;
  filterText: string;
}

export function projectEntries(
  entries: readonly FileInfo[],
  options: EntryProjectionOptions,
): FileInfo[] {
  const query = normalizeSearchText(options.filterText);
  // Filtering first cuts the expensive locale-aware sort down to only the
  // entries that can be rendered. This is the fallback path for providers
  // which cannot apply the projection in their own backend.
  const filtered = !query ? entries : entries.filter(entry =>
    normalizeSearchText(entry.name).includes(query) ||
    normalizeSearchText(entry.path).includes(query),
  );

  return [...filtered].sort((left, right) =>
    compareEntries(left, right, options),
  );
}

export function sortEntries(
  entries: readonly FileInfo[],
  options: Omit<EntryProjectionOptions, 'filterText'>,
): FileInfo[] {
  return [...entries].sort((left, right) =>
    compareEntries(left, right, { ...options, filterText: '' }),
  );
}

export function createFilenameMaskMatcher(
  mask: string,
): (name: string) => boolean {
  const escaped = (mask ?? '')
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*/g, '.*')
    .replace(/\?/g, '.');

  const expression = new RegExp(`^${escaped}$`, 'i');
  return name => expression.test(name ?? '');
}

export function fileExtension(name: string): string {
  const value = name ?? '';
  const index = value.lastIndexOf('.');
  if (index <= 0 || index === value.length - 1) return '';
  return value.slice(index + 1).toLocaleLowerCase();
}

export function entryTypeLabel(entry: FileInfo): string {
  if (entry.type === FileType.DIRECTORY) return 'folder';
  if (entry.type === FileType.ARCHIVE) return 'archive';
  if (entry.type === FileType.SYMLINK) return 'symbolic link';
  return 'file';
}

function compareEntries(
  left: FileInfo,
  right: FileInfo,
  options: EntryProjectionOptions,
): number {
  // Synthetic parent navigation is structural UI, not sortable content.
  // It must remain the first row regardless of field/direction/folder policy.
  const leftParent = left.path === '__parent__';
  const rightParent = right.path === '__parent__';

  if (leftParent !== rightParent) {
    return leftParent ? -1 : 1;
  }

  if (options.directoriesFirst) {
    const leftDirectory = left.type === FileType.DIRECTORY;
    const rightDirectory = right.type === FileType.DIRECTORY;
    if (leftDirectory !== rightDirectory) {
      return leftDirectory ? -1 : 1;
    }
  }

  const direction = options.sortDirection === 'desc' ? -1 : 1;
  let result = 0;

  if (options.sortField === 'size') {
    result = numeric(left.size) - numeric(right.size);
  } else if (options.sortField === 'modified') {
    result = numeric(left.modified) - numeric(right.modified);
  } else if (options.sortField === 'type') {
    result = String(left.type ?? '').localeCompare(String(right.type ?? ''));
  } else {
    result = compareNames(left.name, right.name);
  }

  if (result === 0 && options.sortField !== 'name') {
    result = compareNames(left.name, right.name);
  }

  return result * direction;
}

function numeric(value: unknown): number {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

function compareNames(left: unknown, right: unknown): number {
  return String(left ?? '').localeCompare(
    String(right ?? ''),
    undefined,
    { numeric: true, sensitivity: 'base' },
  );
}

function normalizeSearchText(value: unknown): string {
  return String(value ?? '').trim().toLocaleLowerCase();
}
