import { readFileSync } from 'node:fs';

describe('workspace scroll boundary policy', () => {
  const globalStyles = readFileSync(new URL('../../styles.scss', import.meta.url), 'utf8');

  it('uses the shared explicit scroll-surface contract', () => {
    expect(globalStyles).toContain(':where(.app-scroll-surface)');
    expect(globalStyles).toContain('overscroll-behavior: none;');
    expect(globalStyles).toContain('scroll-behavior: auto;');
  });

  it('marks workspace-owned scrolling surfaces without marking hidden pane wrappers', () => {
    const host = readFileSync(new URL('./tool-workspace-host.component.ts', import.meta.url), 'utf8');
    expect(host).toContain('class="search-surface app-scroll-surface"');
    expect(host).toContain('class="compare-surface app-scroll-surface"');
    expect(host).toContain('class="summary-surface app-scroll-surface"');
    expect(host).not.toContain('details-tree-surface tree-view app-scroll-surface');
    expect(host).not.toContain('details-surface details-inspector list view-details app-scroll-surface');
  });

  for (const component of [
    './workspace-switcher.component.ts',
    './workspace-selector.component.ts',
  ]) {
    it(`${component} marks its scrollable list`, () => {
      const source = readFileSync(new URL(component, import.meta.url), 'utf8');
      expect(source).toContain('app-scroll-surface');
      expect(source).toContain('touch-action: pan-y;');
    });
  }
});
