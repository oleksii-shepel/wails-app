import { readFileSync } from 'node:fs';

describe('DirectoryPaneComponent header sizing', () => {
  const source = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
  const responsive = readFileSync(new URL('./responsive-toolbar.component.ts', import.meta.url), 'utf8');

  it('keeps the action row at one deterministic height', () => {
    expect(source).toContain('min-height: 34px;');
  });

  it('collapses actions into a dropdown instead of widening or scrolling the pane', () => {
    expect(source).toContain('<app-responsive-toolbar');
    expect(responsive).toContain('overflowActions');
    expect(responsive).toContain('overflow-trigger');
    expect(source).not.toContain('overflow-x: auto;');
  });
});
