import type { FileInfo } from '../shared/protocol';
import { DirectoryPaneComponent } from './directory-pane.component';

describe('DirectoryPaneComponent tree rename', () => {
  const root = 'filesystem://c/root';
  const parent = root + '/child';
  const entry = { path: parent + '/old.txt', name: 'old.txt' } as FileInfo;

  function setup(existing: string[]) {
    const pane = Object.create(DirectoryPaneComponent.prototype) as DirectoryPaneComponent;
    const batchMove = jasmine.createSpy('batchMove').and.resolveTo({ failed: 0, skipped: 0 });
    const alert = jasmine.createSpy('alert').and.resolveTo(undefined);
    const refreshTreeNode = jasmine.createSpy('refreshTreeNode');
    Object.assign(pane, {
      path: root,
      viewMode: 'tree',
      api: {
        getParentPath: async (path: string) => path.slice(0, path.lastIndexOf('/')),
        joinPaths: async (segments: string[]) => segments.join('/'),
        getFileInfo: async (path: string) => {
          if (!existing.includes(path)) throw new Error('Missing');
          return { path };
        },
        batchMove,
      },
      confirmDialog: { alert },
      cdr: { markForCheck() {} },
      cursorChange: { emit() {} },
      operationHistory: { record() {} },
      paneViewHost: { refreshTreeNode },
      actionFeedback: { success() {} },
      refresh: async () => {},
      focusContent() {},
    });
    return { pane, batchMove, alert, refreshTreeNode };
  }

  it('ignores a namesake at the pane root and keeps the renamed child selected', async () => {
    const { pane, batchMove, alert, refreshTreeNode } = setup([root + '/new.txt']);
    const destination = parent + '/new.txt';
    await pane.renameEntry(entry, 'new.txt');

    expect(alert).not.toHaveBeenCalled();
    expect(batchMove).toHaveBeenCalledOnceWith([{ source: entry.path, destination, exactDestination: true }], true);
    expect(refreshTreeNode).toHaveBeenCalledWith(parent);
    expect(pane.selectedPaths).toEqual(new Set([destination]));
  });

  it('rejects a collision with a sibling in the expanded folder', async () => {
    const { pane, batchMove, alert } = setup([parent + '/new.txt']);
    await pane.renameEntry(entry, 'new.txt');

    expect(batchMove).not.toHaveBeenCalled();
    expect(alert).toHaveBeenCalledWith(jasmine.objectContaining({ title: 'Name Already Exists' }));
  });

  it('stages a case-only rename in the source folder', async () => {
    const destination = parent + '/OLD.txt';
    const { pane, batchMove } = setup([]);
    await pane.renameEntry(entry, 'OLD.txt');

    expect(batchMove).toHaveBeenCalledTimes(2);
    const temporary = batchMove.calls.first().args[0][0].destination;
    expect(temporary.startsWith(parent + '/.rename-')).toBeTrue();
    expect(batchMove.calls.mostRecent().args).toEqual([[{
      source: temporary, destination, exactDestination: true,
    }], true]);
  });
});
