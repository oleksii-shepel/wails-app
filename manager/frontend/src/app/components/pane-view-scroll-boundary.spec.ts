import { readFileSync } from 'node:fs';

describe('shared pane view scroll boundary', () => {
  const globalStyles = readFileSync(new URL('../../styles.scss', import.meta.url), 'utf8');
  const sources = [
    readFileSync(new URL('./pane-list.component.ts', import.meta.url), 'utf8'),
    readFileSync(new URL('./pane-grid.component.ts', import.meta.url), 'utf8'),
    readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8'),
    readFileSync(new URL('./pane-tree.component.ts', import.meta.url), 'utf8'),
  ];

  it('defines overscroll and scroll timing once as the application default', () => {
    expect(globalStyles).toContain('overscroll-behavior: none;');
    expect(globalStyles).toContain('scroll-behavior: auto;');
  });

  it('keeps pane scrolling native and only declares the view-specific pan axis', () => {
    for (const source of sources) {
      expect(source).toContain('touch-action: pan-y;');
      expect(source).not.toContain("addEventListener('wheel'");
    }
  });
});
