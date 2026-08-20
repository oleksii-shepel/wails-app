import { readFileSync } from 'node:fs';

describe('Reassembly pane compare rendering', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('does not render comparison dots or badges inside pane rows', () => {
    expect(pane).not.toContain('class="compare-badge"');
    expect(pane).not.toContain('.compare-badge');
  });

  it('keeps comparison logic available without row decoration', () => {
    expect(pane).toContain('compareOverlay');
  });
});