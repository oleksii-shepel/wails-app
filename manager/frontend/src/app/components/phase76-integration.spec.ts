import { readFileSync } from 'node:fs';

describe('Phase 76 smart results integration', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const smart = readFileSync(
    new URL('../services/smart-results.service.ts', import.meta.url),
    'utf8',
  );
  const pluginApi = readFileSync(
    new URL('../plugin/plugin-registry-api.ts', import.meta.url),
    'utf8',
  );

  it('keeps the active pane outer border removed', () => {
    expect(pane).not.toContain('.pane.active-pane');
  });

  it('routes normal pane refresh through smart collection refresh', () => {
    expect(pane).toContain('if (this.activeResultsPanel)');
    expect(pane).toContain('await this.refreshResultsCollection()');
  });

  it('shows collection controls only for results locations', () => {
    expect(pane).toContain('*ngIf="activeResultsPanel as panel"');
    expect(pane).toContain('refreshResultsCollection()');
    expect(pane).toContain('toggleResultsCollectionPin()');
    expect(pane).toContain('renameResultsCollection()');
  });

  it('refreshes built-in search, recent, and duplicate collections', () => {
    expect(smart).toContain("origin === 'global-search'");
    expect(smart).toContain("origin === 'recent-files'");
    expect(smart).toContain("origin === 'duplicate-finder'");
  });

  it('allows plugin collections to provide live refresh callbacks', () => {
    expect(pluginApi).toContain(
      'refresh?: () => Promise<readonly FileInfo[]>',
    );
  });

  it('preserves cursor and selection by resource identity after refresh', () => {
    expect(pane).toContain('const previousCursor = this.cursorPath');
    expect(pane).toContain('const previousSelection = new Set(this.selectedPaths)');
    expect(pane).toContain('existing.has(previousCursor)');
  });
});