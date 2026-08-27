import { readFileSync } from 'node:fs';

describe('inactive pane selection vs cursor', () => {
  const paneNames = [
    'directory-pane.component.ts',
    'bin-pane.component.ts',
    'registry-pane.component.ts',
    'autorun-pane.component.ts',
  ];

  for (const name of paneNames) {
    it(`${name}: keeps selection visible but hides inactive cursor`, () => {
      const source = readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

      expect(source).toContain('.pane:not(.active-pane) .entry.selected');
      expect(source).toContain('--vsc-list-inactive-selection');
      expect(source).not.toContain('.pane:not(.active-pane) .entry.cursor {');
      expect(source).not.toContain('.pane:not(.active-pane) .tree-row.cursor {');
    });
  }

  it('Details keeps inactive selection but no inactive cursor decoration', () => {
    const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

    expect(source)
      .toContain(':host-context(.pane:not(.active-pane)) .details-row.selected');
    expect(source).toContain('--vsc-list-inactive-selection');
    expect(source)
      .not.toContain(':host-context(.pane:not(.active-pane)) .details-row.cursor {');
  });
});
