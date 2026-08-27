import { readFileSync } from 'node:fs';

describe('Details initial column arrangement', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('uses concrete fixed widths on first render instead of late CSS variables', () => {
    expect(source).toContain('return `${Math.max(1, configured)}px`;');
    expect(source).not.toContain('return `var(--details-column-${this.cssKey(key)}-width)`');
  });

  it('rebuilds column VMs whenever column inputs/configuration change', () => {
    expect(source).toContain('this.refreshColumnState();\n        this.rebuildColumnVms();');
  });

  it('keeps header and rows on the same grid', () => {
    expect(source.match(/\[style\.grid-template-columns\]="columnGridTemplate"/g)?.length).toBe(2);
  });

  it('has a small inter-column gap without an outer row inset', () => {
    expect(source).toContain('column-gap: 6px;');
    expect(source).toMatch(/\.details-row\s*\{[^}]*padding:\s*0;/s);
  });
});
