import type { OperationState } from '../services/operation.service';

/** Returns a percentage, or null when neither byte nor item totals are known. */
export function operationProgress(metrics: OperationState['metrics']): number | null {
  const { totalBytes, processedBytes, totalItems, processedItems } = metrics;
  const percentage = (processed: number, total: number): number | null =>
    Number.isFinite(total) && total > 0 && Number.isFinite(processed)
      ? Math.max(0, Math.min(100, (processed / total) * 100))
      : null;
  return percentage(processedBytes, totalBytes) ?? percentage(processedItems, totalItems);
}
