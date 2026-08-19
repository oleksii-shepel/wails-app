import { readFileSync } from 'node:fs';

describe('Phase 75 Search & Discovery Excellence', () => {
  const search = readFileSync(
    new URL('./global-search.component.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );
  const discovery = readFileSync(
    new URL('../services/discovery-search.service.ts', import.meta.url),
    'utf8',
  );

  it('unifies search, recent files and collections in discovery', () => {
    expect(search).toContain("type DiscoveryMode = 'search' | 'recent' | 'collections'");
    expect(search).toContain("setMode('recent')");
    expect(search).toContain("setMode('collections')");
  });

  it('supports saved searches with root/provider semantics', () => {
    expect(search).toContain('saveSearch()');
    expect(search).toContain('useSavedSearch(saved');
    expect(search).toContain('currentSearchSpec()');
  });

  it('searches non-filesystem providers through PanelSourceRegistry', () => {
    expect(discovery).toContain('this.panelSources.resolve');
    expect(discovery).toContain('this.panelSources.list(');
    expect(discovery).toContain("resolved.source.kind === 'filesystem'");
  });

  it('opens existing results/plugin collections as pane locations', () => {
    expect(search).toContain('openPanelPath.emit(panel.path)');
    expect(manager).toContain('openDiscoveryPanelPath(path: string)');
    expect(manager).toContain("if (!/^results:\\/\\//i.test(path)) return");
  });

  it('uses rich history instead of the removed query-only array', () => {
    expect(search).not.toContain('recentQueries');
    expect(search).toContain('historyEntries.length>0');
  });

  it('keeps Phase 74 and 73 regressions closed', () => {
    expect(manager).not.toContain("item.status !== 'equal'");
    expect(manager).not.toContain('targetPane.load(peer');
  });
});