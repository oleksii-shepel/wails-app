import { readFileSync } from 'node:fs';

describe('Registry type replacement wiring', () => {
  const api = readFileSync(
    new URL('./api.service.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('./plugin-manager.service.ts', import.meta.url),
    'utf8',
  );

  it('uses the typed Registry write binding', () => {
    expect(api).toContain('WriteRegistryValue(');
    expect(manager).toContain('this.api.writeRegistryValue(');
  });

  it('does not reject type changes in the UI', () => {
    expect(manager).not.toContain('requires the typed Registry write path');
  });
});
