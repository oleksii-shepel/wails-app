import type { PinnedLocation } from '../components/address-bar.types';

export interface RelatedLocationContext {
  currentLocation: string;
  currentSurfaceId: string;
  /** Durable location that represents the semantic context (e.g. a search base path). */
  contextLocation: string | null;
}

export interface RelatedLocationContribution {
  id: string;
  /** 0 = unrelated; higher scores appear first. */
  score(candidate: PinnedLocation, context: RelatedLocationContext): number;
}