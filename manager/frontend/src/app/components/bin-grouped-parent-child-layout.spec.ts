import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin grouped parent/child layout', () => {
  it('does not expose Location as a grouped table header', () => {
    expect(source).toContain(
      '<span></span><span>Name</span><span></span><span>Deleted</span><span>Size</span><span>Revision</span>',
    );
  });

  it('keeps parent location available on group rows', () => {
    expect(source).toContain('class="kind bin-group-location"');
  });

  it('makes child rows shorter than expandable parent rows', () => {
    const selector = '.list.view-details.bin-grouped .entry.bin-revision-row {';
    const start = source.indexOf(selector);
    const end = source.indexOf('}', start);
    const block = source.slice(start, end + 1);

    expect(block).toContain('margin-left: 22px;');
    expect(block).toContain('margin-right: 18px;');
    expect(block).toContain('min-width: calc(100% - 40px);');
  });
});
