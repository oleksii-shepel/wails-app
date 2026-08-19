import { OperationHistoryService } from './operation-history.service';

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