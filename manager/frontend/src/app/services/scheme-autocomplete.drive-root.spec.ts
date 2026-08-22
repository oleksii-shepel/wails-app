import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./scheme-autocomplete.service.ts', import.meta.url), 'utf8');

describe('SchemeAutocompleteService Windows drive-root context', () => {
  it('maps C:/ semantics to canonical filesystem://c/ directory context', () => {
    expect(source).toContain(
      "if (driveRootMatch) return { dirPath: `${q}/`, prefix: '' };",
    );
  });
});
