import { Injectable, Type } from '@angular/core';
import { ContributionRegistry } from '../plugin/contribution-registry';
import { SurfaceCapability, SurfaceContribution, SurfaceRequest } from './surface-contribution';
import { DirSurfaceTabComponent } from './builtins/dir-surface-tab.component';
import { SearchResultsSurfaceTabComponent } from './builtins/search-results-surface-tab.component';
import { ViewerSurfaceTabComponent } from './builtins/viewer-surface-tab.component';
import { SizeManagerSurfaceTabComponent } from './size-manager/size-manager-surface-tab.component';
import { DuplicateFinderSurfaceTabComponent } from './builtins/duplicate-finder-surface-tab.component';
import { CliSurfaceTabComponent } from './builtins/cli-surface-tab.component';

function tabSurfaceId(req: SurfaceRequest): string {
  return req.tab.surfaceId ?? req.tab.kind ?? 'dir';
}

@Injectable({ providedIn: 'root' })
export class SurfaceRegistryService {
  readonly registry = new ContributionRegistry<SurfaceContribution, SurfaceRequest>();

  constructor() {
    this.register({
      id: 'dir',
      capabilities: ['navigate', 'selection', 'fileOperations', 'preview'],
      score: (req) => (tabSurfaceId(req) === 'dir' ? 100 : 0),
      load: () => DirSurfaceTabComponent,
    });

    this.register({
      id: 'search-results',
      capabilities: ['selection', 'fileOperations', 'preview'],
      score: (req) => (tabSurfaceId(req) === 'search-results' ? 100 : 0),
      load: () => SearchResultsSurfaceTabComponent,
    });

    this.register({
      id: 'viewer',
      capabilities: ['viewerControls'],
      score: (req) => (tabSurfaceId(req) === 'viewer' ? 100 : 0),
      load: () => ViewerSurfaceTabComponent,
    });

    this.register({
      id: 'size-manager',
      capabilities: ['navigate'],
      score: (req) => (tabSurfaceId(req) === 'size-manager' ? 100 : 0),
      load: () => SizeManagerSurfaceTabComponent,
    });

    this.register({
      id: 'duplicates',
      capabilities: [],
      score: (req) => (tabSurfaceId(req) === 'duplicates' ? 100 : 0),
      load: () => DuplicateFinderSurfaceTabComponent,
    });

    this.register({
      id: 'cli',
      capabilities: [],
      score: (req) => (tabSurfaceId(req) === 'cli' ? 100 : 0),
      load: () => CliSurfaceTabComponent,
    });
  }

  register(c: SurfaceContribution): void {
    this.registry.register(c);
  }

  unregister(id: string): void {
    this.registry.unregister(id);
  }

  async resolve(req: SurfaceRequest): Promise<{ id: string; component: Type<unknown> }> {
    const id = tabSurfaceId(req);
    const exact = this.registry.list().find((candidate) => candidate.id === id);
    if (exact) {
      return { id: exact.id, component: await exact.load() };
    }
    return this.registry.resolve(req);
  }

  getCapabilities(req: SurfaceRequest): ReadonlySet<SurfaceCapability> {
    const id = tabSurfaceId(req);
    const exact = this.registry.list().find((candidate) => candidate.id === id);
    if (exact) {
      return new Set(exact.capabilities ?? []);
    }

    let best: SurfaceContribution | null = null;
    let bestScore = 0;
    for (const candidate of this.registry.list()) {
      const score = candidate.score(req);
      if (score > bestScore) {
        best = candidate;
        bestScore = score;
      }
    }
    return new Set(best?.capabilities ?? []);
  }

  list(): readonly SurfaceContribution[] {
    return this.registry.list();
  }
}