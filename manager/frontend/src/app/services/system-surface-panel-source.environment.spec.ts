import { FileType } from '../shared/protocol';
import { SystemSurfacePanelSource } from './system-surface-panel-source';

describe('Environment SystemSurfacePanelSource hierarchy', () => {
  const descriptor = {
    kind: 'environment' as const,
    title: 'Environment Variables',
    columns: [],
  };

  afterEach(() => {
    delete (window as any).go;
  });

  it('exposes Process, User and System as file-like scope directories', async () => {
    const source = new SystemSurfacePanelSource(descriptor);
    const listing = await source.list({ path: 'environment://' }, false);

    expect(listing.entries.map(entry => entry.name)).toEqual([
      'Process variables',
      'User variables',
      'System variables',
    ]);
    expect(listing.entries.every(entry => entry.type === FileType.DIRECTORY)).toBe(true);
  });

  it('filters variables by scope when a scope is entered', async () => {
    (window as any).go = { app: { App: {
      SystemSurfaceList: jasmine.createSpy().and.resolveTo([
        { id: 'process:PATH', name: 'PATH', fields: { scope: 'Process', value: 'P' } },
        { id: 'user:PATH', name: 'PATH', fields: { scope: 'User', value: 'U' } },
        { id: 'system:PATH', name: 'PATH', fields: { scope: 'System', value: 'S' } },
      ]),
    } } };

    const source = new SystemSurfacePanelSource(descriptor);

    const process = await source.list({ path: 'environment://process' }, false);
    const user = await source.list({ path: 'environment://user' }, false);
    const system = await source.list({ path: 'environment://system' }, false);

    expect(process.entries.map(entry => entry.systemFields['value'])).toEqual(['P']);
    expect(user.entries.map(entry => entry.systemFields['value'])).toEqual(['U']);
    expect(system.entries.map(entry => entry.systemFields['value'])).toEqual(['S']);
  });

  it('returns the Environment root as parent for every scope', () => {
    const source = new SystemSurfacePanelSource(descriptor);
    expect(source.getParentPath({ path: 'environment://user' })).toBe('environment://');
    expect(source.getParentPath({ path: 'environment://system' })).toBe('environment://');
  });
});
