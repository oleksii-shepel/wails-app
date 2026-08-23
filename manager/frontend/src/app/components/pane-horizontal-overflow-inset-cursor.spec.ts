import { readFileSync } from 'node:fs';

function read(name: string): string {
  return readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');
}

describe('pane horizontal overflow cursor geometry', () => {
  const directoryFamily = [
    'directory-pane.component.ts',
    'registry-pane.component.ts',
    'autorun-pane.component.ts',
    'network-pane.component.ts',
    'environment-pane.component.ts',
  ];

  it('uses one max-content track and preserves the standard 4px row inset', () => {
    for (const name of directoryFamily) {
      const source = read(name);
      expect(source).toContain('grid-template-columns: minmax(100%, max-content);');
      expect(source).toContain('margin-left: 4px;');
      expect(source).toContain('margin-right: 4px;');
      expect(source).toContain('justify-self: stretch;');
    }
  });

  it('keeps Details headers flush while rows retain their cursor inset', () => {
    for (const name of directoryFamily) {
      const source = read(name);
      expect(source).toContain('.list.view-details > .details-header-viewport > .details-header {');
      expect(source).toContain('margin: 0;');
      expect(source).toContain('padding: 0;');
    }
  });

  it('keeps Bin parent cursors inset and adds hierarchy indentation only to revision rows', () => {
    const source = read('bin-pane.component.ts');
    expect(source).toContain('.list.view-details .list-content-track {');
    expect(source).toContain('grid-template-columns: minmax(100%, max-content);');
    expect(source).toContain('.list.view-details .list-content-track > .entry {');
    expect(source).toContain('margin-left: 4px;');
    expect(source).toContain('margin-right: 4px;');
    expect(source).toContain('.list.view-details.bin-grouped .list-content-track > .entry.bin-revision-row {');
    expect(source).toContain('margin-left: 26px;');
  });

  it('uses the same overflow track/inset contract in System Table', () => {
    const source = read('system-table-pane.component.ts');
    expect(source).toContain('grid-template-columns: minmax(100%, max-content);');
    expect(source).toContain('margin-left: 4px;');
    expect(source).toContain('margin-right: 4px;');
  });
});
