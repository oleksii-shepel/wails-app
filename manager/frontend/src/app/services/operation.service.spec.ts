import { normalizeOperationKind, normalizeOperationStatus } from './operation.service';

describe('operation wire normalization', () => {
  it('normalizes backend uppercase operation kinds', () => {
    expect(normalizeOperationKind('COPY')).toBe('copy');
    expect(normalizeOperationKind('CALCULATE')).toBe('calculate');
  });

  it('normalizes backend uppercase operation statuses', () => {
    expect(normalizeOperationStatus('RUNNING')).toBe('running');
    expect(normalizeOperationStatus('PAUSED')).toBe('paused');
    expect(normalizeOperationStatus('COMPLETED')).toBe('completed');
  });
});