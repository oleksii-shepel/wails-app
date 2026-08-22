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

  it('projects backend autorun records into directory-pane entries with metadata', async () => {
    const source = new AutorunPanelSource();
    spyOn(source, 'records').and.resolveTo([{
      id: 'registry|user|registry-run|Demo App',
      name: 'Demo App',
      command: '"C:\\Demo\\demo.exe" --background',
      executable: 'C:\\Demo\\demo.exe',
      sourcePath: 'HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run',
      scope: 'user',
      origin: 'registry-run',
      enabled: true,
    }]);

    const listing = await source.list({ path: 'autorun://user/registry-run' });
    expect(listing.entries.length).toBe(1);

    const entry = listing.entries[0] as any;
    expect(entry.name).toBe('Demo App');
    expect(entry.autorunCommand).toContain('--background');
    expect(entry.autorunExecutable).toBe('C:\\Demo\\demo.exe');
    expect(entry.autorunSourcePath).toContain('CurrentVersion');
    expect(entry.autorunScope).toBe('user');
    expect(entry.autorunOrigin).toBe('registry-run');
    expect(entry.autorunEnabled).toBeTrue();
  });
});