import { FileType } from '../shared/protocol';
import { AutorunPanelSource } from './autorun-panel-source.service';

describe('AutorunPanelSource', () => {
  it('exposes user and machine roots', async () => {
    const source = new AutorunPanelSource();
    const listing = await source.list({ path: 'autorun://' });
    expect(listing.entries.map(entry => [entry.name, entry.type])).toEqual([
      ['Current User', FileType.DIRECTORY],
      ['All Users', FileType.DIRECTORY],
    ]);
  });

  it('exposes Run, RunOnce and Startup Folder categories', async () => {
    const source = new AutorunPanelSource();
    const listing = await source.list({ path: 'autorun://user' });
    expect(listing.entries.map(entry => entry.path)).toEqual([
      'autorun://user/registry-run',
      'autorun://user/registry-run-once',
      'autorun://user/startup-folder',
    ]);
  });

  it('round-trips stable entry ids through virtual entry paths', () => {
    const source = new AutorunPanelSource();
    const id = 'registry|user|registry-run|Demo App';
    expect(source.decodeEntryPath(`autorun://entry/${encodeURIComponent(id)}`)).toBe(id);
  });
});
