import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('Details workspace folder tree and containment', () => {
  const dual = read('dual-pane.component.ts');
  const host = read('tool-workspace-host.component.ts');

  it('builds Details as a folder-tree / inspector split', () => {
    expect(dual).toContain("'details-tree'");
    expect(dual).toContain("'details-workspace'");
    expect(dual).toContain("splitLayout(\n          'horizontal'");
    expect(dual).toContain('          25,');
    expect(host).toContain('[class.details-workspace-split]="isDetailsWorkspaceSplit(layout)"');
    expect(host).toContain('grid-template-columns: minmax(220px, 25%) minmax(0, 1fr);');
  });

  it('selects folders in the tree and retargets the details surface', () => {
    expect(host).toContain("emitNamedAction(surface, 'select-folder', node)");
    expect(host).toContain("emitNamedAction(surface, 'toggle-folder', node)");
    expect(dual).toContain('selectDetailsTreeFolder');
    expect(dual).toContain("this.patchLiveSurface(detailsSurface.id, { path, error: '' });");
  });

  it('loads directory children lazily and only puts folders in the tree', () => {
    expect(dual).toContain('loadDetailsTreeChildren');
    expect(dual).toContain('entry.type === FileType.DIRECTORY');
    expect(dual).toContain('node.loaded = true;');
    expect(dual).toContain("contentMode: 'folders'");
  });

  it('shows selected-folder metadata and immediate containment in the large inspector', () => {
    expect(dual).toContain('containmentSummary');
    expect(dual).toContain('containment: entries.map');
    expect(host).toContain('<h3>Selected folder contents</h3>');
    expect(host).toContain("{{ text(surface, 'path') }}");
    expect(host).toContain('containment-table');
    expect(host).toContain('Attributes & permissions');
  });

  it('uses the normal pane cursor language and compact tree geometry', () => {
    expect(host).toContain('[class.cursor]');
    expect(host).toContain('inset 3px 0 0 var(--vsc-accent-blue');
    expect(host).toContain('grid-template-columns: 16px 18px minmax(0, 1fr)');
  });
});
