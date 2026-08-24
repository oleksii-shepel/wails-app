import { readFileSync } from 'node:fs';

describe('Recycle Bin source invalidation architecture', () => {
  const api = readFileSync(
    new URL('../services/api.service.ts', import.meta.url),
    'utf8',
  );
  const pane = readFileSync(
    new URL('./bin-pane.component.ts', import.meta.url),
    'utf8',
  );
  const observer = readFileSync(
    new URL('../services/source-change-observer.service.ts', import.meta.url),
    'utf8',
  );
  const bus = readFileSync(
    new URL('../services/resource-mutation.service.ts', import.meta.url),
    'utf8',
  );

  it('uses the shared SourceChangeObserverService for Bin invalidation', () => {
    expect(pane).toContain('SourceChangeObserverService');
    expect(pane).toContain('this.sourceChanges.observe(');
    expect(pane).toContain(
      'this.sourceObservation.setLocation(BIN_ROOT_PATH);',
    );
    expect(pane).not.toContain('BinWatcherService');
    expect(pane).not.toContain("EventsOn('bin:");
  });

  it('keeps one shared adaptive OS polling producer', () => {
    expect(observer).toContain('binObserverCount');
    expect(observer).toContain('BIN_VISIBLE_POLL_MS');
    expect(observer).toContain('BIN_HIDDEN_POLL_MS');
    expect(observer).toContain('ListTrashEntries');
    expect(observer).toContain("origin: 'bin-os-observer'");
  });

  it('routes both app mutations and OS observations through the coalesced bus', () => {
    expect(api).toContain("items: [{ source: 'bin://' }]");
    expect(api).toContain("origin: 'recycle-bin'");
    expect(observer).toContain('this.mutations.publish({');
    expect(observer).toContain('this.mutations.subscribe(');
    expect(bus).toContain('MUTATION_COALESCE_MS');
  });

  it('owns the in-flight refresh gate in the observer, not the pane', () => {
    expect(observer).toContain('record.refreshing');
    expect(observer).toContain('record.pending');
    expect(pane).not.toContain('binMutationRefreshRunning');
    expect(pane).not.toContain('binMutationRefreshPending');
  });
});
