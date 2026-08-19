import { FileType } from '../shared/protocol';

export function renameSelectionEnd(
  name: string,
  type: FileType,
): number {
  if (type === FileType.DIRECTORY) return name.length;
  const lastDot = name.lastIndexOf('.');
  return lastDot > 0 ? lastDot : name.length;
}

export function isInlineRenameNoop(
  original: string,
  requested: string,
): boolean {
  return requested.trim() === original;
}