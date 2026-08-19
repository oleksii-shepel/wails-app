import { readFileSync } from 'node:fs';

describe('Registry pane column width persistence', () => {
  const source = readFileSync(
    new URL('./registry-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('does not require Object.fromEntries', () => {
    expect(source).not.toContain('Object.fromEntries');
    expect(source).toContain('const widths: Record<string, number> = {}');
  });
});
