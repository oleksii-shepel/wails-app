import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('entry-name trimming across pane views', () => {
  it('clips names in every pane implementation', () => {
    for (const file of [
      'directory-pane.component.ts',
      'registry-pane.component.ts',
      'autorun-pane.component.ts',
      'network-pane.component.ts',
      'environment-pane.component.ts',
      'bin-pane.component.ts',
    ]) {
      const source = read(file);
      expect(source).toContain('.list.view-details .entry .name');
      expect(source).toContain('.list.view-grid .entry .name');
      expect(source).toContain('text-overflow: ellipsis;');
    }
  });

  it('clips shared list/grid file-entry names', () => {
    const source = read('file-entry.component.ts');
    expect(source).toContain('.file-name-text');
    expect(source).toContain('min-width: 0;');
    expect(source).toContain('text-overflow: ellipsis;');
  });

  it('clips tree names', () => {
    expect(read('file-entry-inline.component.ts')).toContain('text-overflow: ellipsis;');
    expect(read('pane-tree.component.ts')).toContain('.tree-entry');
  });

  it('clips details cells', () => {
    expect(read('pane-details.component.ts')).toContain('.details-cell .file-name');
  });
});
