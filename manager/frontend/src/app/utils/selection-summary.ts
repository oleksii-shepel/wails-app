import { FileType, type FileInfo } from '../shared/protocol';

/**
 * Formats the byte size of the selected entries for the pane status row.
 * Directories have no cheap size on hand, so the summary is empty whenever
 * the selection contains one; the on-demand "calculate size" action still
 * covers that case.
 */
export function selectionSizeSuffix(
  selectedPaths: ReadonlySet<string>,
  entries: readonly FileInfo[],
): string {
  if (selectedPaths.size === 0) {
    return '';
  }
  const selected = new Set(selectedPaths);
  let total = 0;
  for (const entry of entries) {
    if (!selected.has(entry.path)) {
      continue;
    }
    if (entry.type === FileType.DIRECTORY || entry.type === FileType.SYMLINK) {
      return '';
    }
    total += entry.size;
  }
  return ' · ' + formatBytes(total);
}

export function formatBytes(bytes: number): string {
  if (bytes <= 0) {
    return '0 B';
  }
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  const value = bytes / Math.pow(1024, index);
  return (index === 0 || value >= 10 ? value.toFixed(0) : value.toFixed(1)) + ' ' + units[index];
}
