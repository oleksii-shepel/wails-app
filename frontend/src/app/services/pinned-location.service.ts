import { Injectable } from '@angular/core';
import {
  LEGACY_PINNED_PATHS_KEY,
  PINNED_LOCATIONS_KEY,
  PinnedLocation,
  PinnedLocationKind,
} from '../components/address-bar.types';
import { buildSegments, isWindowsPath, normalizePath, shortenPath } from '../components/address-bar.utils';

interface LegacyPinnedPath {
  path: string;
  label?: string;
  icon?: string;
  addedAt?: number;
}

@Injectable({ providedIn: 'root' })
export class PinnedLocationService {
  load(): PinnedLocation[] {
    const current = this.read<PinnedLocation[]>(PINNED_LOCATIONS_KEY);
    if (current) return current.map(pin => this.normalizeStoredPin(pin)).filter((pin): pin is PinnedLocation => !!pin);

    const legacy = this.read<LegacyPinnedPath[]>(LEGACY_PINNED_PATHS_KEY) ?? [];
    const migrated = legacy
      .map(pin => this.create(pin.path, pin.label, pin.icon, pin.addedAt))
      .filter((pin): pin is PinnedLocation => !!pin);

    if (migrated.length > 0) this.save(migrated);
    return migrated;
  }

  save(pins: readonly PinnedLocation[]): void {
    try {
      localStorage.setItem(PINNED_LOCATIONS_KEY, JSON.stringify(pins));
    } catch {
      // Pinning is a convenience feature; storage failures must not break navigation.
    }
  }

  create(location: string, label?: string, icon?: string, addedAt = Date.now()): PinnedLocation | null {
    const canonical = this.canonicalize(location);
    if (!canonical) return null;
    const scheme = this.schemeOf(canonical);
    return {
      id: this.identityOf(canonical),
      location: canonical,
      scheme,
      kind: this.kindOf(scheme),
      label: label?.trim() || this.labelOf(canonical),
      icon: icon?.trim() || this.iconOf(scheme),
      addedAt,
    };
  }

  isPinned(pins: readonly PinnedLocation[], location: string): boolean {
    const id = this.identityOf(location);
    return !!id && pins.some(pin => pin.id === id);
  }

  remove(pins: readonly PinnedLocation[], location: string): PinnedLocation[] {
    const id = this.identityOf(location);
    return pins.filter(pin => pin.id !== id);
  }

  schemeOf(location: string): string {
    const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec(location.trim());
    return match ? match[1].toLowerCase() : 'filesystem';
  }

  private kindOf(scheme: string): PinnedLocationKind {
    if (scheme === 'search') return 'query';
    return scheme === 'surface' ? 'surface' : 'location';
  }

  private canonicalize(location: string): string {
    const trimmed = location.trim();
    if (!trimmed) return '';
    return this.schemeOf(trimmed) === 'filesystem' ? normalizePath(trimmed) : trimmed;
  }

  private identityOf(location: string): string {
    const canonical = this.canonicalize(location);
    if (!canonical) return '';
    const scheme = this.schemeOf(canonical);
    const identityLocation = scheme === 'filesystem' ? canonical.toLowerCase() : canonical;
    return `${scheme}:${identityLocation}`;
  }

  private labelOf(location: string): string {
    if (this.schemeOf(location) !== 'filesystem') {
      try {
        const url = new URL(location);
        const query = url.searchParams.get('pattern') || url.searchParams.get('q');
        if (query) return query;
        const tail = url.pathname.split('/').filter(Boolean).at(-1);
        return tail || `${url.protocol.slice(0, -1)} location`;
      } catch {
        return location;
      }
    }

    const segments = buildSegments(location);
    return segments.at(-1)?.label || (isWindowsPath(location) ? shortenPath(location) : '/');
  }

  private iconOf(scheme: string): string {
    const icons: Record<string, string> = {
      search: '🔎', archive: '🗜️', s3: '☁️', smb: '🖧',
      gdrive: '☁️', onedrive: '☁️', dropbox: '☁️',
    };
    return icons[scheme] ?? '📁';
  }

  private normalizeStoredPin(pin: PinnedLocation): PinnedLocation | null {
    if (!pin || typeof pin.location !== 'string') return null;
    return this.create(pin.location, pin.label, pin.icon, Number(pin.addedAt) || Date.now());
  }

  private read<T>(key: string): T | null {
    try {
      const raw = localStorage.getItem(key);
      return raw ? JSON.parse(raw) as T : null;
    } catch {
      return null;
    }
  }
}
