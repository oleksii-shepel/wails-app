import { readdirSync, readFileSync } from 'node:fs';
import { basename, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const rendererFiles = [
  'pane-list.component.ts',
  'pane-grid.component.ts',
  'pane-tree.component.ts',
  'pane-details.component.ts',
] as const;

describe('pane renderer architecture', () => {
  it('has exactly one shared renderer for each collection presentation', () => {
    const files = readdirSync(here);
    for (const name of rendererFiles) expect(files).toContain(name);

    expect(files).not.toContain('file-list.component.ts');
    expect(files).not.toContain('file-grid.component.ts');
    expect(files).not.toContain('file-tree.component.ts');
    expect(files).not.toContain('file-details.component.ts');
  });

  it('uses only pane-* selectors and class names for shared renderers', () => {
    const expected = [
      ['pane-list.component.ts', 'PaneListComponent', "selector: 'app-pane-list'"],
      ['pane-grid.component.ts', 'PaneGridComponent', "selector: 'app-pane-grid'"],
      ['pane-tree.component.ts', 'PaneTreeComponent', "selector: 'app-pane-tree'"],
      ['pane-details.component.ts', 'PaneDetailsComponent', "selector: 'app-pane-details'"],
    ] as const;

    for (const [file, className, selector] of expected) {
      const source = readFileSync(join(here, file), 'utf8');
      expect(source).toContain(`export class ${className}`);
      expect(source).toContain(selector);
    }
  });

  it('contains no references to the retired File renderer classes/selectors', () => {
    const forbidden = [
      'FileListComponent', 'FileGridComponent', 'FileTreeComponent', 'FileDetailsComponent',
      'app-file-list', 'app-file-grid', 'app-file-tree', 'app-file-details',
      'file-list.component', 'file-grid.component', 'file-tree.component', 'file-details.component',
    ];

    for (const name of readdirSync(here).filter(name => /\.(?:ts|html|scss)$/.test(name))) {
      if (basename(name) === 'pane-renderer-architecture.spec.ts') continue;
      const source = readFileSync(join(here, name), 'utf8');
      for (const token of forbidden) expect(source).not.toContain(token);
    }
  });
});
