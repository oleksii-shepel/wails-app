import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';

const pane = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
const source = readFileSync(new URL('../services/directory-source.service.ts', import.meta.url), 'utf8');

describe('bin:// version-aware grouped view', () => {
  it('preserves exact trash identity and origin metadata on every row', () => {
    expect(source).toContain('binTrashId: entry.id');
    expect(source).toContain("binOriginalPath: entry.originalPath || ''");
    expect(source).toContain("binDeletedAt: entry.deletedAt || ''");
  });

  it('groups versions by normalized original path and labels latest/vN', () => {
    expect(pane).toContain('private binVersionGroups(entries: FileInfo[])');
    expect(pane).toContain("label: index === 0 ? 'Latest' : `v${group.length - index}`");
    expect(pane).toContain('this.expandedBinGroups.has(group.key)');
  });

  it('supports keyboard expansion/collapse without replacing version identity', () => {
    expect(pane).toContain('async treeLeft(): Promise<void>');
    expect(pane).toContain('async treeRight(): Promise<void>');
    expect(pane).toContain('if (this.isBinLocation)');
    expect(pane).toContain('const child = this.binVersionGroups(this.entries).get(info.groupKey)?.[1]');
  });

  it('searches both name and original path', () => {
    expect(pane).toContain('this.binOriginalPath(entry).toLocaleLowerCase().includes(query)');
  });
});


describe('bin:// production row layout', () => {
  it('keeps disclosure and icon in one grid cell', () => {
    expect(pane).toContain('class="bin-leading"');
    expect(pane).toContain('#normalEntryIcon');
  });

  it('shows original location only on Latest', () => {
    expect(pane).toContain("binVersionInfo(entry).latest ? binOriginalDirectory(entry) : ''");
  });
});


describe('bin:// metadata typing', () => {
  it('uses explicit structural fields without stale ad-hoc bin object access', () => {
    expect(pane).toContain('type BinFileInfo = FileInfo & {');
    expect(pane).toContain('binOriginalPath?: string;');
    expect(pane).toContain('binDeletedAt?: string;');
    expect(pane).not.toContain('(entry as BinFileInfo).bin;');
    expect(pane).not.toContain('(a as BinFileInfo).bin?.deletedAt');
    expect(pane).not.toContain('(b as BinFileInfo).bin?.deletedAt');
  });
});
