import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';

const pane = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');

describe('details UI polish', () => {
  it('keeps the header horizontally aligned with body scroll', () => {
    expect(pane).toContain('detailsScrollLeft = Math.max(0, target.scrollLeft)');
    expect(pane).toContain("translateX(' + (-detailsScrollLeft) + 'px)");
  });

  it('uses revision terminology for Bin history', () => {
    expect(pane).toContain('<span>Revision</span>');
    expect(pane).toContain(' revisions');
    expect(pane).toContain('`r${group.length - index}`');
  });
});
