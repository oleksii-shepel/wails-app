import { readFileSync } from 'node:fs';

describe('DirectoryPane flat parent entry', () => {
  const source = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('projects parent into list, grid and details through the shared flat collection', () => {
    expect(source).toContain('get flatVisibleEntries(): FileInfo[]');
    expect(source).toContain("name: '..'");
    expect(source).toContain("path: '__parent__'");
    expect(source).toContain('? this.flatVisibleEntries.slice(window.start, window.end)');
  });

  it('does not add parent at roots or duplicate provider parent entries', () => {
    expect(source).toContain('this.atRoot');
    expect(source).toContain("this.visibleEntries.some(entry => entry.path === '__parent__')");
  });

  it('includes parent in flat keyboard navigation but excludes it from select-all', () => {
    expect(source).toContain(': this.flatVisibleEntries;');
    expect(source).toContain(".filter(entry => entry.path !== '__parent__')");
  });
});
