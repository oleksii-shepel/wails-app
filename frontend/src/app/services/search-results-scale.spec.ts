import { FileType, type FileInfo } from '../shared/protocol';
import { ResultsPanelService } from './results-panel.service';
import { SearchResultIndex } from './search-result-index';

function entry(index: number): FileInfo {
  return {
    name: `item-${index}.txt`,
    path: `/items/item-${index}.txt`,
    type: FileType.FILE,
    size: index + 1,
    modified: index,
    created: index,
    accessed: index,
    permissions: '',
    isHidden: false,
    isSymlink: false,
  };
}

describe('100k search result panels', () => {
  beforeEach(() => {
    sessionStorage.clear();
    localStorage.clear();
  });

  it('opens a 100k runtime collection through bounded result windows', () => {
    const index = new SearchResultIndex();
    for (let start = 0; start < 100_000; start += 512) {
      index.append(
        Array.from(
          { length: Math.min(512, 100_000 - start) },
          (_, offset) => entry(start + offset),
        ),
      );
    }

    const service = new ResultsPanelService();
    const panel = service.createFromIndex('100k', index, {
      kind: 'search',
      rootPath: '/items',
      query: '*.txt',
      recursive: true,
    });

    expect(panel.entries.length).toBeLessThanOrEqual(2_000);
    expect(panel.metadata.resultCount).toBe(100_000);

    const window = service.readWindow(panel.path, 75_000, 1_024);
    expect(window.entries.length).toBe(1_024);
    expect(window.totalCount).toBe(100_000);
  });

  it('keeps saved query collections regenerable without persisting all results', () => {
    const service = new ResultsPanelService();
    const panel = service.createSearch(
      'Search',
      [entry(0)],
      'C:/Work',
      '*.txt',
      true,
    );

    expect(service.backendSearchAlias(panel.path)).toContain('search://?');
    expect(service.backendSearchAlias(panel.path)).toContain('pattern=');
    expect(service.backendSearchAlias(panel.path)).toContain('recursive=true');
  });
});
