import { readFileSync } from 'node:fs';

describe('dual-pane hit-test boundaries', () => {
  const source = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

  it('assigns every desktop pane element to an explicit grid column', () => {
    expect(source).toContain('.left-pane-slot {\n      grid-column: 1;');
    expect(source).toContain('.divider {\n      grid-column: 2;');
    expect(source).toContain('.right-pane-slot {\n      grid-column: 3;');
  });

  it('contains custom pane views inside their own hit-test boundary', () => {
    expect(source).toContain('contain: layout paint;');
    expect(source).toContain('isolation: isolate;');
  });

  it('owns pane activation at the slot boundary', () => {
    expect(source).toContain('(pointerdown)="activate(\'left\')"');
    expect(source).toContain('(pointerdown)="activate(\'right\')"');
  });
});
