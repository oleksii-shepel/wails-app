import { describe, expect, it, vi } from 'vitest';
import { DuplicateFinderService } from './duplicate-finder.service';
import { FileType } from '../shared/protocol';

function f(path: string, size: number, modified = 1) {
  return { name: path.split('/').pop()!, path, type: FileType.FILE, size, modified, created: 0, accessed: 0, permissions: '', isHidden: false };
}
function d(path: string) {
  return { ...f(path, 0), type: FileType.DIRECTORY };
}

describe('DuplicateFinderService', () => {
  it('hashes only size-collision candidates and groups verified matches', async () => {
    const api = {
      listDirectory: vi.fn(async (path: string) => path === '/root'
        ? { entries: [f('/root/a.txt', 10), f('/root/b.txt', 10), f('/root/c.txt', 20), d('/root/nested')] }
        : { entries: [f('/root/nested/d.txt', 10)] }),
      hashFile: vi.fn(async (path: string) => path.endsWith('b.txt') ? 'other' : 'same'),
    };
    const result = await new DuplicateFinderService(api as any).scan('/root');
    expect(api.hashFile).toHaveBeenCalledTimes(3);
    expect(result.groups).toHaveLength(1);
    expect(result.groups[0].files.map(x => x.path)).toEqual(['/root/a.txt', '/root/nested/d.txt']);
    expect(result.reclaimableBytes).toBe(10);
  });

  it('does not descend through directory symlinks', async () => {
    const linked = { ...d('/root/link'), isSymlink: true };
    const api = { listDirectory: vi.fn(async () => ({ entries: [linked] })), hashFile: vi.fn() };
    await new DuplicateFinderService(api as any).scan('/root');
    expect(api.listDirectory).toHaveBeenCalledTimes(1);
  });

  it('returns suggested redundant paths while preserving one keeper', () => {
    const service = new DuplicateFinderService({} as any);
    const group = { hash: 'x', size: 3, files: [f('/a', 3), f('/b', 3), f('/c', 3)], reclaimableBytes: 6, suggestedKeeperPath: '/a' };
    expect(service.suggestedRedundantPaths([group])).toEqual(['/b', '/c']);
  });
});

describe('DuplicateFinderService keeper policies', () => {
  const service = new DuplicateFinderService({} as any);
  const group = {
    hash: 'same', size: 10, reclaimableBytes: 20,
    files: [f('/very/long/path/a.txt', 10, 2), f('/b.txt', 10, 3), f('/mid/c.txt', 10, 1)],
    suggestedKeeperPath: '/mid/c.txt',
  };

  it('supports oldest, newest and shortest-path keepers', () => {
    expect(service.keeperPath(group, 'oldest')).toBe('/mid/c.txt');
    expect(service.keeperPath(group, 'newest')).toBe('/b.txt');
    expect(service.keeperPath(group, 'shortest-path')).toBe('/b.txt');
  });

  it('builds redundant selections from the chosen policy', () => {
    expect(service.redundantPaths([group], 'newest')).toEqual(['/very/long/path/a.txt', '/mid/c.txt']);
  });
});
