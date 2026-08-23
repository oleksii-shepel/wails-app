import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin horizontal overflow cursor surface', () => {
  it('uses one intrinsic-width content track for header and every row', () => {
    expect(source).toContain('class="list-content-track"');
    expect(source).toContain('.list.view-details .list-content-track {');
    expect(source).toContain('grid-template-columns: minmax(100%, max-content);');
    expect(source).toContain('width: max-content;');
    expect(source).toContain('min-width: 100%;');
  });

  it('stretches cursor rows to the shared track instead of sizing each row independently', () => {
    const start = source.indexOf('.list.view-list .entry,\n    .list.view-details .entry,');
    const end = source.indexOf('}', start);
    const block = source.slice(start, end + 1);

    expect(start).toBeGreaterThanOrEqual(0);
    expect(block).toContain('width: auto;');
    expect(block).toContain('min-width: 0;');
    expect(block).toContain('justify-self: stretch;');
    expect(block).not.toContain('width: 100%;');
  });

  it('keeps the Details header on the same track as rows', () => {
    const listOpen = source.indexOf('class="list"');
    const track = source.indexOf('class="list-content-track"', listOpen);
    const header = source.indexOf('class="details-header-viewport"', track);
    const entry = source.indexOf('class="entry"', header);
    const trackClose = source.indexOf('<div class="empty"', entry);

    expect(track).toBeGreaterThan(listOpen);
    expect(header).toBeGreaterThan(track);
    expect(entry).toBeGreaterThan(header);
    expect(trackClose).toBeGreaterThan(entry);
  });
});
