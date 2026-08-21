import { readFileSync } from 'node:fs';

const protocol = readFileSync(new URL('../shared/protocol.ts', import.meta.url), 'utf8');
const coreFiles = readFileSync(
  new URL('../../../../../../packages/core/frontend/src/contracts/files/file-info.ts', import.meta.url),
  'utf8',
);
const coreState = readFileSync(
  new URL('../../../../../../packages/core/frontend/src/contracts/panes/state.ts', import.meta.url),
  'utf8',
);
const coreCommands = readFileSync(
  new URL('../../../../../../packages/core/frontend/src/contracts/panes/commands.ts', import.meta.url),
  'utf8',
);

describe('core contract compatibility', () => {
  it('uses core FileType/FileInfo as Manager canonical file contracts', () => {
    expect(protocol).toContain("from '@reassembly/core/contracts/files'");
    expect(protocol).not.toContain('export enum FileType {');
    expect(protocol).not.toContain('export interface FileInfo {');
  });

  it('does not require provider metadata through an index signature', () => {
    expect(coreFiles).not.toContain('[key: string]: unknown');
    expect(coreFiles).toContain('export enum FileType');
  });

  it('preserves existing pane tab/source compatibility while extraction settles', () => {
    expect(coreState).toContain('readonly activeTab: any');
    expect(coreState).toContain('readonly activeViewerTab: any');
    expect(coreState).toContain('readonly activePanelSource: any');
  });

  it('types archive transfers instead of leaking unknown to DualPane', () => {
    expect(coreCommands).toContain('export interface ArchiveTransferResult');
    expect(coreCommands).toContain('Promise<ArchiveTransferResult | null>');
  });
});
