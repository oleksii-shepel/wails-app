import '@angular/compiler';
import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { PathHistoryService } from './path-history.service';
import { FileType } from '../shared/protocol';

class StorageStub {
  private values = new Map<string, string>();
  getItem(key: string) { return this.values.get(key) ?? null; }
  setItem(key: string, value: string) { this.values.set(key, value); }
  removeItem(key: string) { this.values.delete(key); }
  clear() { this.values.clear(); }
}

describe('PathHistoryService location history', () => {
  beforeEach(() => {
    stubGlobal('localStorage', new StorageStub());
  });
  afterEach(restoreGlobals);

  it('stores scheme and kind for search history', () => {
    const service = new PathHistoryService({ listDirectory: fn() } as any);
    service.addLocation('search://?path=filesystem://c/Work&pattern=invoice', 'left');
    expect(service.getRecentLocations(1)[0]).toEqual(jasmine.objectContaining({
      scheme: 'search',
      kind: 'query',
      paneId: 'left',
    }));
  });

  it('migrates legacy path-only storage to location-aware entries', () => {
    localStorage.setItem('filemanager:path-history', JSON.stringify([{
      path: 's3://archive/reports', timestamp: 10, accessCount: 3, paneId: 'right',
    }]));
    const service = new PathHistoryService({ listDirectory: fn() } as any);
    expect(service.getAllHistory()[0]).toEqual(jasmine.objectContaining({
      location: 's3://archive/reports', scheme: 's3', kind: 'location', accessCount: 3,
    }));
    expect(JSON.parse(localStorage.getItem('filemanager:path-history')!)[0].location).toBe('s3://archive/reports');
  });

  it('returns recent and frequent locations without filesystem assumptions', () => {
    const service = new PathHistoryService({ listDirectory: fn() } as any);
    service.addLocation('smb://server/share', 'left');
    service.addLocation('smb://server/share', 'left');
    service.addLocation('search://?pattern=todo', 'right');
    expect(service.getFrequentLocations(5)[0]).toEqual(jasmine.objectContaining({ scheme: 'smb', accessCount: 2 }));
    expect(service.getRecentLocations(5).some(entry => entry.scheme === 'search')).toBe(true);
  });

  it('preserves distinct case-sensitive locations', () => {
    const service = new PathHistoryService({ listDirectory: fn() } as any);
    service.addLocation('/srv/Reports', 'left');
    service.addLocation('/srv/reports', 'left');
    expect(service.getAllHistory()).toHaveSize(2);
  });

  it('keeps valid entries when stored history contains malformed rows', () => {
    localStorage.setItem('filemanager:path-history', JSON.stringify([
      null,
      { path: true },
      { path: '/valid', timestamp: -1, accessCount: -2 },
    ]));
    const service = new PathHistoryService({ listDirectory: fn() } as any);
    expect(service.getAllHistory()).toHaveSize(1);
    expect(service.getAllHistory()[0]).toEqual(jasmine.objectContaining({ location: '/valid', accessCount: 1 }));
    expect(service.getAllHistory()[0].timestamp).toBeGreaterThan(0);
  });

  it('does not expose mutable references to stored history', () => {
    const service = new PathHistoryService({ listDirectory: fn() } as any);
    service.addLocation('/original', 'left');
    service.getAllHistory()[0].location = '/changed';
    expect(service.getAllHistory()[0].location).toBe('/original');
  });

  it('resolves relative directory suggestions against the current pane', async () => {
    const listDirectory = jasmine.createSpy('listDirectory').and.resolveTo({
      entries: [{ type: FileType.DIRECTORY, name: 'reports', path: '/workspace/reports' }],
    });
    const service = new PathHistoryService({ listDirectory } as any);

    const suggestions = await service.getSuggestions('../', '/workspace/current');

    expect(listDirectory).toHaveBeenCalledOnceWith('/workspace', false);
    expect(suggestions[0].path).toBe('/workspace/reports');
  });

  it('does not resolve local relative paths against opaque providers', async () => {
    const listDirectory = jasmine.createSpy('listDirectory');
    const service = new PathHistoryService({ listDirectory } as any);

    await service.getSuggestions('./repo', 's3://archive');

    expect(listDirectory).not.toHaveBeenCalled();
  });
});
