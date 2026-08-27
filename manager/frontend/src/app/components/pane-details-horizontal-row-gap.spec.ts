import { readFileSync } from 'node:fs';

describe('Details horizontal row gap', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('matches List view with a 4px body-row inset', () => {
    expect(source).toContain('margin-left: 4px;');
    expect(source).toContain('margin-right: 4px;');
  });

  it('keeps header column content aligned with inset body rows', () => {
    expect(source).toContain('padding-left: 4px;');
    expect(source).toContain('padding-right: 4px;');
  });

  it('keeps the small inter-column gutter', () => {
    expect(source).toContain('column-gap: 6px;');
  });
});
