import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('BinPane component styles', () => {
  it('keeps styles[0] as one statically analyzable template literal', () => {
    const startToken = 'styles: [`';
    const start = source.indexOf(startToken);
    const end = source.indexOf('`],\n})', start);

    expect(start).toBeGreaterThanOrEqual(0);
    expect(end).toBeGreaterThan(start);

    const css = source.slice(start + startToken.length, end);
    expect(css).not.toContain('`');
  });
});
