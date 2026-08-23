import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin Directory-style projection performance', () => {
  it('caches grouping by entries collection identity', () => {
    expect(source).toContain('private binGroupsEntriesRef: FileInfo[] | null = null;');
    expect(source).toContain('if (this.binGroupsEntriesRef === entries)');
  });

  it('caches visible Bin projection independently of cursor and scroll', () => {
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

  it('uses stable trash identity instead of filename or index', () => {
    expect(source).toContain('return `revision:${metadata.binTrashId}`;');
    expect(source).toContain('binRenderKey: `group:${key}`');
    expect(source).not.toContain('revision:${entry.name}');
  });

  it('precomputes revision display metadata', () => {
    expect(source).toContain('metadata.binRevisionLabel = String(count - index);');
    expect(source).toContain('metadata.binDeletedText = this.computeBinDeletedText(entry);');
    expect(source).toContain('metadata.binOriginalDirectory = this.computeBinOriginalDirectory(original);');
  });

  it('renders revision labels through the O(1) helper', () => {
    expect(source).toContain('{{ binRevisionLabel(entry) }}');
  });
});
