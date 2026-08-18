import type { FileInfo } from '../shared/protocol';
import { sortEntries } from './entry-model';

export function parentEntryStaysFirst(
  entries: readonly FileInfo[],
): boolean {
  const parent = entries.find(entry => entry.path === '__parent__');
  if (!parent) return true;

  const fields = ['name', 'size', 'modified', 'type'] as const;
  const directions = ['asc', 'desc'] as const;

  return fields.every(sortField =>
    directions.every(sortDirection =>
      [true, false].every(directoriesFirst =>
        sortEntries(entries, {
          sortField,
          sortDirection,
          directoriesFirst,
        })[0]?.path === '__parent__',
      ),
    ),
  );
}
