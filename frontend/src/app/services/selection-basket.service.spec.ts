import '@angular/compiler';
import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { FileType } from '../shared/protocol';
import { SelectionBasketService } from './selection-basket.service';

function makeStorage() {
  const data = new Map<string, string>();
  return {
    getItem: fn((key: string) => data.get(key) ?? null),
    setItem: fn((key: string, value: string) => data.set(key, value)),
    removeItem: fn((key: string) => data.delete(key)),
    clear: fn(() => data.clear()),
  };
}

function makeService() {
  const api = {
    getFileInfo: fn(async (path: string) => ({
      name: path.split('/').pop() || path,
      path,
      type: FileType.FILE,
      size: 12,
      modified: 10,
    })),
  };
  return { service: new SelectionBasketService(api as never), api };
}

describe('SelectionBasketService', () => {
  beforeEach(() => {
    stubGlobal('sessionStorage', makeStorage());
  });

  it('deduplicates canonical paths while preserving insertion order', () => {
    const { service } = makeService();
    expect(service.add(['filesystem://c/work/a.txt', 'filesystem://c/work/a.txt'], 'pane:left')).toBe(1);
    expect(service.add(['filesystem://c/work/b.txt'], 'search')).toBe(1);
    expect(service.paths).toEqual(['filesystem://c/work/a.txt', 'filesystem://c/work/b.txt']);
  });

  it('removes successful paths without clearing failed basket entries', () => {
    const { service } = makeService();
    service.add(['filesystem://c/a.txt', 'filesystem://c/b.txt']);
    expect(service.removePaths(['filesystem://c/a.txt'])).toBe(1);
    expect(service.paths).toEqual(['filesystem://c/b.txt']);
  });

  it('persists only durable basket identity to session storage', () => {
    const { service } = makeService();
    service.add(['s3://bucket/report.csv'], 'search-results');
    const stored = JSON.parse(sessionStorage.getItem('filemanager:selection-basket-v1') ?? '[]');
    expect(stored).toHaveLength(1);
    expect(stored[0]).toMatchObject({ path: 's3://bucket/report.csv', source: 'search-results' });
    expect(stored[0].size).toBeUndefined();
  });
});

