import { SystemSurfacePanelSource } from './system-surface-panel-source';

describe('SystemSurfacePanelSource', () => {
  const descriptor = {
    kind: 'processes' as const,
    title: 'Processes',
    columns: [{ id: 'pid', label: 'PID', field: 'pid' }],
  };

  afterEach(() => {
    delete (window as any).go;
  });

  it('maps backend records into ordinary synthetic file rows', async () => {
    (window as any).go = { app: { App: {
      SystemSurfaceList: jasmine.createSpy().and.resolveTo([
        { id: '42', name: 'worker', fields: { pid: 42, workingSet: 8192 } },
      ]),
    } } };
    const source = new SystemSurfacePanelSource(descriptor);

    const listing = await source.list({ path: 'processes://' }, false);

    expect(listing.location.sourceId).toBe('builtin.system.processes');
    expect(listing.entries.length).toBe(1);
    expect(listing.entries[0].name).toBe('worker');
    expect(listing.entries[0].path).toBe('processes://entry/42');
    expect(listing.entries[0].size).toBe(8192);
    expect((listing.entries[0] as any).systemFields.pid).toBe(42);
  });

  it('never routes synthetic leaves into the file viewer', () => {
    const source = new SystemSurfacePanelSource(descriptor);
    expect(source.canPreviewEntry?.({ path: 'processes://' }, {} as any)).toBe(false);
    expect(source.activateEntry?.({ path: 'processes://' }, {} as any)).toBe(true);
  });
});
