import { readFileSync } from 'node:fs';

const files = [
  './directory-pane.component.ts',
  './pane-view-host.component.ts',
];

describe('retired toolbar dependencies', () => {
  for (const file of files) {
    it(`${file} does not depend on retired toolbar components`, () => {
      const source = readFileSync(new URL(file, import.meta.url), 'utf8');
      expect(source).not.toContain('./action-bar.component');
      expect(source).not.toContain('./pane-view-controls.component');
      expect(source).not.toContain('./pane-arrange-controls.component');
      expect(source).not.toContain('ActionBarComponent');
      expect(source).not.toContain('PaneViewControlsComponent');
      expect(source).not.toContain('PaneArrangeControlsComponent');
    });
  }

  it('Directory imports the responsive command type it uses', () => {
    const source = readFileSync(
      new URL('./directory-pane.component.ts', import.meta.url),
      'utf8',
    );
    expect(source).toContain(
      "import { ResponsiveToolbarComponent, type ResponsiveToolbarCommand } from './responsive-toolbar.component';",
    );
  });
});
