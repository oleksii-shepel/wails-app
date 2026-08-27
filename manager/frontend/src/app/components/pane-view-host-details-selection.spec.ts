import { readFileSync } from 'node:fs';

describe('PaneViewHost Details selection refresh', () => {
  const source = readFileSync(new URL('./pane-view-host.component.ts', import.meta.url), 'utf8');

  it('marks the OnPush host after the view controller changes selection', () => {
    expect(source).toContain('thisRef.selectedFiles = new Set(paths);\n        thisRef.cdr.markForCheck();');
  });

  it('marks the OnPush host after the view controller changes cursor', () => {
    expect(source).toContain('thisRef.cursorPath = path;\n        thisRef.cdr.markForCheck();');
  });
});
