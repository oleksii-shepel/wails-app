import { readFileSync } from 'node:fs';

describe('DirectoryPaneComponent header sizing', () => {
  const source = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');

  it('keeps the action row at one deterministic height', () => {
    expect(source).toContain('height: 34px;');
    expect(source).toContain('max-height: 34px;');
    expect(source).toContain('flex-wrap: nowrap;');
    expect(source).toContain('white-space: nowrap;');
  });

  it('preserves access to overflowing actions without growing the pane header', () => {
    expect(source).toContain('overflow-x: auto;');
    expect(source).toContain('overflow-y: hidden;');
  });
});
