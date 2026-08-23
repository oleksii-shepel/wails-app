import { readFileSync } from 'node:fs';

const panes = [
  'directory-pane.component.ts',
  'bin-pane.component.ts',
  'registry-pane.component.ts',
  'autorun-pane.component.ts',
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
});
