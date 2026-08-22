import { readFileSync } from 'node:fs';

const edit = readFileSync(new URL('./address-bar-edit.component.ts', import.meta.url), 'utf8');

describe('address bar forced autocomplete', () => {
  it('makes Ctrl+Space bypass normal suggestion scheduling heuristics', () => {
    expect(edit).toContain("registerCommand('widget.addressBar.edit.showSuggestions', async () =>");
    expect(edit).toContain('await this.forceShowSuggestions()');
    expect(edit).not.toContain(
      "registerCommand('widget.addressBar.edit.showSuggestions', () => {\n      this.tabSession = null;\n      this.scheduleSuggestions",
    );
  });

  it('force-fetches candidates from the current routed source', () => {
    expect(edit).toContain('private async forceShowSuggestions()');
    expect(edit).toContain('const suggestions = await this.fetchChildren(query)');
    expect(edit).toContain('this.setSuggestions(suggestions)');
  });

  it('uses the same forced path for ArrowUp and ArrowDown when closed', () => {
    expect(edit).toContain('if (this.suggestions.length === 0) {\n      await this.forceShowSuggestions();');
  });
});
