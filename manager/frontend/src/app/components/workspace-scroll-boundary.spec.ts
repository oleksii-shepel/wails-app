import { readFileSync } from 'node:fs';

describe('workspace scroll boundary', () => {
  const globalStyles = readFileSync(new URL('../../styles.scss', import.meta.url), 'utf8');
  const sources = [
    readFileSync(new URL('./tool-workspace-host.component.ts', import.meta.url), 'utf8'),
    readFileSync(new URL('./workspace-selector.component.ts', import.meta.url), 'utf8'),
    readFileSync(new URL('./workspace-switcher.component.ts', import.meta.url), 'utf8'),
  ];

  it('inherits the application scroll-boundary default', () => {
    expect(globalStyles).toContain('overscroll-behavior: none;');
    expect(globalStyles).toContain('scroll-behavior: auto;');
  });

  it('keeps workspace scrolling native with a local vertical pan axis', () => {
    for (const source of sources) {
      expect(source).toContain('touch-action: pan-y;');
      expect(source).not.toContain("addEventListener('wheel'");
    }
  });
});
