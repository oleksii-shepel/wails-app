import { readFileSync } from 'node:fs';

const read = (name: string) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('Tree content modes and cursor parity', () => {
  const directory = read('directory-pane.component.ts');
  const paneTree = read('pane-tree.component.ts');
  const container = read('file-view-container.component.ts');
  const controller = read('file-view-tree.controller.ts');

  it('offers all and folders as explicit Directory tree content modes', () => {
    expect(directory).toContain("type TreeContentMode = 'all' | 'folders';");
    expect(directory).toContain("treeContentMode: TreeContentMode = 'all';");
    expect(directory).toContain("setTreeContentMode('folders')");
    expect(directory).toContain("this.treeContentMode === 'folders' && entry.type !== FileType.DIRECTORY");
  });

  it('keeps folders mode available in the shared tree pipeline', () => {
    expect(paneTree).toContain("export type TreeContentMode = 'all' | 'folders';");
    expect(paneTree).toContain("@Input() contentMode: TreeContentMode = 'all';");
    expect(container).toContain('[contentMode]="treeContentMode"');
    expect(controller).toContain("host.treeContentMode === 'all' || entry.type === FileType.DIRECTORY");
  });

  it('uses the same rounded left-accent cursor language as other pane views', () => {
    expect(directory).toContain('border-radius: 3px;');
    expect(directory).toContain('inset 3px 0 0 var(--vsc-accent-blue');
    expect(directory).not.toContain('.pane.active-pane .tree-row.cursor {\n      outline:');
    expect(paneTree).toContain('border-radius: 3px;');
    expect(paneTree).toContain('inset 3px 0 0 var(--vsc-accent-blue');
  });
});
