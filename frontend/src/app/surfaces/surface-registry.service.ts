import { Injectable, Type } from '@angular/core';
import { SurfaceContribution, SurfaceRequest } from './surface-contribution';
import { DirSurfaceTabComponent } from './builtins/dir-surface-tab.component';
import { ViewerSurfaceTabComponent } from './builtins/viewer-surface-tab.component';
import { SizeManagerSurfaceTabComponent } from './size-manager/size-manager-surface-tab.component';

@Injectable({ providedIn: 'root' })
export class SurfaceRegistryService {
  private readonly contributions: SurfaceContribution[] = [];
  private readonly loaded = new Map<string, Type<unknown>>();

  constructor() {
    // Built-in surfaces. Foreign surfaces can be registered later without
    // changing panes/tab infrastructure.
    this.register({
      id: 'dir',
      score: (req) => ((req.tab.kind ?? 'dir') === 'dir' ? 100 : 0),
      load: () => DirSurfaceTabComponent,
    });

    this.register({
      id: 'viewer',
      score: (req) => ((req.tab.kind ?? 'dir') === 'viewer' ? 100 : 0),
      load: () => ViewerSurfaceTabComponent,
    });

    this.register({
      id: 'size-manager',
      score: (req) => ((req.tab.kind ?? 'dir') === 'size-manager' ? 100 : 0),
      load: () => SizeManagerSurfaceTabComponent,
    });
  }

  register(c: SurfaceContribution): void {
    const i = this.contributions.findIndex(x => x.id === c.id);
    if (i >= 0) this.contributions.splice(i, 1, c);
    else this.contributions.push(c);
  }

  async resolve(req: SurfaceRequest): Promise<{ id: string; component: Type<unknown> }> {
    let best: SurfaceContribution | null = null;
    let bestScore = 0;
    for (const c of this.contributions) {
      const s = c.score(req);
      if (s > bestScore) { bestScore = s; best = c; }
    }
    if (!best) best = this.contributions[0];
    return { id: best.id, component: await this.loadContribution(best) };
  }

  private async loadContribution(c: SurfaceContribution): Promise<Type<unknown>> {
    const cached = this.loaded.get(c.id);
    if (cached) return cached;
    const loaded = await Promise.resolve(c.load()) as Type<unknown>;
    this.loaded.set(c.id, loaded);
    return loaded;
  }
}
