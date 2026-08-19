import type { SurfaceCapability } from '../surfaces/surface-contribution';

export interface SurfaceParityState {
  readonly surfaceAwarePins: boolean;
  readonly quickAccessSurfaceTargets: boolean;
  readonly capabilityResolution: boolean;
  readonly genericInvocationProxy: boolean;
  readonly surfaceFocus: boolean;
}

export function missingSurfaceParity(
  state: SurfaceParityState,
): string[] {
  return Object.entries(state)
    .filter(([, restored]) => !restored)
    .map(([feature]) => feature);
}

export function capabilitySet(
  capabilities: readonly SurfaceCapability[],
): ReadonlySet<SurfaceCapability> {
  return new Set(capabilities);
}