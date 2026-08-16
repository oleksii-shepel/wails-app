import { ResultsPanelService } from './results-panel.service';
import { FileType } from '../shared/protocol';

function entry(path: string) {
  const name = path.split('/').pop() || path;
  return { name, path, type: FileType.FILE, size: 1, modified: 1, created: 1, accessed: 1, permissions: '', isHidden: false, isSymlink: false };
}

describe('ResultsPanelService producers', () => {
  beforeEach(() => sessionStorage.clear());

  it('creates search, recent, duplicate and plugin collections on one results scheme', () => {
    const service = new ResultsPanelService();
    const file = entry('/work/a.txt');

    const search = service.createSearch('Search: a', [file], '/work', 'a', true);
    const recent = service.createRecentFiles([file]);
    const duplicates = service.createDuplicateResults('/work', [file]);
    const plugin = service.createPluginCollection('demo', 'reports', 'Reports', [file]);

    expect([search.path, recent.path, duplicates.path, plugin.path].every(path => path.startsWith('results://'))).toBe(true);
    expect(search.metadata.kind).toBe('recursive-search');
    expect(recent.metadata.kind).toBe('recent-files');
    expect(duplicates.metadata.kind).toBe('duplicates');
    expect(plugin.metadata).toEqual(jasmine.objectContaining({ pluginId: 'demo', collectionId: 'reports', kind: 'plugin-collection' }));
  });

  it('removes all collections owned by one plugin only', () => {
    const service = new ResultsPanelService();
    const a = service.createPluginCollection('a', 'one', 'One', []);
    const b = service.createPluginCollection('b', 'two', 'Two', []);

    service.removePluginCollections('a');

    expect(service.get(a.path)).toBeNull();
    expect(service.get(b.path)).not.toBeNull();
  });
});
