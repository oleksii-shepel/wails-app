import { readFileSync } from 'node:fs';

const address = readFileSync(
  new URL('./address-bar.component.ts', import.meta.url),
  'utf8',
);

describe('address bar implicit root resolution', () => {
  it('resolves bare source names through the address root helper before normalizing input', () => {
    expect(address).toContain('const resolvedCandidate = this.resolveImplicitAddressRoot(candidate) ?? candidate;');
    expect(address).toContain('const resolvedRaw = this.resolveImplicitAddressRoot(raw) ?? raw;');
    expect(address).toContain('private resolveImplicitAddressRoot(value: string): string | null {');
  });

  it('maps single-letter drive roots to drive references instead of relative paths', () => {
    expect(address).toContain("if (/^[a-z]$/i.test(normalized)) {");
    expect(address).toContain("return `${normalized.toUpperCase()}:`;");
  });

  it('maps built-in pane source names to their scheme roots', () => {
    expect(address).toContain('if (normalized === schemeName || normalized === displayName) {');
    expect(address).toContain('return root.path;');
  });
});
