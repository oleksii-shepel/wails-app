import { Injectable, Type } from '@angular/core';
import { ContributionRegistry } from '../plugin/contribution-registry';
import { SurfaceContribution, SurfaceRequest } from './surface-contribution';
import { DirSurfaceTabComponent } from './builtins/dir-surface-tab.component';
import { ViewerSurfaceTabComponent } from './builtins/viewer-surface-tab.component';
import { SizeManagerSurfaceTabComponent } from './size-manager/size-manager-surface-tab.component';
import { tabSurfaceId } from './tab-surface';

@Injectable({ providedIn: 'root' })
export class SurfaceRegistryService {
  readonly registry = new ContributionRegistry<SurfaceContribution, SurfaceRequest>();

  constructor() {
    // Scores are compatibility/fallback behavior. Persisted surfaceId is the
    // primary identity and is forced during resolve().
    this.register({
      id: 'dir',
      score: (req) => (tabSurfaceId(req.tab) === 'dir' ? 100 : 0),
      load: () => DirSurfaceTabComponent,
    });

    this.register({
      id: 'viewer',
      score: (req) => (tabSurfaceId(req.tab) === 'viewer' ? 100 : 0),
      load: () => ViewerSurfaceTabComponent,
    });

    this.register({
      id: 'size-manager',
      score: (req) => (tabSurfaceId(req.tab) === 'size-manager' ? 100 : 0),
      load: () => SizeManagerSurfaceTabComponent,
    });
  }

  register(c: SurfaceContribution): void {
    this.registry.register(c);
  }

  unregister(id: string): void {
    this.registry.unregister(id);
  }

  async resolve(req: SurfaceRequest): Promise<{ id: string; component: Type<unknown> }> {
    // Exact identity first. This is what makes plugin/tool surfaces extensible:
    // a tab with surfaceId="console" resolves the registered "console" surface
    // without adding another case to the pane or a central type union.
    return this.registry.resolve(req, tabSurfaceId(req.tab));
  }

  list(): readonly SurfaceContribution[] {
    return this.registry.list();
  }
}
