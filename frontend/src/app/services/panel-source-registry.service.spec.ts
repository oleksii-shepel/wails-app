import { FileType } from '../shared/protocol';
import type { FileInfo } from '../shared/protocol-enhanced';
import { PanelSourceRegistryService, RegistryPanelSource } from './panel-source-registry.service';

function entry(path: string): FileInfo {
  const name = path.split('/').pop() || path;
  return {
    name,
    path,
    type: FileType.FILE,
    size: 1,
    modified: 1,
    created: 1,
    accessed: 1,
    permissions: '',
    isHidden: false,
    isSymlink: false,
  };
}

describe('PanelSourceRegistryService', () => {
  function createService() {
    const directories = {
      listDirectory: jasmine.createSpy('listDirectory').and.callFake(async (path: string) => ({
        path,
        entries: [entry(`${path}/local.txt`)],
      })),
      getRootPath: jasmine.createSpy('getRootPath').and.returnValue('filesystem://c'),
      getParentPath: jasmine.createSpy('getParentPath').and.returnValue('filesystem://c'),
      joinChildPath: jasmine.createSpy('joinChildPath').and.callFake((path: string, name: string) => `${path}/${name}`),
      supportsStreaming: jasmine.createSpy('supportsStreaming').and.returnValue(true),
      getPathKind: jasmine.createSpy('getPathKind').and.returnValue('filesystem'),
    };
    const remoteConnections = {
      isRemoteUri: jasmine.createSpy('isRemoteUri').and.returnValue(false),
      capabilitiesFor: jasmine.createSpy('capabilitiesFor'),
    };
    const resultsPanels = {
      get: jasmine.createSpy('get').and.returnValue(null),
      create: jasmine.createSpy('create'),
      title: jasmine.createSpy('title'),
    };

    return {
      service: new PanelSourceRegistryService(
        directories as any,
        remoteConnections as any,
        resultsPanels as any,
      ),
      directories,
    };
  }

  it('treats the local filesystem as a normal panel source', async () => {
    const { service, directories } = createService();
    const context = {
      path: 'filesystem://c/work',
      entries: [] as FileInfo[],
      inArchive: false,
    };

    const resolved = service.resolve(context);
    const listing = await service.listContext(context, false);

    expect(resolved.source.id).toBe('filesystem');
    expect(resolved.source.kind).toBe('filesystem');
    expect(resolved.location.sourceId).toBe('filesystem.local');
    expect(directories.listDirectory).toHaveBeenCalledWith('filesystem://c/work', false);
    expect(listing.entries.map(item => item.name)).toEqual(['local.txt']);
  });

  it('treats an open archive as its own panel source without falling through to filesystem listing', async () => {
    const { service, directories } = createService();
    const entries = [entry('docs/readme.txt')];
    const context = {
      path: 'filesystem://c/work/package.zip/docs',
      entries,
      inArchive: true,
      archiveTopLevel: true,
    };

    const resolved = service.resolve(context);
    const listing = await service.listContext(context, false);

    expect(resolved.source.id).toBe('archive');
    expect(resolved.source.kind).toBe('archive');
    expect(resolved.capabilities.create).toBe(false);
    expect(resolved.capabilities.rename).toBe(false);
    expect(listing.location.sourceId).toBe('archive');
    expect(listing.entries).toEqual(entries);
    expect(listing.entries).not.toBe(entries);
    expect(directories.listDirectory).not.toHaveBeenCalled();
  });

  it('keeps archive mutation policy compatible with top-level versus nested archives', () => {
    const { service } = createService();

    const topLevel = service.resolve({
      path: 'filesystem://c/work/package.zip',
      inArchive: true,
      archiveTopLevel: true,
    });
    const nested = service.resolve({
      path: 'filesystem://c/work/package.zip/nested.zip',
      inArchive: true,
      archiveTopLevel: false,
    });

    expect(topLevel.capabilities.delete).toBe(true);
    expect(nested.capabilities.delete).toBe(false);
  });
  it('resolves contributed panel sources before ordinary built-ins and removes them cleanly', () => {
    const { service } = createService();
    const source = {
      id: 'plugin.demo',
      kind: 'virtual' as const,
      title: 'Demo',
      canHandle: (context: any) => context.path.startsWith('demo://'),
      capabilities: () => ({ read: true, create: false, delete: false, rename: false, trash: false }),
      list: async (context: any) => ({ location: { path: context.path, sourceId: 'plugin.demo', sourceKind: 'virtual' as const }, entries: [] }),
      getRootPath: () => 'demo://',
      getParentPath: () => 'demo://',
      joinChildPath: (_context: any, name: string) => `demo://${name}`,
      supportsStreaming: () => false,
    };

    service.register(source);
    expect(service.resolve({ path: 'demo://collection' }).source.id).toBe('plugin.demo');
    expect(service.listSources().some(candidate => candidate.id === 'plugin.demo')).toBe(true);

    service.unregister('plugin.demo');
    expect(service.listSources().some(candidate => candidate.id === 'plugin.demo')).toBe(false);
  });

  it('keeps Registry out of core resolution until the Registry plugin contributes it', () => {
    const { service, directories } = createService();
    (directories.getPathKind as jasmine.Spy).and.returnValue('registry');

    expect(service.resolve({ path: 'registry://HKEY_CURRENT_USER/Software' }).source.id).toBe('filesystem');

    service.register(new RegistryPanelSource(directories as any));
    const resolved = service.resolve({ path: 'registry://HKEY_CURRENT_USER/Software' });
    expect(resolved.source.id).toBe('registry');
    expect(resolved.source.kind).toBe('registry');
  });

});
