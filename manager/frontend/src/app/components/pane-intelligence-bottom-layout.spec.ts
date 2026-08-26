import { readFileSync } from 'node:fs';

describe('pane intelligence bottom layout', () => {
  const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

  it('keeps pane intelligence below both panes', () => {
    expect(dual).toContain('grid-template-rows: minmax(0, 1fr) auto;');
    expect(dual).toContain('.pane-intelligence { grid-row: 4; }');
    expect(dual).toContain('class="pane-slot left-pane-slot"');
    expect(dual).toContain('class="pane-slot right-pane-slot"');
  });

  it('opens the bottom-bar overflow menu upward', () => {
    expect(dual).toContain('bottom:calc(100% + 2px);');
  });
});
