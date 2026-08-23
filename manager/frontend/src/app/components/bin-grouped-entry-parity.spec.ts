import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin grouped revision entry parity', () => {
  it('uses normal Directory entry child classes for revision rows', () => {
    expect(source).not.toContain('class="icon bin-revision-icon"');
    expect(source).not.toContain('class="modified bin-deleted"');
    expect(source).not.toContain('class="size bin-size"');
    expect(source).not.toContain('class="kind bin-revision"');
    expect(source).not.toContain('bin-grouped-revision-name');
  });

  it('keeps only structural indentation for grouped revision rows', () => {
    const selector = '.list.view-details.bin-grouped .entry.bin-revision-row {';
    const start = source.indexOf(selector);
    const end = source.indexOf('}', start);
    const block = source.slice(start, end + 1);

    expect(block).toContain('margin-left: 22px');
    expect(block).toContain('width: max-content');
    expect(block).toContain('min-width: calc(100% - 22px)');
    expect(block).not.toContain('font-size');
    expect(block).not.toContain('min-height');
    expect(block).not.toContain('opacity');
    expect(block).not.toContain('background');
  });
});
