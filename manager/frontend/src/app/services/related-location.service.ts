import { Injectable } from '@angular/core';
import type { PinnedLocation } from '../components/address-bar.types';
import { locationIdentity, schemeOfLocation } from './location-reference';
import { PinnedLocationService } from './pinned-location.service';
import type { RelatedLocationContext, RelatedLocationContribution } from './related-location.types';

@Injectable({ providedIn: 'root' })
export class RelatedLocationService {
  private readonly contributions = new Map<string, RelatedLocationContribution>();

  constructor(private readonly pins: PinnedLocationService) {
    this.register({
      id: 'builtin.saved-search-context',
      score: (candidate, context) => this.scoreSavedSearch(candidate, context),
    });
  }

  register(contribution: RelatedLocationContribution): void {
    this.contributions.set(contribution.id, contribution);
  }

  unregister(id: string): void {
    this.contributions.delete(id);
  }

  list(): readonly RelatedLocationContribution[] {
    return Array.from(this.contributions.values());
  }

  resolve(
    candidates: readonly PinnedLocation[],
    currentLocation: string,
    currentSurfaceId = 'dir',
  ): PinnedLocation[] {
    const context: RelatedLocationContext = {
      currentLocation,
      currentSurfaceId,
      contextLocation: this.pins.contextLocation(currentLocation),
    };
    if (!context.contextLocation) return [];

    return candidates
      .map((candidate, index) => ({
        candidate,
        index,
        score: this.scoreCandidate(candidate, context),
      }))
      .filter(entry => entry.score > 0)
      .sort((a, b) => b.score - a.score || b.candidate.addedAt - a.candidate.addedAt || a.index - b.index)
      .map(entry => entry.candidate);
  }

  private scoreCandidate(candidate: PinnedLocation, context: RelatedLocationContext): number {
    if (this.isCurrentPin(candidate, context)) return 0;

    let best = 0;
    for (const contribution of this.contributions.values()) {
      try {
        best = Math.max(best, Number(contribution.score(candidate, context)) || 0);
      } catch (error) {
        console.error(`Related-location contribution ${contribution.id} failed:`, error);
      }
    }
    return best;
  }

  private isCurrentPin(candidate: PinnedLocation, context: RelatedLocationContext): boolean {
    return candidate.surfaceId === context.currentSurfaceId
      && locationIdentity(candidate.location) === locationIdentity(context.currentLocation);
  }

  private scoreSavedSearch(candidate: PinnedLocation, context: RelatedLocationContext): number {
    if (candidate.scheme !== 'search' || !context.contextLocation) return 0;

    const candidateContext = this.pins.contextLocation(candidate.location);
    if (!candidateContext) return 0;

    const contextId = locationIdentity(context.contextLocation);
    const candidateId = locationIdentity(candidateContext);
    if (contextId && contextId === candidateId) return 1000;

    if (schemeOfLocation(context.contextLocation) !== 'filesystem' || schemeOfLocation(candidateContext) !== 'filesystem') {
      return 0;
    }

    const current = normalizeComparablePath(context.contextLocation);
    const saved = normalizeComparablePath(candidateContext);
    if (!current || !saved) return 0;

    if (saved.startsWith(`${current}/`)) return 760;
    if (current.startsWith(`${saved}/`)) return 700;
    return 0;
  }
}

function normalizeComparablePath(value: string): string {
  return value.replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase();
}