import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./registry-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Registry details-only presentation', () => {
  it('locks Registry to Details view', () => {
    expect(source).toContain("readonly viewMode: DirectoryView = 'details';");
    expect(source).toContain("lastAction = 'view: details';");
  });

  it('does not expose Directory list/grid/tree switches', () => {
    expect(source).not.toContain("setViewMode('list')");
    expect(source).not.toContain("setViewMode('grid')");
    expect(source).not.toContain("setViewMode('tree')");
    expect(source).not.toContain('class="view-switcher"');
  });

  it('cannot be replaced by a generic panel presentation', () => {
    expect(source).toContain('get activePanelPresentation(): PanelPresentationContribution | null');
    expect(source).toContain('Registry owns a fixed Details renderer.');
    expect(source).not.toContain("viewMode: 'details',\n    });\n  }\n\n  get panelPresentationModel");
  });

  it('keeps the Registry Details columns', () => {
    expect(source).toContain(
      '<span></span><span>Name</span><span>Kind</span><span>Type</span><span>Value</span>',
    );
    expect(source).toContain('{{ entryDisplayType(entry) }}');
    expect(source).toContain('{{ entryDisplayValue(entry) }}');
  });
});