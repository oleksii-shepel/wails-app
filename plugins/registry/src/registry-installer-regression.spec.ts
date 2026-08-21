import { readFileSync } from 'node:fs';

describe('Registry sibling installer regression', () => {
  const source = readFileSync(
    new URL('./registry-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('keeps root hive paths as valid TypeScript source', () => {
    expect(source).toContain("path: 'registry://' + name");
    expect(source).not.toContain('path: egistry://,');
  });
});