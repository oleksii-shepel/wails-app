import { FileType } from '../shared/protocol';
import { BinPanelSource } from './bin-panel-source.service';

describe('BinPanelSource', () => {
  it('projects system trash entries into bin:// rows', async () => {
    const api = {
      listTrashEntries: async () => [{ id: 'native-id', name: 'report.txt', originalPath: 'C:/Work/report.txt', trashedPath: 'native', deletedAt: '2026-08-22T10:00:00Z', size: 42, isDir: false, canRestore: true }],
    } as any;
    const source = new BinPanelSource(api);

    const listing = await source.list({ path: 'bin://' });

    expect(listing.location.path).toBe('bin://');
    expect(listing.entries.length).toBe(1);
    expect(listing.entries[0].type).toBe(FileType.FILE);
    expect(listing.entries[0].path.startsWith('bin://entry/')).toBeTrue();
    expect((listing.entries[0] as any).binOriginalPath).toBe('C:/Work/report.txt');
  });

  it('round-trips native trash identities through synthetic paths', () => {
    const source = new BinPanelSource({} as any);
    const id = 'C:/\u0024Recycle.Bin/item with spaces';
    const path = `bin://entry/${encodeURIComponent(id)}`;
    expect(source.decodeEntryPath(path)).toBe(id);
  });
});
