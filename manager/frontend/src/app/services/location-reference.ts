import { buildSegments, isWindowsPath, normalizePath, pathIdentityKey, shortenPath } from '../components/address-bar.utils';

import type { LocationKind, LocationReference } from '../shared/protocol-enhanced';
export type { LocationKind, LocationReference } from '../shared/protocol-enhanced';

const LOCATION_ICONS: Readonly<Record<string, string>> = {
  search: '🔎',
  archive: '🗜️',
  s3: '☁️',
  smb: '🖧',
  gdrive: '☁️',
  onedrive: '☁️',
  dropbox: '☁️',
  registry: '🧩',
  memory: '🧠',
  http: '🌐',
  https: '🔒',
  surface: '◫',
};

interface OpaqueLocationParts {
  readonly scheme: string;
  readonly body: string;
  readonly pathBody: string;
  readonly query: URLSearchParams;
}

function splitOpaqueLocation(location: string): OpaqueLocationParts | null {
  const value = (location ?? '').trim();
  const match = /^([a-z][a-z0-9+.-]*):\/\/(.*)$/i.exec(value);
  if (!match) return null;

  const body = match[2] ?? '';
  const queryIndex = body.indexOf('?');
  const pathBody = queryIndex >= 0 ? body.slice(0, queryIndex) : body;
  const queryText = queryIndex >= 0 ? body.slice(queryIndex + 1) : '';

  return {
    // Scheme comparison is case-insensitive, but the location string itself is
    // never reconstructed from this normalized value.
    scheme: match[1].toLowerCase(),
    body,
    pathBody,
    query: new URLSearchParams(queryText),
  };
}

export function schemeOfLocation(location: string): string {
  return splitOpaqueLocation(location)?.scheme ?? 'filesystem';
}

export function kindOfLocation(locationOrScheme: string): LocationKind {
  const scheme = locationOrScheme.includes('://')
    ? schemeOfLocation(locationOrScheme)
    : locationOrScheme.toLowerCase();
  if (scheme === 'search') return 'query';
  if (scheme === 'surface') return 'surface';
  return 'location';
}

export function canonicalizeLocation(location: string): string {
  const trimmed = (location ?? '').trim();
  if (!trimmed) return '';

  // Non-filesystem locations are opaque provider-owned identifiers.
  // Do not feed them through URL, pathname, hostname, or case normalization.
  return splitOpaqueLocation(trimmed) ? trimmed : normalizePath(trimmed);
}

export function locationIdentity(location: string): string {
  const canonical = canonicalizeLocation(location);
  if (!canonical) return '';
  const scheme = schemeOfLocation(canonical);
  const value = scheme === 'filesystem' ? pathIdentityKey(canonical) : canonical;
  return `${scheme}:${value}`;
}

export function locationLabel(location: string): string {
  const canonical = canonicalizeLocation(location);
  if (!canonical) return '';

  const opaque = splitOpaqueLocation(canonical);
  if (opaque) {
    const query = opaque.query.get('pattern') || opaque.query.get('q');
    if (query) return query;

    // Important: never use `new URL(canonical)` here. For plugin schemes,
    // the first component after `://` is not a hostname. URL parsing would
    // case-fold it (for example HKEY_LOCAL_MACHINE -> hkey_local_machine).
    const tail = opaque.pathBody.split('/').filter(Boolean).at(-1);
    return tail || `${opaque.scheme} location`;
  }

  const segments = buildSegments(canonical);
  return segments.at(-1)?.label || (isWindowsPath(canonical) ? shortenPath(canonical) : '/');
}

export function locationIcon(locationOrScheme: string): string {
  const scheme = locationOrScheme.includes('://')
    ? schemeOfLocation(locationOrScheme)
    : locationOrScheme.toLowerCase();
  return LOCATION_ICONS[scheme] ?? '📁';
}

export function describeLocation(location: string): LocationReference | null {
  const canonical = canonicalizeLocation(location);
  if (!canonical) return null;
  const scheme = schemeOfLocation(canonical);
  return { location: canonical, scheme, kind: kindOfLocation(scheme) };
}
