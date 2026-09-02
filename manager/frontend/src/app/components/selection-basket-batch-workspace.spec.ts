import { readFileSync } from 'node:fs';

const read = (name: string) => readFileSync(new URL(name, import.meta.url), 'utf8');

describe('Selection Basket batch workspace', () => {
  const basket = read('./selection-basket.component.ts');
  const app = read('../app.component.ts');
  const dual = read('./dual-pane.component.ts');

  it('prepares copy, move and delete operations in one workspace', () => {
    expect(basket).toContain("operation: SelectionBasketOperation = 'copy';");
    expect(basket).toContain("selectOperation('copy')");
    expect(basket).toContain("selectOperation('move')");
    expect(basket).toContain("selectOperation('delete')");
    expect(basket).toContain('executeRequested.emit({');
  });

  it('uses pane destinations and only enables writable directory panes', () => {
    expect(app).toContain('[leftDestination]="managerRef?.leftPane?.contextState?.path ?? ''"');
    expect(app).toContain('[rightDestination]="managerRef?.rightPane?.contextState?.path ?? ''"');
    expect(app).toContain('[leftDestinationWritable]');
    expect(app).toContain('[rightDestinationWritable]');
  });

  it('forwards recursive and recycle options to the existing batch APIs', () => {
    expect(dual).toContain('executeBasketOperation(request: SelectionBasketOperationRequest)');
    expect(dual).toContain('this.deleteBasket(request.recursive, request.useTrash)');
    expect(dual).toContain('this.api.batchCopy(items, recursive)');
    expect(dual).toContain('this.api.batchMove(items, recursive)');
    expect(dual).toContain('this.api.batchDelete(paths, recursive, useTrash)');
  });

  it('removes successful sources while preserving unresolved basket entries', () => {
    expect(dual).toContain('removeSuccessfulBasketSources(');
    expect(dual).toContain('this.selectionBasketService.removePaths(explicit);');
    expect(dual).toContain('const failed = new Set(failedSources.filter(Boolean));');
  });
});
