import { Injectable } from '@angular/core';

import type {
  WorkspaceSurfaceActionContribution,
  WorkspaceSurfaceActionContext,
  WorkspaceSurfaceActionEvent,
  WorkspaceSurfaceActionHostServices,
} from './workspace-surface-action.types';

@Injectable({ providedIn: 'root' })
export class WorkspaceSurfaceActionRegistryService {
  private readonly contributions =
    new Map<string, WorkspaceSurfaceActionContribution>();

  register(
    contribution: WorkspaceSurfaceActionContribution,
  ): void {
    this.contributions.set(contribution.id, contribution);
  }

  unregister(id: string): void {
    this.contributions.delete(id);
  }

  list(): readonly WorkspaceSurfaceActionContribution[] {
    return [...this.contributions.values()];
  }

  resolve(
    event: WorkspaceSurfaceActionEvent,
  ): WorkspaceSurfaceActionContribution | null {
    let best: WorkspaceSurfaceActionContribution | null = null;
    let bestScore = 0;

    for (const contribution of this.contributions.values()) {
      if (
        contribution.action !== event.action ||
        (contribution.surfaceId &&
          contribution.surfaceId !== event.surface.surfaceId)
      ) {
        continue;
      }

      const score =
        contribution.score?.(event) ??
        (contribution.surfaceId === event.surface.surfaceId ? 100 : 10);

      if (score > bestScore) {
        best = contribution;
        bestScore = score;
      }
    }

    return best;
  }

  async dispatch(
    event: WorkspaceSurfaceActionEvent,
    services: WorkspaceSurfaceActionHostServices,
  ): Promise<boolean> {
    const contribution = this.resolve(event);
    if (!contribution) return false;

    const context: WorkspaceSurfaceActionContext = {
      event,
      services,
    };

    await contribution.execute(context);
    return true;
  }
}
