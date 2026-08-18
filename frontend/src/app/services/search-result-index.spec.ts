import { FileType, type FileInfo } from '../shared/protocol';
import { SearchResultIndex } from './search-result-index';

function entry(index: number, size = index + 1): FileInfo {
  return {
    name: `file-${String(index).padStart(6, '0')}.txt`,
    path: `/root/file-${String(index).padStart(6, '0')}.txt`,
    type: FileType.FILE,
    size,
    modified: index,
    created: index,
    accessed: index,
    permissions: '',
    isHidden: false,
    isSymlink: false,
  };
}

describe('SearchResultIndex', () => {
  it('keeps 100k results chunked and reads bounded windows', () => {
    const index = new SearchResultIndex();

    for (let start = 0; start < 100_000; start += 512) {
      index.append(
        Array.from(
          { length: Math.min(512, 100_000 - start) },
          (_, offset) => entry(start + offset),
        ),
      );
    }

    expect(index.resultCount).toBe(100_000);
    const window = index.window(50_000, 1_024);
    expect(window.entries.length).toBe(1_024);
    expect(window.totalCount).toBe(100_000);
  });

  it('progressively sorts independent runs without rebuilding one full array', () => {
    const index = new SearchResultIndex();
    index.setSort('name');
    index.append([entry(9), entry(7), entry(8)]);
    index.append([entry(2), entry(0), entry(1)]);

    const names = index.window(0, 6).entries.map(value => value.name);
    expect(names).toEqual([
      'file-000000.txt',
      'file-000001.txt',
      'file-000002.txt',
      'file-000007.txt',
      'file-000008.txt',
      'file-000009.txt',
    ]);
  });

  it('filters incrementally and indexes stable path identity', () => {
    const index = new SearchResultIndex();
    index.setFilter(value => value.name.endsWith('2.txt'));
    index.append([entry(0), entry(1), entry(2)]);
    index.append([entry(12), entry(13)]);

    expect(index.resultCount).toBe(2);
    expect(index.window(0, 10).entries.map(value => value.path)).toEqual([
      '/root/file-000002.txt',
      '/root/file-000012.txt',
    ]);
  });

  it('groups duplicate candidates without content hashing on the presentation path', () => {
    const index = new SearchResultIndex();
    index.append([
      entry(1, 4096),
      { ...entry(2, 4096), name: 'other.txt' },
      { ...entry(3, 4096), name: 'other.bin' },
    ]);

    const groups = index.duplicateCandidateGroups();
    expect(groups.some(group => group.size === 4096 && group.count === 2)).toBeTrue();
  });
});
