import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('custom Details column resize contract', () => {
  const environment = read('environment-pane.component.ts');
  const network = read('network-pane.component.ts');
  const groupedBin = read('bin-grouped-view.component.ts');
  const flatBin = read('bin-pane.component.ts');
  const directorySurface = read('directory-list-surface.component.ts');
  const resize = read('pane-details-resize-handle.directive.ts');

  it('puts the shared resize handle on every custom Details header', () => {
    for (const source of [environment, network, groupedBin, flatBin, directorySurface]) {
      expect(source).toContain('PaneDetailsResizeHandleDirective');
      expect(source).toContain('paneDetailsResizeHandle');
      expect(source).toContain('data-pane-details-root');
    }
  });

  it('uses live CSS width variables in custom Details grids', () => {
    expect(environment).toContain('var(--details-column-environment-name-width');
    expect(environment).toContain('var(--details-column-environment-value-width');
    expect(network).toContain('var(--details-column-network-name-width');
    expect(network).toContain('var(--details-column-network-endpoints-width');
    expect(groupedBin).toContain('var(--details-column-bin-name-width');
    expect(groupedBin).toContain('var(--details-column-bin-location-width');
    expect(flatBin).toContain('var(--details-column-bin-name-width');
    expect(directorySurface).toContain('[style.grid-template-columns]="detailsGridTemplate"');
  });

  it('shows resize activity while the shared drag is active', () => {
    expect(resize).toContain("root.setAttribute('data-pane-column-resizing', this.key)");
    expect(resize).toContain("root?.style.setProperty(this.variableName(this.key), `${width}px`)");
    expect(resize).toContain("@HostBinding('class.pane-details-resize-handle--dragging')");
    expect(resize).toContain("this.root?.removeAttribute('data-pane-column-resizing')");
  });
});
