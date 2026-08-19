import { readFileSync } from 'node:fs';

describe('Reassembly compare visualization', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('does not draw compare state as structural row edges', () => {
    expect(pane).not.toContain('box-shadow:inset 2px 0 0');
    expect(pane).not.toContain('[class.compare-only-here]');
  });

  it('uses compact status badges instead', () => {
    expect(pane).toContain('class="compare-badge"');
    expect(pane).toContain('.compare-badge.only-here');
    expect(pane).toContain('.compare-badge.different');
  });

  it('hides compare markers during ordinary browsing', () => {
    expect(pane).toContain('if (!this.compareOverlayVisible) return null');
    expect(manager).toContain(
      '[compareOverlayVisible]="comparisonVisible || syncReviewVisible"',
    );
  });
});