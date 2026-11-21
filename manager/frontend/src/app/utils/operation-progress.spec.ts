import { operationProgress } from './operation-progress';

describe('operationProgress', () => {
  it('prefers known byte totals', () => {
    expect(operationProgress({ processedBytes: 25, totalBytes: 100, processedItems: 3, totalItems: 4 })).toBe(25);
  });

  it('uses file counts when byte totals are unavailable', () => {
    expect(operationProgress({ processedBytes: 0, totalBytes: 0, processedItems: 3, totalItems: 4 })).toBe(75);
  });

  it('returns null when progress is indeterminate', () => {
    expect(operationProgress({ processedBytes: 0, totalBytes: 0, processedItems: 0, totalItems: 0 })).toBeNull();
  });

  it('clamps progress to the valid display range', () => {
    expect(operationProgress({ processedBytes: 110, totalBytes: 100, processedItems: 0, totalItems: 0 })).toBe(100);
    expect(operationProgress({ processedBytes: -1, totalBytes: 100, processedItems: 0, totalItems: 0 })).toBe(0);
  });
});
