import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin grouped Details columns', () => {
  it('keeps grouped Name and Path columns compact instead of consuming the viewport', () => {
    expect(source).toContain(
      "return ['28px', 'minmax(160px, 240px)', 'minmax(220px, 360px)', '128px', '82px', '68px'].join(' ');",
    );
  });
});
