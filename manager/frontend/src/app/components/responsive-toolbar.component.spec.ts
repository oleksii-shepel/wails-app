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
const actionBar = readFileSync(
  new URL('./action-bar.component.ts', import.meta.url),
  'utf8',
);

describe('responsive toolbar convergence', () => {
  it('owns toolbar overflow in one reusable component', () => {
    expect(responsive).toContain("selector: 'app-responsive-toolbar'");
    expect(responsive).toContain('new ResizeObserver(');
    expect(responsive).toContain('visibleActions');
    expect(responsive).toContain('overflowActions');
    expect(responsive).toContain('overflowLabel');
    expect(responsive).toContain('&#x2026;');
  });

  it('measures actual rendered command widths instead of hard-coded pane breakpoints', () => {
    expect(responsive).toContain('#measureAction');
    expect(responsive).toContain('getBoundingClientRect().width');
    expect(responsive).toContain('this.toolbarRef.nativeElement.clientWidth');
    expect(responsive).not.toContain('@media');
  });

  it('uses the shared overflow component in Directory and Registry', () => {
    expect(directory).toContain('<app-responsive-toolbar');
    expect(directory).toContain('[actions]="responsiveToolbarActions"');
    expect(registry).toContain('<app-responsive-toolbar');
    expect(registry).toContain('[actions]="pluginToolbarActions"');
  });

  it('also makes the reusable pane ActionBar delegate command overflow', () => {
    expect(actionBar).toContain('ResponsiveToolbarComponent');
    expect(actionBar).toContain('<app-responsive-toolbar');
    expect(actionBar).toContain('[actions]="actions"');
  });

  it('does not solve pane toolbar overflow with horizontal scrolling', () => {
    const actionBarCss = directory.slice(
      directory.indexOf('.action-bar {'),
      directory.indexOf('.action-bar button {'),
    );
    expect(actionBarCss).toContain('overflow: hidden;');
    expect(actionBarCss).not.toContain('overflow-x: auto;');
  });
});
