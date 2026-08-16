import {
  appendNestedArchiveEntry,
  normalizeNestedArchiveChain,
} from './archive.service';

export function auditNestedArchiveNavigation(
  containingChain: readonly string[],
  currentInnerPath: string,
  entryPath: string,
) {
  const normalized = normalizeNestedArchiveChain(containingChain);
  const result = appendNestedArchiveEntry(
    normalized,
    currentInnerPath,
    entryPath,
  );

  return {
    containingChain: normalized,
    resultChain: result,
    appendedExactlyOne: result.length === normalized.length + 1,
  };
}
