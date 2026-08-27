import { readFileSync } from 'node:fs';

describe('Details rows are flush to view edges', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('adds no body inset around the first or last row', () => {
    expect(source).toContain('padding: 0;');
    expect(source).toContain('margin: 0;');
  });

  it('does not reserve transparent row-border space', () => {
    expect(source).toContain('border-bottom: 0;');
  });

  it('keeps the virtual window flush as well', () => {
    expect(source).toContain('.details-window {');
    expect(source).toContain('.details-window.virtual-window {');
  });
});
