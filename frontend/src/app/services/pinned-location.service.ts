import { Injectable } from '@angular/core';
import {
  LEGACY_PINNED_PATHS_KEY,
  PINNED_LOCATIONS_KEY,
  PinnedLocation,
} from '../components/address-bar.types';
import {
  canonicalizeLocation,
  kindOfLocation,
  locationIcon,
  locationIdentity,
  locationLabel,
  schemeOfLocation,
} from './location-reference';

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

  create(
    location: string,
    label?: string,
    icon?: string,
    addedAt = Date.now(),
    surfaceId?: string,
  ): PinnedLocation | null {
    const canonical = this.canonicalize(location);
    if (!canonical) return null;
    const scheme = schemeOfLocation(canonical);
    const resolvedSurfaceId = this.resolveSurfaceId(canonical, surfaceId);
    return {
      id: this.identity(canonical, resolvedSurfaceId),
      location: canonical,
      scheme,
      kind: kindOfLocation(scheme),
      surfaceId: resolvedSurfaceId,
      label: label?.trim() || locationLabel(canonical),
      icon: icon?.trim() || locationIcon(scheme),
      addedAt,
    };
  }

  isPinned(pins: readonly PinnedLocation[], location: string, surfaceId?: string): boolean {
    const canonical = this.canonicalize(location);
    if (!canonical) return false;
    const id = this.identity(canonical, this.resolveSurfaceId(canonical, surfaceId));
    return pins.some(pin => pin.id === id);
  }

  remove(pins: readonly PinnedLocation[], location: string, surfaceId?: string): PinnedLocation[] {
    const canonical = this.canonicalize(location);
    if (!canonical) return [...pins];
    const id = this.identity(canonical, this.resolveSurfaceId(canonical, surfaceId));
    return pins.filter(pin => pin.id !== id);
  }

  schemeOf(location: string): string {
    return schemeOfLocation(location);
  }

  rename(pins: readonly PinnedLocation[], pinId: string, label: string): PinnedLocation[] {
    const nextLabel = label.trim();
    if (!nextLabel) return [...pins];
    return pins.map(pin => pin.id === pinId ? { ...pin, label: nextLabel } : pin);
  }

  related(pins: readonly PinnedLocation[], currentLocation: string): PinnedLocation[] {
    const context = this.contextLocation(currentLocation);
    if (!context) return [];

    return pins
      .map((pin, index) => ({ pin, index, score: this.relatedScore(pin, currentLocation, context) }))
      .filter(entry => entry.score > 0)
      .sort((a, b) => b.score - a.score || b.pin.addedAt - a.pin.addedAt || a.index - b.index)
      .map(entry => entry.pin);
  }

  contextLocation(location: string): string | null {
    const canonical = this.canonicalize(location);
    if (!canonical) return null;
    if (schemeOfLocation(canonical) !== 'search') return canonical;

    try {
      const query = canonical.slice('search://'.length);
      const params = new URLSearchParams(query.startsWith('?') ? query.slice(1) : query);
      const base = params.get('path')?.trim();
      return base ? this.canonicalize(base) : null;
    } catch {
      return null;
    }
  }

  private canonicalize(location: string): string {
    return canonicalizeLocation(location);
  }

  private normalizeStoredPin(pin: PinnedLocation): PinnedLocation | null {
    if (!pin || typeof pin.location !== 'string') return null;
    return this.create(
      pin.location,
      pin.label,
      pin.icon,
      Number(pin.addedAt) || Date.now(),
      typeof pin.surfaceId === 'string' && !(schemeOfLocation(pin.location) === 'search' && pin.surfaceId === 'dir')
        ? pin.surfaceId
        : undefined,
    );
  }


  private relatedScore(pin: PinnedLocation, currentLocation: string, contextLocation: string): number {
    const pinContext = this.contextLocation(pin.location);
    if (!pinContext) return 0;

    const currentScheme = schemeOfLocation(currentLocation);
    const pinScheme = pin.scheme;
    const contextId = locationIdentity(contextLocation);
    const pinContextId = locationIdentity(pinContext);

    if (contextId && contextId === pinContextId) {
      return pinScheme === 'search' ? 1000 : (pinScheme === currentScheme && currentScheme !== 'filesystem' ? 900 : 0);
    }

    if (pinScheme === 'search' && schemeOfLocation(contextLocation) === 'filesystem' && schemeOfLocation(pinContext) === 'filesystem') {
      const left = contextLocation.replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase();
      const right = pinContext.replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase();
      if (left && right) {
        if (right.startsWith(`${left}/`)) return 760;
        if (left.startsWith(`${right}/`)) return 700;
      }
    }

    if (pinScheme === currentScheme && currentScheme !== 'filesystem') return 300;
    return 0;
  }

  private identity(location: string, surfaceId: string): string {
    return `${locationIdentity(location)}|surface:${surfaceId}`;
  }

  private resolveSurfaceId(location: string, surfaceId?: string): string {
    const explicit = surfaceId?.trim();
    if (explicit) return explicit;

    if (schemeOfLocation(location) === 'search') return 'search-results';
    if (kindOfLocation(location) !== 'surface') return 'dir';

    try {
      const parsed = new URL(location);
      const fromHost = parsed.hostname.trim();
      if (fromHost) return fromHost;
      const fromPath = parsed.pathname.split('/').filter(Boolean)[0]?.trim();
      if (fromPath) return fromPath;
    } catch {}

    return 'dir';
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
