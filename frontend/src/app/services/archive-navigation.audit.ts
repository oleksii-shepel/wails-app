import {
  buildArchivePath,
  normalizeArchiveLocation,
  parseArchivePath,
} from './archive.service';

export interface ArchiveNavigationAuditResult {
  readonly canonical: string;
  readonly roundTrips: boolean;
  readonly schemePreserved: boolean;
}

export function auditArchiveNavigation(
  location: string,
): ArchiveNavigationAuditResult {
  const canonical = normalizeArchiveLocation(location);
  const parsed = parseArchivePath(canonical);
  const rebuilt = buildArchivePath(
    parsed.fsPath,
    parsed.nestedChain,
    parsed.innerPath,
  );

  const scheme = /^([a-z][a-z0-9+.-]*):\/\//i.exec(canonical)?.[1];

  return {
    canonical,
    roundTrips: normalizeArchiveLocation(rebuilt) === canonical,
    schemePreserved: !scheme || rebuilt.startsWith(`${scheme}://`),
  };
}
