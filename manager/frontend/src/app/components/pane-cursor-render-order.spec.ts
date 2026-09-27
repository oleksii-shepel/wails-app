import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('pane cursor follows rendered order', () => {
  const panes = [
    'environment-pane.component.ts',
    'network-pane.component.ts',
    'autorun-pane.component.ts',
    'registry-pane.component.ts',
    'bin-pane.component.ts',
    'directory-pane.component.ts',
  ];

  it('uses one selectable sequence for Arrow cursor movement in every pane', () => {
    for (const name of panes) {
      const source = read(name);
      const start = source.indexOf('moveCursor(delta: -1 | 1): void');
      const end = source.indexOf('private setSingleSelection', start);
      const body = source.slice(start, end);

      expect(start).toBeGreaterThanOrEqual(0);
      expect(body).toContain('const visible = this.selectableEntries();');
      expect(body).not.toContain("this.viewMode === 'tree'\n      ? this.treeRows");
    }
  });

  it('uses PaneViewHost render order in every host-backed pane', () => {
    for (const name of [
      'autorun-pane.component.ts',
      'registry-pane.component.ts',
      'directory-pane.component.ts',
    ]) {
      const source = read(name);
      const start = source.indexOf('private selectableEntries(): FileInfo[]');
      const end = source.indexOf('private syncCursorFromSelection', start);
      const body = source.slice(start, end);

      expect(body).toContain("this.paneViewHost?.getSelectableEntries('__parent__',");
    }

    // Full-view panes must not gate the shared host behind a tree exclusion —
    // the tree cursor moves through the rendered hierarchy too.
    for (const name of ['autorun-pane.component.ts', 'directory-pane.component.ts']) {
      const source = read(name);
      const start = source.indexOf('private selectableEntries(): FileInfo[]');
      const end = source.indexOf('private syncCursorFromSelection', start);
      const body = source.slice(start, end);

      expect(body).toContain("this.paneViewHost?.getSelectableEntries('__parent__', this.viewMode !== 'tree')");
      expect(body).not.toContain('if (this.viewMode !== \'tree\') {');
    }
  });

  it('keeps the parent row as the first cursor step', () => {
    const host = read('file-view-runtime.controller.ts');
    expect(host).toContain(
      "return (host.sortedEntries ?? []).filter((entry) => includeParent || entry.path !== parentEntryPath);",
    );

    const provider = read('panel-source-pane-session.ts');
    expect(provider).toContain('const current = entries.findIndex');
    expect(provider).toContain("? (delta < 0 ? entries.length - 1 : 0)");
    expect(provider).not.toContain('Math.max(0, entries.findIndex');
  });
});
