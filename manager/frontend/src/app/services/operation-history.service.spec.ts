import { OperationHistoryService } from './operation-history.service';

describe('OperationHistoryService undo and redo', () => {
  it('updates each same-label operation separately', async () => {
    const service = new OperationHistoryService();
    for (const path of ['/first', '/second']) {
      service.record({
        label: 'Copy',
        affectedPaths: [path],
        undo: async () => {},
        redo: async () => {},
      });
    }
    const [second, first] = service.recentOperations();

    await service.undo();
    await service.undo();
    expect(service.recentOperations().map(({ id, state }) => [id, state]))
      .toEqual([[second.id, 'undone'], [first.id, 'undone']]);

    await service.redo();
    expect(service.recentOperations().map(({ id, state }) => [id, state]))
      .toEqual([[second.id, 'undone'], [first.id, 'redone']]);
  });

  it('restores the undo action after a failed attempt', async () => {
    const service = new OperationHistoryService();
    service.record({
      label: 'Rename',
      undo: async () => { throw new Error('Requested file is missing'); },
      redo: async () => {},
    });

    await expectAsync(service.undo()).toBeRejectedWithError('Requested file is missing');
    expect(service.canUndo).toBe(true);
    expect(service.canRedo).toBe(false);
    expect(service.recentOperations()[0].state).toBe('completed');
  });
});

describe('OperationHistoryService synchronization journal', () => {
  beforeEach(() => localStorage.clear());

  it('records completed synchronization runs independently of undo history', () => {
    const service = new OperationHistoryService();
    service.recordSync({
      startedAt: 10, completedAt: 20, leftRoot: '/left', rightRoot: '/right',
      mode: 'bidirectional', recursive: true, planned: 4, succeeded: 3, skipped: 1, failed: 0, conflicts: 1,
    });

    expect(service.canUndo).toBe(false);
    expect(service.recentSyncs(1)[0]).toEqual(jasmine.objectContaining({
      leftRoot: '/left', rightRoot: '/right', mode: 'bidirectional', succeeded: 3, conflicts: 1,
    }));
  });

  it('persists synchronization history across service instances', () => {
    const first = new OperationHistoryService();
    first.recordSync({
      startedAt: 1, completedAt: 2, leftRoot: 'L', rightRoot: 'R',
      mode: 'left-to-right', recursive: false, planned: 1, succeeded: 1, skipped: 0, failed: 0, conflicts: 0,
    });

    const second = new OperationHistoryService();
    expect(second.recentSyncs()).toHaveSize(1);
    expect(second.recentSyncs()[0].mode).toBe('left-to-right');
  });
});
