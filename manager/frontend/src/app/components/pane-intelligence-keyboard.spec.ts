import { readFileSync } from 'node:fs';

describe('Reassembly pane intelligence keyboard contract', () => {
  const manager = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('focuses the first enabled action when the menu opens', () => {
    expect(manager).toContain('this.focusPaneIntelligenceItem(0, 1)');
  });

  it('supports arrow, Home/End, Enter/Space and Escape', () => {
    for (const key of [
      "case 'ArrowDown':",
      "case 'ArrowUp':",
      "case 'Home':",
      "case 'End':",
      "case 'Enter':",
      "case ' ':",
      "case 'Escape':",
    ]) {
      expect(manager).toContain(key);
    }
  });

  it('skips disabled actions during keyboard movement', () => {
    expect(manager).toContain('if (!item.disabled)');
  });

  it('restores focus to the overflow trigger on Escape', () => {
    expect(manager).toContain('this.closePaneIntelligenceMenu(true)');
    expect(manager).toContain('paneIntelligenceMore?.nativeElement.focus');
  });

  it('supports type-ahead search', () => {
    expect(manager).toContain('paneIntelligenceTypeahead');
    expect(manager).toContain('.startsWith(this.paneIntelligenceTypeahead)');
  });
});