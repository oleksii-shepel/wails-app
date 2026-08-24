import { readFileSync } from 'node:fs';

const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const api = readFileSync(new URL('../services/api.service.ts', import.meta.url), 'utf8');

const bindingsPath = new URL('../../../../internal/app/bindings_file_basics.go', import.meta.url);
const contractsPath = new URL('../../../../internal/contracts/trash_transfer.go', import.meta.url);
const bindings = readFileSync(bindingsPath, 'utf8');
const contracts = readFileSync(contractsPath, 'utf8');

describe('Recycle Bin operations integration', () => {
  it('uses one tracked backend operation for copy/restore transfers out of bin', () => {
    expect(dual).toContain('this.api.batchTransferTrash(');
    expect(api).toContain('BatchTransferTrash(request:');
    expect(bindings).toContain('func (a *App) BatchTransferTrash(');
    expect(bindings).toContain('a.StartOperation(opType, title, "bin://", req.DestinationDir)');
    expect(bindings).toContain('op.SetTotal(int64(len(req.IDs)), 0)');
    expect(bindings).toContain('op.UpdateProgress(');
    expect(bindings).toContain('op.IsCancelled()');
    expect(bindings).toContain('op.Complete()');
  });

  it('maps copy to COPY and restore-to-pane to MOVE operation kinds', () => {
    expect(bindings).toContain('opType = contracts.OpCopy');
    expect(bindings).toContain('title = "Copy from Recycle Bin"');
    expect(bindings).toContain('opType = contracts.OpMove');
    expect(bindings).toContain('title = "Restore from Recycle Bin"');
    expect(contracts).toContain('TrashTransferCopy');
    expect(contracts).toContain('TrashTransferRestore');
  });
  it('models filesystem to Recycle Bin as MOVE progress', () => {
    const batchService = readFileSync(
      new URL('../../../../internal/batch/service.go', import.meta.url),
      'utf8',
    );
    const transfers = readFileSync(
      new URL('../../../../internal/batch/transfers.go', import.meta.url),
      'utf8',
    );

    expect(batchService).toContain('opType = contracts.OpMove');
    expect(batchService).toContain('title = "Move to Recycle Bin"');
    expect(batchService).toContain('destination = "bin://"');
    expect(transfers).toContain('itemWork := work[path]');
    expect(transfers).toContain('itemWork.bytes');
  });

});