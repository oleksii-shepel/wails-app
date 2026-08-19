import { readFileSync } from 'node:fs';

describe('Directory parent entry presentation', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('shows the synthetic parent in every non-tree Directory view', () => {
    expect(pane).toContain(
      "return this.isDirectoryTab && this.viewMode !== 'tree' && !this.atRoot",
    );
    expect(pane).toContain("path: '__parent__'");
    expect(pane).toContain("name: '..'");
  });

  it('explicitly removes the synthetic parent from Tree', () => {
    expect(pane).toContain(
      "return entries.filter(entry => entry.path !== '__parent__')",
    );
  });

  it('keeps the parent visible while filtering', () => {
    expect(pane).toContain("entry.path === '__parent__' ||");
  });

  it('accounts for parent index in backend-virtualized directories', () => {
    expect(pane).toContain(
      'this.backendVisibleCount + (this.showPresentationParent ? 1 : 0)',
    );
    expect(pane).toContain(
      'const sourceStart = Math.max(0, window.start - parentOffset)',
    );
  });
});
