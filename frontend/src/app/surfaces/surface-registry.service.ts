import { Injectable, Type } from '@angular/core';
import { ContributionRegistry } from '../plugin/contribution-registry';
import { SurfaceContribution, SurfaceRequest } from './surface-contribution';
import { DirSurfaceTabComponent } from './builtins/dir-surface-tab.component';
import { ViewerSurfaceTabComponent } from './builtins/viewer-surface-tab.component';
import { SizeManagerSurfaceTabComponent } from './size-manager/size-manager-surface-tab.component';

@Injectable({ providedIn: 'root' })
export class SurfaceRegistryService {
  readonly registry = new ContributionRegistry<SurfaceContribution, SurfaceRequest>();

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
    this.registry.register(c);
  }

  unregister(id: string): void {
    this.registry.unregister(id);
  }

  async resolve(req: SurfaceRequest): Promise<{ id: string; component: Type<unknown> }> {
    return this.registry.resolve(req);
  }

  list(): readonly SurfaceContribution[] {
    return this.registry.list();
  }
}
