import { readFileSync } from 'node:fs';

const responsive = readFileSync(
  new URL('./responsive-toolbar.component.ts', import.meta.url),
  'utf8',
);
const directory = readFileSync(
  new URL('./directory-pane.component.ts', import.meta.url),
  'utf8',
);
const registry = readFileSync(
  new URL('./registry-pane.component.ts', import.meta.url),
  'utf8',
);
const dirSurface = readFileSync(
  new URL('./dir-surface.component.ts', import.meta.url),
  'utf8',
);
const fileView = readFileSync(
  new URL('./file-view-container.component.ts', import.meta.url),
  'utf8',
);

describe('fully collapsible toolbar architecture', () => {
  it('allows every toolbar item to collapse into overflow', () => {
    expect(responsive).toContain('@Input() minVisible = 0');
    expect(directory).toContain('[minVisible]="0"');
    expect(registry).toContain('[minVisible]="0"');
    expect(dirSurface).toContain('[minVisible]="0"');
  });

  it('models view and arrange controls as responsive commands', () => {
    for (const source of [directory, dirSurface]) {
      expect(source).toContain("label: 'List'");
      expect(source).toContain("label: 'Grid'");
      expect(source).toContain("label: 'Details'");
      expect(source).toContain("label: 'Tree'");
      expect(source).toContain("label: 'Sort: Name'");
      expect(source).toContain("label: 'Folders first'");
      expect(source).not.toContain('<app-pane-view-controls');
      expect(source).not.toContain('<app-pane-arrange-controls');
    }
  });

  it('removes stale ActionBar and PaneArrange dependencies', () => {
    for (const source of [directory, registry, dirSurface]) {
      expect(source).not.toContain('ActionBarComponent');
      expect(source).not.toContain('PaneArrangeControlsComponent');
      expect(source).not.toContain('PaneViewControlsComponent');
    }
    expect(fileView).not.toContain("./action-bar.component");
  });

  it('preserves active state in both visible and overflow renderings', () => {
    expect(responsive).toContain('[class.active]="action.active"');
    expect(responsive).toContain("readonly active?: boolean");
  });
});
