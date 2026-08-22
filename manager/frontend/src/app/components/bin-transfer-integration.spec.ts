import { readFileSync } from 'node:fs';

const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const api = readFileSync(new URL('../services/api.service.ts', import.meta.url), 'utf8');

describe('Recycle Bin transfer integration', () => {
  it('routes bin transfers through explicit trash/copy/restore semantics', () => {
    expect(dual).toContain('transferRecycleBinDrop');
    expect(dual).toContain('transferRecycleBinBetweenPanes');
    expect(dual).toContain('batchDelete([...selected], true, true)');
    expect(dual).toContain('this.api.batchTransferTrash(');
    expect(dual).not.toContain('copyTrashEntryTo(item.id, destinationDir)');
    expect(dual).not.toContain('restoreTrashEntryTo(item.id, destinationDir)');
  });

  it('keeps copy-in blocked while enabling copy-out and restore-out', () => {
    expect(dual).toContain('Copying into the Recycle Bin is not supported');
    expect(dual).toContain("this.menuCommand(COMMAND.copyToOther, 'Copy to other pane', 'F5')");
    expect(dual).toContain("this.menuCommand(COMMAND.moveToOther, 'Restore to other pane', 'F6')");
    expect(api).toContain('async batchTransferTrash');
    expect(api).toContain('BatchTransferTrash(request:');
    expect(api).toContain('async copyTrashEntryTo');
    expect(api).toContain('async restoreTrashEntryTo');
  });
});
