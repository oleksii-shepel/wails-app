import { readFileSync } from 'node:fs';

describe('scroll surface wheel tuning', () => {
  const source = readFileSync(new URL('./input-engine.service.ts', import.meta.url), 'utf8');

  it('gives plain app scroll surfaces a small additive wheel boost', () => {
    expect(source).toContain("const SCROLL_SURFACE_SELECTOR = '.app-scroll-surface';");
    expect(source).toContain('const SCROLL_SURFACE_EXTRA_FACTOR = 0.15;');
    expect(source).toContain('this.boostPlainScrollSurfaceWheel(event);');
    expect(source).toContain('target.scrollTop + deltaY * SCROLL_SURFACE_EXTRA_FACTOR');
  });

  it('keeps modifier-wheel command gestures out of the boost path', () => {
    const boost = source.slice(
      source.indexOf('private boostPlainScrollSurfaceWheel'),
      source.indexOf('private beginMouseDrag'),
    );
    expect(boost).toContain('event.ctrlKey');
    expect(boost).toContain('event.metaKey');
    expect(boost).toContain('event.shiftKey');
    expect(boost).toContain('event.altKey');
    expect(boost).not.toContain('preventDefault');
  });
});
