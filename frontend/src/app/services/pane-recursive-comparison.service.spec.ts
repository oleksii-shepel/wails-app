import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { FileType, type DirectoryListing, type FileInfo } from '../shared/protocol';
import { PaneComparisonService } from './pane-comparison.service';
import { PaneRecursiveComparisonService } from './pane-recursive-comparison.service';
import type { ApiService } from './api.service';

function entry(path: string, type = FileType.FILE, size = 1, modified = 1, isSymlink = false): FileInfo {
  return { name: path.split('/').pop()!, path, type, size, modified, created: modified, accessed: modified, permissions: '', isHidden: false, isSymlink };
}
function listing(path: string, entries: FileInfo[]): DirectoryListing { return { path, entries, totalSize: 0 }; }

describe('PaneRecursiveComparisonService', () => {
  it('descends matched directories and compares by relative path', async () => {
    const dirs = new Map<string, DirectoryListing>([
      ['L', listing('L', [entry('L/src', FileType.DIRECTORY)])],
      ['R', listing('R', [entry('R/src', FileType.DIRECTORY)])],
      ['L/src', listing('L/src', [entry('L/src/app.ts', FileType.FILE, 10, 5)])],
      ['R/src', listing('R/src', [entry('R/src/app.ts', FileType.FILE, 10, 1)])],
    ]);
    const api = { listDirectory: fn(async (path: string) => dirs.get(path)!) } as unknown as ApiService;
    const result = await new PaneRecursiveComparisonService(new PaneComparisonService()).compare(api, 'L', 'R');
    expect(result.comparison.items.find(item => item.key === 'src/app.ts')?.status).toBe('left-newer');
  });

  it('collapses a missing directory subtree into one copyable item', async () => {
    const api = { listDirectory: fn(async (path: string) => {
      if (path === 'L') return listing('L', [entry('L/photos', FileType.DIRECTORY)]);
      if (path === 'R') return listing('R', []);
      throw new Error('missing subtree must not be scanned');
    }) } as unknown as ApiService;
    const result = await new PaneRecursiveComparisonService(new PaneComparisonService()).compare(api, 'L', 'R');
    expect(result.comparison.items.map(item => [item.key, item.status])).toEqual([['photos', 'only-left']]);
    expect(api.listDirectory).toHaveBeenCalledTimes(2);
  });

  it('does not follow symlink directories', async () => {
    const api = { listDirectory: fn(async (path: string) => listing(path, [entry(`${path}/link`, FileType.DIRECTORY, 0, 0, true)])) } as unknown as ApiService;
    await new PaneRecursiveComparisonService(new PaneComparisonService()).compare(api, 'L', 'R');
    expect(api.listDirectory).toHaveBeenCalledTimes(2);
  });

  it('marks results truncated when the item limit is reached', async () => {
    const many = Array.from({ length: 120 }, (_, i) => entry(`L/f${i}.txt`));
    const api = { listDirectory: fn(async (path: string) => path === 'L' ? listing('L', many) : listing('R', [])) } as unknown as ApiService;
    const result = await new PaneRecursiveComparisonService(new PaneComparisonService()).compare(api, 'L', 'R', { maxItems: 100 });
    expect(result.truncated).toBe(true);
    expect((result.comparison.items).length).toBe(100);
  });
});