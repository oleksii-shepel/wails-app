import type { ArchiveTabState } from './workspace.service';
import {
  parseArchivePath,
  sameArchiveLocation,
} from './archive.service';

export interface ArchiveTabStateAuditResult {
  readonly hasFrames: boolean;
  readonly rootMatchesLocation: boolean;
  readonly nestedChainMatchesLocation: boolean;
}

export function auditArchiveTabState(
  location: string,
  state: ArchiveTabState | null,
): ArchiveTabStateAuditResult {
  if (!state || state.archiveStack.length === 0) {
    return {
      hasFrames: false,
      rootMatchesLocation: false,
      nestedChainMatchesLocation: false,
    };
  }

  const parsed = parseArchivePath(location);
  const root = state.archiveStack[0];
  const current = state.archiveStack[state.archiveStack.length - 1];

  return {
    hasFrames: true,
    rootMatchesLocation:
      !!root && sameArchiveLocation(root.fsPath, parsed.fsPath),
    nestedChainMatchesLocation:
      !!current &&
      current.nestedChain.length === parsed.nestedChain.length &&
      current.nestedChain.every(
        (segment, index) => segment === parsed.nestedChain[index],
      ),
  };
}