import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin name trimming in all views', () => {
  it('uses one clipping contract for list/details/grid/tree names', () => {
    expect(source).toContain('.list.view-list .entry .name');
    expect(source).toContain('.list.view-details .entry .name');
    expect(source).toContain('.list.view-grid .entry .name');
    expect(source).toContain('.tree-row .name');
    expect(source).toContain('text-overflow: ellipsis;');
    expect(source).toContain('white-space: nowrap;');
  });

  it('clips grouped parent metadata with the same contract', () => {
    expect(source).toContain('.bin-group-filename');
    expect(source).toContain('.bin-group-location');
    expect(source).toContain('min-width: 0;');
    expect(source).toContain('max-width: 100%;');
  });

  it('keeps the fix local to BinPane', () => {
    expect(source).toContain('Bin name trimming contract');
  });
});
