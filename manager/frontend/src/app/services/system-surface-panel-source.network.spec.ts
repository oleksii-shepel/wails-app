import { FileType } from '../shared/protocol';
import { SystemSurfacePanelSource } from './system-surface-panel-source';

describe('Network SystemSurfacePanelSource hierarchy', () => {
  const descriptor = {
    kind: 'network' as const,
    title: 'Network',
    columns: [],
  };

  afterEach(() => {
    delete (window as any).go;
  });

  it('exposes a file-like root of browsable categories', async () => {
    const source = new SystemSurfacePanelSource(descriptor);
    const listing = await source.list({ path: 'network://' }, false);

    expect(listing.location.path).toBe('network://');
    expect(listing.entries.map(entry => entry.name)).toEqual([
      'Computers',
      'Remote locations',
      'Network adapters',
      'Active connections',
    ]);
    expect(listing.entries.every(entry => entry.type === FileType.DIRECTORY)).toBe(true);
  });

  it('filters native network diagnostics into adapters and connections', async () => {
    (window as any).go = { app: { App: {
      SystemSurfaceList: jasmine.createSpy().and.resolveTo([
        { id: 'a', name: 'Ethernet', fields: { kind: 'Adapter', status: 'Up' } },
        { id: 't', name: 'TCP 127.0.0.1:80', fields: { kind: 'TCP', status: 'Listen' } },
        { id: 'u', name: 'UDP 0.0.0.0:53', fields: { kind: 'UDP', status: 'Listening' } },
      ]),
    } } };

    const source = new SystemSurfacePanelSource(descriptor);
    const adapters = await source.list({ path: 'network://adapters' }, false);
    const connections = await source.list({ path: 'network://connections' }, false);

    expect(adapters.entries.map(entry => entry.name)).toEqual(['Ethernet']);
    expect(connections.entries.map(entry => entry.name)).toEqual([
      'TCP 127.0.0.1:80',
      'UDP 0.0.0.0:53',
    ]);
  });

  it('discovers computers and maps them as directories', async () => {
    (window as any).go = { app: { App: {
      NetworkListComputers: jasmine.createSpy().and.resolveTo([
        { id: 'nas', name: 'NAS', fields: { kind: 'Computer' } },
      ]),
    } } };

    const source = new SystemSurfacePanelSource(descriptor);
    const listing = await source.list({ path: 'network://computers' }, false);

    expect(listing.entries[0].type).toBe(FileType.DIRECTORY);
    expect(listing.entries[0].path).toBe('network://computers/NAS');
  });

  it('maps computer shares to ordinary UNC directories', async () => {
    (window as any).go = { app: { App: {
      NetworkListShares: jasmine.createSpy().and.resolveTo([
        {
          id: 'nas/media',
          name: 'Media',
          fields: { kind: 'Share', remote: '\\\\NAS\\Media' },
        },
      ]),
    } } };

    const source = new SystemSurfacePanelSource(descriptor);
    const listing = await source.list({ path: 'network://computers/NAS' }, false);

    expect(listing.entries[0].type).toBe(FileType.DIRECTORY);
    expect(listing.entries[0].path).toBe('//NAS/Media');
  });

  it('has hierarchical parent paths', () => {
    const source = new SystemSurfacePanelSource(descriptor);
    expect(source.getParentPath({ path: 'network://connections' })).toBe('network://');
    expect(source.getParentPath({ path: 'network://computers/NAS' }))
      .toBe('network://computers');
  });
});
