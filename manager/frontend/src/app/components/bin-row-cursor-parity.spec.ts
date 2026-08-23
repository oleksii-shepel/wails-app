import { readFileSync } from 'node:fs';

const bin = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');
const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');

describe('Bin row/cursor geometry parity', () => {
  it('does not define a custom Bin row height', () => {
    expect(bin).not.toContain('.bin-revision-row {\n      min-height:');
    expect(bin).not.toContain('.bin-group-header {\n      min-height:');
  });

  it('does not define custom Bin selection/cursor geometry', () => {
    expect(bin).not.toContain('.bin-revision-row.selected {');
    expect(bin).not.toContain('.bin-group-header.cursor {');
    expect(bin).not.toContain('.bin-group-header.selected {');
  });

  it('keeps only grouped indentation as Bin-specific row geometry', () => {
    expect(bin).toContain('.list.view-details.bin-grouped .entry.bin-revision-row {');
    expect(bin).toContain('margin-left: 22px;');
    expect(bin).toContain('min-width: calc(100% - 22px);');
  });

  it('keeps horizontal scroll width without changing cursor style', () => {
    expect(bin).toContain('.list.view-details .entry {\n      width: max-content;\n      min-width: 100%;');
    expect(directory).toContain('.entry {');
  });
});
