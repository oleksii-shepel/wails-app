import { readFileSync } from 'node:fs';

describe('Registry grouping toolbar', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('labels the keys-first toggle as Group', () => {
    expect(pane).toContain('title="Group keys first"');
    expect(pane).toContain('>\n              Group\n');
  });
});
