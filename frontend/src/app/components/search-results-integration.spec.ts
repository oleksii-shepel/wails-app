import { readFileSync } from 'node:fs';

describe('search results large collection integration', () => {
  const search = readFileSync(
    new URL('./global-search.component.ts', import.meta.url),
    'utf8',
  );
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const discovery = readFileSync(
    new URL('../services/discovery-search.service.ts', import.meta.url),
    'utf8',
  );

  it('streams global discovery batches into an incremental index', () => {
    expect(discovery).toContain('readonly onBatch?');
    expect(search).toContain('this.searchIndex.append(batch');
    expect(search).not.toContain('this.results = [...result.entries]');
  });

  it('uses the Go search stream for query-backed results panels', () => {
    expect(pane).toContain('resultsPanelSearchAlias');
    expect(pane).toContain('this.dirStream.start(');
    expect(pane).toContain('this.backendDisplayPath = requestedPath');
  });

  it('uses windowed result service reads for static saved collections', () => {
    expect(pane).toContain('readResultsPanelWindow');
    expect(pane).toContain("streamId.startsWith('results:')");
  });
});
