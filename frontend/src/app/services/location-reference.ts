import { buildSegments, isWindowsPath, normalizePath, shortenPath } from '../components/address-bar.utils';

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

export function schemeOfLocation(location: string): string {
  const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec((location ?? '').trim());
  return match ? match[1].toLowerCase() : 'filesystem';
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
  return schemeOfLocation(trimmed) === 'filesystem' ? normalizePath(trimmed) : trimmed;
}

export function locationIdentity(location: string): string {
  const canonical = canonicalizeLocation(location);
  if (!canonical) return '';
  const scheme = schemeOfLocation(canonical);
  const value = scheme === 'filesystem' ? canonical.toLowerCase() : canonical;
  return `${scheme}:${value}`;
}

export function locationLabel(location: string): string {
  const canonical = canonicalizeLocation(location);
  if (!canonical) return '';
  const scheme = schemeOfLocation(canonical);

  if (scheme !== 'filesystem') {
    try {
      const url = new URL(canonical);
      const query = url.searchParams.get('pattern') || url.searchParams.get('q');
      if (query) return query;
      const tail = url.pathname.split('/').filter(Boolean).at(-1);
      return tail || `${scheme} location`;
    } catch {
      const withoutScheme = canonical.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '');
      return withoutScheme.split('/').filter(Boolean).at(-1) || `${scheme} location`;
    }
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
