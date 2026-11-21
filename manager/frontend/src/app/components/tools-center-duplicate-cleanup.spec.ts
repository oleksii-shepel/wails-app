import '@angular/compiler';
import { ToolsCenterComponent } from './tools-center.component';
import type { DuplicateScanResult } from '../services/duplicate-finder.service';

function setup(confirm: () => Promise<boolean>) {
  const keeper = '/test/keep.txt';
  const duplicate = '/test/copy.txt';
  const group = {
    suggestedKeeperPath: keeper,
    files: [{ path: keeper }, { path: duplicate }],
  };
  const scan = { groups: [group] } as unknown as DuplicateScanResult;
  const batchDelete = jasmine.createSpy('batchDelete').and.resolveTo({ failed: 0 });
  const component = Object.create(ToolsCenterComponent.prototype) as ToolsCenterComponent;
  Object.assign(
    component,
    {
      confirm: { confirm },
      api: { batchDelete },
      duplicates: { suggestedRedundantPaths: () => [duplicate] },
      cdr: { markForCheck() {} },
      changed: { emit() {} },
      rootPath: '/test',
      duplicateFilter: '',
      duplicateResult: scan,
      duplicateSelected: new Set([keeper, duplicate]),
      duplicateDeleting: false,
      visible: true,
      scanDuplicates: async () => {},
    },
  );
  return { component, batchDelete, duplicate };
}

describe('Tools Center duplicate cleanup', () => {
  it('sends only redundant files to Trash', async () => {
    const { component, batchDelete, duplicate } = setup(async () => true);

    await component.deleteDuplicateSelection();

    expect(batchDelete).toHaveBeenCalledOnceWith([duplicate], false, true);
  });

  it('does not apply a confirmed action to replaced scan results', async () => {
    let accept!: (answer: boolean) => void;
    const { component, batchDelete } = setup(() => new Promise(resolve => { accept = resolve; }));

    const request = component.deleteDuplicateSelection();
    component.duplicateResult = null;
    accept(true);
    await request;

    expect(batchDelete).not.toHaveBeenCalled();
  });

  it('allows only one cleanup confirmation at a time', async () => {
    let accept!: (answer: boolean) => void;
    const confirm = jasmine.createSpy('confirm').and.callFake(
      () => new Promise<boolean>(resolve => { accept = resolve; }),
    );
    const { component, batchDelete } = setup(confirm);

    const request = component.deleteDuplicateSelection();
    await component.deleteDuplicateSelection();
    accept(false);
    await request;

    expect(confirm).toHaveBeenCalledTimes(1);
    expect(batchDelete).not.toHaveBeenCalled();
  });
});
