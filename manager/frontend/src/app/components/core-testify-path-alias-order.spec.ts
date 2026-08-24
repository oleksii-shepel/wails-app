import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

describe('Testify @reassembly/core path alias ordering', () => {
  const root = resolve(import.meta.dirname, '../../../../..');

  for (const rel of [
    'manager/frontend/tsconfig.json',
    'plugins/registry/tsconfig.json',
    'plugins/registry/tsconfig.lib.json',
  ]) {
    it(`keeps the bare core alias after every public subpath in ${rel}`, () => {
      const config = JSON.parse(readFileSync(resolve(root, rel), 'utf8'));
      const keys = Object.keys(config.compilerOptions?.paths ?? {});
      const rootIndex = keys.indexOf('@reassembly/core');

      expect(rootIndex).toBeGreaterThan(-1);
      for (const key of keys) {
        if (!key.startsWith('@reassembly/core/')) continue;
        expect(keys.indexOf(key)).toBeLessThan(rootIndex);
      }
    });
  }
});
