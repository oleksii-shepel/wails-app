import { readFileSync } from 'node:fs';

const edit = readFileSync(new URL('./address-bar-edit.component.ts', import.meta.url), 'utf8');

describe('address bar autocomplete source routing', () => {
  it('resolves hierarchical autocomplete through PanelSourceRegistryService', () => {
    expect(edit).toContain('const resolvedSource = this.panelSources.resolve(sourceContext)');
    expect(edit).toContain('await resolvedSource.source.list(sourceContext, false)');
  });

  it('does not use generic scheme autocomplete for contributed hierarchical sources', () => {
    expect(edit).toContain("if (scheme === 'search')");
    expect(edit).toContain('resolvedSource.source.id === \'filesystem\'');
  });

  it('preserves bounded progressive streaming for the local filesystem', () => {
    expect(edit).toContain('startListDirectoryStream');
    expect(edit).toContain('MAX_AUTOCOMPLETE_SUGGESTIONS');
  });

  it('filters routed source results to directory candidates and prefix matches', () => {
    expect(edit).toContain('if (entry.type !== FileType.DIRECTORY) continue;');
    expect(edit).toContain('entry.name.toLowerCase().startsWith(prefixLower)');
  });
});
