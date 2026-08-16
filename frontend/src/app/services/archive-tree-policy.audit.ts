import type { FileInfo } from '../shared/protocol';

export function treeOperationalEntries(
  entries: readonly FileInfo[],
): FileInfo[] {
  return entries.filter(entry => entry.path !== '__parent__');
}

export function isTreeNavigationOnly(
  entry: FileInfo,
): boolean {
  return entry.path === '__parent__';
}
