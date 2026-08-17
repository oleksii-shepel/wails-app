import { readFileSync } from 'node:fs';

describe('Reassembly tab interaction contract', () => {
  const source = readFileSync(
    new URL('./reassembly-directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('supports middle-click closing without activating the tab first', () => {
    expect(source).toContain('(auxclick)="onTabAuxClick(tab, $event)"');
    expect(source).toContain("if (event.button !== 1) return;");
  });

  it('keeps pinned tabs protected from middle-click close', () => {
    expect(source).toContain('if (!tab.isClosable || tab.isPinned)');
  });

  it('creates a tab from empty-strip double click', () => {
    expect(source).toContain('(dblclick)="onTabStripDoubleClick($event)"');
    expect(source).toContain("target?.closest('.tab, .new-tab, button')");
  });

  it('scrolls overflow tabs horizontally with the mouse wheel', () => {
    expect(source).toContain('(wheel)="onTabStripWheel($event)"');
    expect(source).toContain('strip.scrollLeft += delta');
  });
});
