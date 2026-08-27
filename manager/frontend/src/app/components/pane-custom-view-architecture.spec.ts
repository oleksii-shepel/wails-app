import { readFileSync } from 'node:fs';

describe('pane custom view architecture', () => {
  const host = readFileSync(new URL('./pane-view-host.component.ts', import.meta.url), 'utf8');
  const bin = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');
  const grouped = readFileSync(new URL('./bin-grouped-view.component.ts', import.meta.url), 'utf8');

  it('keeps the four standard pane views in the shared host', () => {
    for (const selector of ['app-pane-list', 'app-pane-grid', 'app-pane-tree', 'app-pane-details']) {
      expect(host).toContain(selector);
    }
  });

  it('models Recycle Bin grouping as an explicit custom view', () => {
    expect(bin).toContain('BinGroupedViewComponent');
    expect(bin).toContain('<app-bin-grouped-view');
    expect(grouped).toContain("selector: 'app-bin-grouped-view'");
    expect(grouped).toContain('binGroupHeader');
    expect(grouped).toContain('expandedGroups');
  });

  it('does not mount the legacy flat renderer behind grouped mode', () => {
    expect(bin).toContain("!(isBinLocation && binDetailsMode === 'grouped')");
  });
});
