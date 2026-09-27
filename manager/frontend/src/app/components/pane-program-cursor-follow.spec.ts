import { readFileSync } from 'node:fs';

const read = (name: string) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('Program cursor following across pane views', () => {
  const list = read('pane-list.component.ts');
  const grid = read('pane-grid.component.ts');

  it('makes List follow cursorPath changes from the pane controller', () => {
    expect(list).toContain('private cursorFollowRAF: number | null = null;');
    expect(list).toContain('private scheduleCursorFollow(): void {');
    expect(list).toContain("if (changes['cursorPath'])");
    expect(list).toContain('this.scrollToPath(path);');
  });

  it('makes Grid follow cursorPath changes from the pane controller', () => {
    expect(grid).toContain('private cursorFollowRAF: number | null = null;');
    expect(grid).toContain('private scheduleCursorFollow(): void {');
    expect(grid).toContain("if (changes['cursorPath'])");
    expect(grid).toContain('this.scrollToPath(path);');
  });

  it('can follow the parent pseudo-entry instead of rejecting it', () => {
    expect(list).toContain('if (path === PARENT_ENTRY_PATH) {');
    expect(list).not.toContain('if (!path || path === PARENT_ENTRY_PATH) return;');
    expect(grid).not.toContain('if (!path || path === PARENT_ENTRY_PATH) return;');
  });
});
