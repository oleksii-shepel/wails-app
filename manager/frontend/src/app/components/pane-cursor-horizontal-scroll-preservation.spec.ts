import { readFileSync } from 'node:fs';

// Panes that still own their list/tree markup keep the preservation inline.
// Panes migrated to the shared pane view host (directory, autorun) delegate
// cursor scrolling to the view components, where pane-list owns it now.
const panes = [
  'bin-pane.component.ts',
  'registry-pane.component.ts',
  'network-pane.component.ts',
  'environment-pane.component.ts',
];

describe('cursor visibility preserves horizontal row inset', () => {
  for (const pane of panes) {
    it(`${pane} does not horizontally scroll a wide cursor into view`, () => {
      const source = readFileSync(new URL(`./${pane}`, import.meta.url), 'utf8');
      expect(source).toContain('const scrollLeft = host.scrollLeft;');
      expect(source).toContain("cursorElement.scrollIntoView({ block: 'nearest', inline: 'nearest' });");
      expect(source).toContain('host.scrollLeft = scrollLeft;');
    });
  }

  for (const pane of ['directory-pane.component.ts', 'autorun-pane.component.ts']) {
    it(`${pane} delegates cursor scrolling to the shared view host`, () => {
      const source = readFileSync(new URL(`./${pane}`, import.meta.url), 'utf8');
      expect(source).toContain('this.paneViewHost?.scrollToCursorPath(this.cursorPath);');
    });
  }

  it('pane-list does not horizontally scroll a wide cursor into view', () => {
    const source = readFileSync(new URL('./pane-list.component.ts', import.meta.url), 'utf8');
    expect(source).toContain('const scrollLeft = host.scrollLeft;');
    expect(source).toContain("el.scrollIntoView({ block: 'nearest', inline: 'nearest' });");
    expect(source).toContain('host.scrollLeft = scrollLeft;');
  });
});
