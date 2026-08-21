import { Injectable } from '@angular/core';
import {
  PaneRendererRegistry,
  schemeOf,
  type PaneRendererContribution as CorePaneRendererContribution,
  type PaneRendererMatch,
} from '@reassembly/core/runtime/panes';

export type PaneRendererKind =
  | 'directory'
  | 'registry'
  | 'autorun'
  | 'network'
  | 'environment'
  | 'system-table';

export type PaneRendererContribution = CorePaneRendererContribution<
  Exclude<PaneRendererKind, 'directory'>
>;

export type { PaneRendererMatch };

export const SYSTEM_TABLE_SCHEMES = new Set([
  'tasks',
  'services',
  'processes',
  'devices',
  'shell-extensions',
]);

@Injectable({ providedIn: 'root' })
export class PaneRendererRegistryService {
  private readonly registry = new PaneRendererRegistry<PaneRendererKind>('directory');

  constructor() {
    this.register({
      id: 'builtin.registry-pane',
      kind: 'registry',
      score: scheme => scheme === 'registry' ? 100 : 0,
    });
    this.register({
      id: 'builtin.autorun-pane',
      kind: 'autorun',
      score: scheme => scheme === 'autorun' ? 100 : 0,
    });
    this.register({
      id: 'builtin.network-pane',
      kind: 'network',
      score: scheme => scheme === 'network' ? 100 : 0,
    });
    this.register({
      id: 'builtin.environment-pane',
      kind: 'environment',
      score: scheme => scheme === 'environment' ? 100 : 0,
    });
    this.register({
      id: 'builtin.system-table-pane',
      kind: 'system-table',
      score: scheme => SYSTEM_TABLE_SCHEMES.has(scheme) ? 100 : 0,
    });
  }

  register(contribution: PaneRendererContribution): () => void {
    return this.registry.register(contribution);
  }

  resolve(path: string): PaneRendererMatch<PaneRendererKind> {
    return this.registry.resolve(path);
  }
}

export { schemeOf };