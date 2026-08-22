import { readFileSync } from 'node:fs';

const edit = readFileSync(new URL('./address-bar-edit.component.ts', import.meta.url), 'utf8');

describe('address bar source-scoped autocomplete', () => {
  it('stops mixing root aliases after a concrete source/root is selected', () => {
    expect(edit).toContain('if (this.hasSelectedAutocompleteRoot(rawQuery))');
    expect(edit).toContain("/^[a-z][a-z0-9+.-]*:\\/\\//i.test(value)");
    expect(edit).toContain("/^[a-z]:\\//i.test(value)");
  });

  it('keeps arrow navigation functional even when no dropdown suggestions are loaded yet', () => {
    expect(edit).toContain('moveSuggestionSelectionWithFallback');
    expect(edit).toContain('const suggestions = await this.fetchChildren(this.value)');
    expect(edit).toContain('this.setSuggestions(suggestions)');
  });

  it('wraps arrow navigation through available autocomplete candidates', () => {
    expect(edit).toContain("this.acIdx = delta > 0 ? 0 : this.suggestions.length - 1");
    expect(edit).toContain('(this.acIdx + delta + this.suggestions.length) % this.suggestions.length');
  });
});
