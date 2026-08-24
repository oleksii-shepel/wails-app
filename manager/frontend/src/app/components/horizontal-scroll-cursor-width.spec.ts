import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('details-row cursor width', () => {
  it('uses viewport-width Details rows in all Directory-family panes', () => {
    for (const name of [
      'directory-pane.component.ts',
      'registry-pane.component.ts',
      'autorun-pane.component.ts',
      'network-pane.component.ts',
      'environment-pane.component.ts',
    ]) {
      const source = read(name);
      const start = source.indexOf('.list.view-details .entry {');
      const block = source.slice(start, source.indexOf('}', start) + 1);
      expect(block).toContain('width: 100%');
      expect(block).toContain('min-width: 100%');
    }
  });

  it('does not require intrinsic-width FileDetails rows', () => {
    const source = read('pane-details.component.ts');
    expect(source).not.toContain('width: max-content;');
    expect(source).toContain('.details-window.virtual-window');
  });

  it('keeps SystemTable row-width behavior scoped to SystemTable', () => {
    const source = read('system-table-pane.component.ts');
    expect(source).toContain('.row {display:grid; width:max-content;min-width:100%;');
  });
});
