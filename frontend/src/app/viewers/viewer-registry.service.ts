import { Injectable, Type } from '@angular/core';
import { ViewerContribution, ViewerRequest } from './viewer-contribution';
import { ViewerSurfaceComponent } from '../components/viewer-surface.component';

function extLower(path: string): string {
  const p = (path ?? '').replace(/\\/g, '/');
  const i = p.lastIndexOf('.');
  if (i < 0) return '';
  return p.slice(i + 1).toLowerCase();
}

@Injectable({ providedIn: 'root' })
export class ViewerRegistryService {
  private readonly contributions: ViewerContribution[] = [];
  private readonly loaded = new Map<string, Type<unknown>>();

  constructor() {
    // Built-ins. This is the "spine" we can extend later with external bundles.
    this.register({
      id: 'legacy',
      title: 'Legacy Viewer',
      score: () => 1, // fallback for everything
      load: () => ViewerSurfaceComponent,
    });

    this.register({
      id: 'image',
      title: 'Image Viewer',
      score: (req) => {
        const ext = extLower(req.archiveContext?.entryPath ?? req.targetPath);
        if (ext === 'svg') return 50;
        if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'ico', 'avif'].includes(ext)) return 50;
        return 0;
      },
      load: async () => (await import('./builtins/image-viewer.component')).ImageViewerComponent,
    });
  }

  register(c: ViewerContribution): void {
    // Replace-by-id to support hot-reload/dev and later plugin updates.
    const i = this.contributions.findIndex(x => x.id === c.id);
    if (i >= 0) this.contributions.splice(i, 1, c);
    else this.contributions.push(c);
  }

  /**
   * Resolve a viewer component for the request.
   * If viewerId is provided and registered, it is forced.
   */
  async resolve(req: ViewerRequest): Promise<{ id: string; component: Type<unknown> }> {
    const forced = (req.viewerId ?? '').trim();
    if (forced) {
      const c = this.contributions.find(x => x.id === forced);
      if (c) return { id: c.id, component: await this.loadContribution(c) };
    }

    let best: ViewerContribution | null = null;
    let bestScore = 0;
    for (const c of this.contributions) {
      const s = c.score(req);
      if (s > bestScore) {
        bestScore = s;
        best = c;
      }
    }
    if (!best) {
      // Should be unreachable due to legacy fallback registration.
      best = this.contributions[0];
    }
    return { id: best.id, component: await this.loadContribution(best) };
  }

  private async loadContribution(c: ViewerContribution): Promise<Type<unknown>> {
    const cached = this.loaded.get(c.id);
    if (cached) return cached;
    const loaded = await Promise.resolve(c.load()) as Type<unknown>;
    this.loaded.set(c.id, loaded);
    return loaded;
  }
}
