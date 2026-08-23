import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin stable render model', () => {
  it('uses trash identity for revision track keys', () => {
    expect(source).toContain('return `revision:${metadata.binTrashId}`;');
    expect(source).toContain('binRenderKey?: string;');
    expect(source).not.toContain('revision:${entry.name}');
  });

  it('uses normalized full original path for group identity', () => {
    expect(source).toContain('binRenderKey: `group:${key}`');
    expect(source).toContain('const key = this.binGroupKey(original);');
  });

  it('caches grouping and projection independently of cursor and scroll', () => {
    expect(source).toContain('private binGroupsEntriesRef: FileInfo[] | null = null;');
    expect(source).toContain('private binProjectionCache: FileInfo[] = [];');
    expect(source).toContain('this.binProjectionExpansionVersion === this.binExpansionVersion');
  });

  it('does not rebuild groups from binRevisionInfo', () => {
    const start = source.indexOf('binRevisionInfo(entry: FileInfo)');
    const end = source.indexOf('isBinGroupExpanded', start);
    const method = source.slice(start, end);
    expect(method).not.toContain('binRevisionGroups(');
    expect(method).not.toContain('findIndex');
  });

  it('renders revision labels from precomputed metadata', () => {
    expect(source).toContain('{{ binRevisionLabel(entry) }}');
    expect(source).toContain('metadata.binRevisionLabel = String(count - index);');
  });
});
