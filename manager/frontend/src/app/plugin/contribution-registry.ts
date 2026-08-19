import { Type } from '@angular/core';

/**
 * Generic contribution registry used by ViewerRegistryService,
 * SurfaceRegistryService, and future plugin-provided registries.
 *
 * T is the specific contribution type (e.g., ViewerContribution).
 */
export interface Contribution<TReq> {
  id: string;
  score(req: TReq): number;
  load(): Promise<Type<unknown>> | Type<unknown>;
}

export class ContributionRegistry<TContrib extends Contribution<TReq>, TReq> {
  private readonly contributions: TContrib[] = [];
  private readonly loaded = new Map<string, Type<unknown>>();

  register(c: TContrib): void {
    const i = this.contributions.findIndex(x => x.id === c.id);
    if (i >= 0) {
      this.contributions.splice(i, 1, c);
    } else {
      this.contributions.push(c);
    }
  }

  unregister(id: string): void {
    const i = this.contributions.findIndex(x => x.id === id);
    if (i >= 0) {
      this.contributions.splice(i, 1);
    }
    this.loaded.delete(id);
  }

  async resolve(req: TReq, forcedId?: string | null): Promise<{ id: string; component: Type<unknown> }> {
    const best = this.pick(req, forcedId);
    const component = await this.load(best);
    return { id: best.id, component };
  }

  pick(req: TReq, forcedId?: string | null): TContrib {
    const forced = (forcedId ?? '').trim();
    if (forced) {
      const c = this.contributions.find(x => x.id === forced);
      if (c) return c;
    }

    let best: TContrib | null = null;
    let bestScore = 0;
    for (const c of this.contributions) {
      const s = c.score(req);
      if (s > bestScore) {
        bestScore = s;
        best = c;
      }
    }
    if (!best) {
      // Should be unreachable when a fallback is registered.
      best = this.contributions[0];
    }
    return best!;
  }

  list(): readonly TContrib[] {
    return this.contributions;
  }

  private load(c: TContrib): Promise<Type<unknown>> {
    const cached = this.loaded.get(c.id);
    if (cached) {
      return Promise.resolve(cached);
    }
    const loaded = Promise.resolve(c.load()) as Promise<Type<unknown>>;
    return loaded.then(component => {
      this.loaded.set(c.id, component);
      return component;
    });
  }
}