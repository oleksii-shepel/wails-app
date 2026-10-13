import { readFileSync } from 'node:fs';

const files = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('parent entry double-click navigation parity', () => {
  it('does not open parent from Directory-family selectFromPointer', () => {
    for (const name of [
      'directory-pane.component.ts',
      'environment-pane.component.ts',
      'autorun-pane.component.ts',
      'network-pane.component.ts',
      'registry-pane.component.ts',
    ]) {
      const source = files(name);
      expect(source).toContain("if (entry.path === '__parent__')");
      expect(source).not.toContain(
        "if (entry.path === '__parent__') {\n      this.openEntry(entry);",
      );
      expect(source).toContain("this.lastAction = 'select: parent'");
    }
  });

  it('does not navigate parent from shared List/Grid/Details click handlers', () => {
    const list = files('pane-list.component.ts');
    const grid = files('pane-grid.component.ts');
    const details = files('pane-details.component.ts');

    expect(list).not.toContain(
      'this.selectionChange.emit([entry]);\n      this.navigate.emit(entry);\n      return;',
    );
    expect(grid).not.toContain(
      'this.selectionChange.emit([entry]);\n        this.navigate.emit(entry);\n        return;',
    );
    expect(details).not.toContain(
      'this.selectionChange.emit([entry]);\n      this.navigate.emit(entry);\n      return;',
    );
  });

  it('selects the directory being exited after parent navigation', () => {
    for (const name of [
      'directory-pane.component.ts',
      'environment-pane.component.ts',
      'autorun-pane.component.ts',
      'network-pane.component.ts',
      'registry-pane.component.ts',
      'bin-pane.component.ts',
    ]) {
      const source = files(name);
      expect(source).toContain('selectPath: this.path');
      expect(source).toContain('const previousCursor = options.selectPath ??');
    }
  });

  it('always allows parent navigation from double-click handlers', () => {
    expect(files('pane-list.component.ts')).toContain(
      'if (this.isParentEntry(entry)) {\n      this.navigate.emit(entry);',
    );
    expect(files('pane-grid.component.ts')).toContain(
      'onItemDoubleClick(entry: FileInfo): void {\n    if (this.isParentEntry(entry)) {',
    );
    expect(files('pane-details.component.ts')).toContain(
      'onRowDoubleClick(entry: FileInfo): void {\n    if (this.isParentEntry(entry)) {',
    );
  });

  it('keeps synthetic parent out of Directory Tree view', () => {
    expect(files('directory-pane.component.ts')).toContain(
      "return entries.filter(entry => entry.path !== '__parent__')",
    );
  });
});
