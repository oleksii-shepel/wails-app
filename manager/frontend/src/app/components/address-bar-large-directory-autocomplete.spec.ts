import { readFileSync } from 'node:fs';

const edit = readFileSync(new URL('./address-bar-edit.component.ts', import.meta.url), 'utf8');

describe('address bar autocomplete in large directories', () => {
  it('keeps only directory candidates', () => {
    expect(edit).toContain('if (entry.type !== FileType.DIRECTORY) continue;');
  });

  it('caps retained autocomplete candidates', () => {
    expect(edit).toContain('MAX_AUTOCOMPLETE_SUGGESTIONS = 100');
    expect(edit).toContain('accumulated.length >= this.MAX_AUTOCOMPLETE_SUGGESTIONS');
  });

  it('cancels the backend stream after enough matches are collected', () => {
    expect(edit).toContain('if (accumulated.length >= this.MAX_AUTOCOMPLETE_SUGGESTIONS) {');
    expect(edit).toContain('cancel();');
  });

  it('also bounds non-streaming provider results', () => {
    expect(edit).toContain('.slice(0, this.MAX_AUTOCOMPLETE_SUGGESTIONS)');
  });
});
