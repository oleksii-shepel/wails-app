import { readFileSync } from 'node:fs';

describe('Phase 79 Performance & Scale Excellence', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const search = readFileSync(
    new URL('./global-search.component.ts', import.meta.url),
    'utf8',
  );

  it('virtualizes large list/grid/detail collections', () => {
    expect(pane).toContain('get listVirtualWindow()');
    expect(pane).toContain('get renderedEntries()');
    expect(pane).toContain('*ngFor="let entry of renderedEntries');
    expect(pane).toContain('list-virtual-spacer');
  });

  it('virtualizes large expanded tree projections', () => {
    expect(pane).toContain('get treeVirtualWindow()');
    expect(pane).toContain('get renderedTreeRows()');
  });

  it('caches sorting/filter projection between change detection passes', () => {
    expect(pane).toContain('projectionEntriesRef');
    expect(pane).toContain('projectionCache');
  });

  it('scrolls virtualized cursor items into the rendered window first', () => {
    expect(pane).toContain('index < virtual.start || index >= virtual.end');
    expect(pane).toContain('this.updateVirtualViewport(viewport)');
  });

  it('bounds global-search DOM result count', () => {
    expect(search).toContain('get renderedSearchResults()');
    expect(search).toContain('start + 160');
  });

  it('keeps the outer pane border removed', () => {
    expect(pane).not.toContain('.pane.active-pane');
  });
});
