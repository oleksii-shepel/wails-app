export interface VirtualWindow {
  readonly enabled: boolean;
  readonly start: number;
  readonly end: number;
  readonly beforePx: number;
  readonly afterPx: number;
  readonly totalExtentPx: number;
  readonly columns: number;
}

export interface VirtualWindowInput {
  readonly itemCount: number;
  readonly scrollTop: number;
  readonly viewportHeight: number;
  readonly itemExtent: number;
  readonly columns?: number;
  readonly overscanRows?: number;
  readonly threshold?: number;
}

export function calculateVirtualWindow(
  input: VirtualWindowInput,
): VirtualWindow {
  const itemCount = Math.max(0, Math.floor(input.itemCount));
  const columns = Math.max(1, Math.floor(input.columns ?? 1));
  const itemExtent = Math.max(1, input.itemExtent);
  const threshold = Math.max(1, Math.floor(input.threshold ?? 300));
  const overscanRows = Math.max(0, Math.floor(input.overscanRows ?? 10));
  const totalRows = Math.ceil(itemCount / columns);
  const totalExtentPx = totalRows * itemExtent;

  if (itemCount < threshold) {
    return {
      enabled: false,
      start: 0,
      end: itemCount,
      beforePx: 0,
      afterPx: 0,
      totalExtentPx,
      columns,
    };
  }

  const viewportHeight = Math.max(itemExtent, input.viewportHeight || 600);
  const maxScroll = Math.max(0, totalExtentPx - viewportHeight);
  const scrollTop = Math.max(0, Math.min(maxScroll, input.scrollTop || 0));

  const firstVisibleRow = Math.floor(scrollTop / itemExtent);
  const visibleRows = Math.max(1, Math.ceil(viewportHeight / itemExtent));

  const startRow = Math.max(0, firstVisibleRow - overscanRows);
  const endRow = Math.min(
    totalRows,
    firstVisibleRow + visibleRows + overscanRows,
  );

  const start = Math.min(itemCount, startRow * columns);
  const end = Math.min(itemCount, endRow * columns);

  return {
    enabled: true,
    start,
    end,
    beforePx: startRow * itemExtent,
    afterPx: Math.max(0, (totalRows - endRow) * itemExtent),
    totalExtentPx,
    columns,
  };
}
