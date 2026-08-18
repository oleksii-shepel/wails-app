import { readFileSync } from 'node:fs';

describe('shared VirtualCollection integration', () => {
  const files = [
    './file-list.component.ts',
    './file-details.component.ts',
    './file-grid.component.ts',
    './global-search.component.ts',
    './directory-pane.component.ts',
  ].map(path => readFileSync(new URL(path, import.meta.url), 'utf8'));

  it('uses the same VirtualCollection engine in list, details, grid, search and pane orchestration', () => {
    for (const source of files) {
      expect(source).toContain('VirtualCollection');
    }
  });

  it('does not retain the old hard-coded global search 160-item slicing implementation', () => {
    const search = files[3];
    expect(search).not.toContain('entries.slice(start, start + 160)');
    expect(search).toContain('resultVirtualCollection.renderedItems()');
  });

  it('uses shared backend prefetch and loaded guard logic', () => {
    const pane = files[4];
    expect(pane).toContain('this.virtualCollection.prefetch()');
    expect(pane).toContain('this.virtualCollection.isRenderCoveredByLoadedGuard()');
  });
});
