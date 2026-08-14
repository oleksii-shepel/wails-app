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

  create(location: string, label?: string, icon?: string, addedAt = Date.now()): PinnedLocation | null {
    const canonical = this.canonicalize(location);
    if (!canonical) return null;
    const scheme = schemeOfLocation(canonical);
    return {
      id: locationIdentity(canonical),
      location: canonical,
      scheme,
      kind: kindOfLocation(scheme),
      label: label?.trim() || locationLabel(canonical),
      icon: icon?.trim() || locationIcon(scheme),
      addedAt,
    };
  }

  isPinned(pins: readonly PinnedLocation[], location: string): boolean {
    const id = locationIdentity(location);
    return !!id && pins.some(pin => pin.id === id);
  }

  remove(pins: readonly PinnedLocation[], location: string): PinnedLocation[] {
    const id = locationIdentity(location);
    return pins.filter(pin => pin.id !== id);
  }

  schemeOf(location: string): string {
    return schemeOfLocation(location);
  }

  private canonicalize(location: string): string {
    return canonicalizeLocation(location);
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
