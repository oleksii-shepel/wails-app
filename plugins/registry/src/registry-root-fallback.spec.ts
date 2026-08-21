import { readFileSync } from 'node:fs';

const frontend = readFileSync(
  new URL('../frontend/index.js', import.meta.url),
  'utf8',
);

describe('Registry universal root fallback', () => {
  it('keeps the five Windows root hives in the source adapter', () => {
    for (const hive of [
      'HKEY_CLASSES_ROOT',
      'HKEY_CURRENT_USER',
      'HKEY_LOCAL_MACHINE',
      'HKEY_USERS',
      'HKEY_CURRENT_CONFIG',
    ]) {
      expect(frontend).toContain(`'${hive}'`);
    }
  });

  it('keeps presentation out of the universal archive entry', () => {
    expect(frontend).toContain("id: 'registry'");
    expect(frontend).not.toContain('RegistryPaneComponent');
    expect(frontend).not.toContain('@angular/core');
  });
});
