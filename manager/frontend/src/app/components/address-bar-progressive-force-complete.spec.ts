import { readFileSync } from 'node:fs';

const edit = readFileSync(new URL('./address-bar-edit.component.ts', import.meta.url), 'utf8');

describe('address bar progressive forced autocomplete', () => {
  it('publishes filesystem autocomplete batches during Ctrl+Space', () => {
    expect(edit).toContain('const publish = (children: PathSuggestion[]) =>');
    expect(edit).toContain('await this.streamDirChildren(dirPath, prefix, publish)');
  });

  it('does not wait for the whole drive listing before opening suggestions', () => {
    const forceStart = edit.indexOf('private async forceShowSuggestions');
    const forceEnd = edit.indexOf('private async moveSuggestionSelectionWithFallback', forceStart);
    const force = edit.slice(forceStart, forceEnd);
    expect(force).not.toContain('const suggestions = await this.fetchChildren(query)');
  });

  it('keeps root aliases for the no-source case', () => {
    expect(edit).toContain('if (!dirPath) {');
    expect(edit).toContain('this.setSuggestions(aliasRoots)');
  });
});
