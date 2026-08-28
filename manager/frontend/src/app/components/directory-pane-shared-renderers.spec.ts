import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');

describe('Directory Pane canonical renderers', () => {
  it('delegates directory presentation to the shared pane view host', () => {
    expect(source).toContain('<app-pane-view-host');
    expect(source).toContain('[currentView]="viewMode"');
    expect(source).toContain('[treeContentMode]="treeContentMode"');
    expect(source).toContain('[detailsColumns]="paneDetailsColumns"');
  });

  it('does not keep private List/Grid/Tree/Details renderer markup', () => {
    const start = source.indexOf('template: `');
    const end = source.indexOf('\n  `,', start);
    const template = source.slice(start, end);

    expect(template).not.toContain('class="tree-row"');
    expect(template).not.toContain('class="list-viewport"');
    expect(template).not.toContain('<app-pane-details');
  });
});
