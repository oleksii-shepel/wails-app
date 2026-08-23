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
    const list = files('file-list.component.ts');
    const grid = files('file-grid.component.ts');
    const details = files('file-details.component.ts');

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

  it('always allows parent navigation from double-click handlers', () => {
    expect(files('file-list.component.ts')).toContain(
      'if (this.isParentEntry(entry)) {\n      this.navigate.emit(entry);',
    );
    expect(files('file-grid.component.ts')).toContain(
      'onItemDoubleClick(entry: FileInfo): void {\n    if (this.isParentEntry(entry)) {',
    );
    expect(files('file-details.component.ts')).toContain(
      'onRowDoubleClick(entry: FileInfo): void {\n    if (this.isParentEntry(entry)) {',
    );
  });

  it('keeps synthetic parent out of Directory Tree view', () => {
    expect(files('directory-pane.component.ts')).toContain(
      "return entries.filter(entry => entry.path !== '__parent__')",
    );
  });
});