import { readFileSync } from 'node:fs';

describe('Recycle Bin source invalidation architecture', () => {
  const api = readFileSync(new URL('../services/api.service.ts', import.meta.url), 'utf8');
  const pane = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');
  const bus = readFileSync(new URL('../services/resource-mutation.service.ts', import.meta.url), 'utf8');

  it('uses the shared ResourceMutationService rather than a Bin-only event bus', () => {
    expect(api).toContain("items: [{ source: 'bin://' }]");
    expect(api).toContain("origin: 'recycle-bin'");
    expect(pane).toContain('private readonly resourceMutations: ResourceMutationService');
    expect(pane).toContain('this.resourceMutations.subscribe(');
    expect(pane).not.toContain('BinWatcherService');
    expect(pane).not.toContain("EventsOn('bin:");
  });

  it('coalesces bursts centrally and prevents overlapping Bin reloads', () => {
    expect(bus).toContain('MUTATION_COALESCE_MS');
    expect(bus).toContain('private readonly pending = new Map<ResourceMutationKind, PendingMutation>()');
    expect(pane).toContain('binMutationRefreshRunning');
    expect(pane).toContain('binMutationRefreshPending');
  });
});
