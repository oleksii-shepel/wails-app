import { readFileSync } from 'node:fs';

describe('pane view scroll boundary policy', () => {
  const globalStyles = readFileSync(new URL('../../styles.scss', import.meta.url), 'utf8');
  const components = [
    './pane-list.component.ts',
    './pane-grid.component.ts',
    './pane-details.component.ts',
    './pane-tree.component.ts',
  ];

  it('defines the boundary policy only on explicit scroll owners', () => {
    expect(globalStyles).toContain(':where(.app-scroll-surface)');
    expect(globalStyles).toContain('overscroll-behavior: none;');
    expect(globalStyles).toContain('scroll-behavior: auto;');
    expect(globalStyles).not.toMatch(/\*\s*\{[^}]*overscroll-behavior:\s*none/s);
  });

  for (const component of components) {
    it(`${component} marks its actual scroll container`, () => {
      const source = readFileSync(new URL(component, import.meta.url), 'utf8');
      expect(source).toContain('app-scroll-surface');
      expect(source).toContain('touch-action: pan-y;');
    });
  }
});
