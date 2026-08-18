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

export interface VirtualCollectionOptions<T, K> {
  readonly identity: (item: T) => K;
  readonly itemExtent: number;
  readonly columns?: number;
  readonly threshold?: number;
  readonly overscanRows?: number;
  readonly guardRows?: number;
  readonly minimumPrefetchItems?: number;
  readonly prefetchMultiplier?: number;
  readonly prefetchGuardRatio?: number;
}

export interface VirtualViewport {
  readonly scrollTop: number;
  readonly viewportHeight: number;
  readonly columns?: number;
}

export interface VirtualPrefetchWindow {
  readonly offset: number;
  readonly limit: number;
  readonly end: number;
}

export interface VirtualCollectionSnapshot {
  readonly generation: number;
  readonly logicalCount: number;
  readonly loadedOffset: number;
  readonly loadedCount: number;
  readonly render: VirtualWindow;
  readonly prefetch: VirtualPrefetchWindow;
}

interface MutableVirtualCollectionOptions<T, K> {
  identity: (item: T) => K;
  itemExtent: number;
  columns: number;
  threshold: number;
  overscanRows: number;
  guardRows: number;
  minimumPrefetchItems: number;
  prefetchMultiplier: number;
  prefetchGuardRatio: number;
}

function clampInteger(value: number, minimum: number, maximum = Number.MAX_SAFE_INTEGER): number {
  if (!Number.isFinite(value)) return minimum;
  return Math.max(minimum, Math.min(maximum, Math.floor(value)));
}

function calculateRawWindow(input: VirtualWindowInput): VirtualWindow {
  const itemCount = clampInteger(input.itemCount, 0);
  const columns = clampInteger(input.columns ?? 1, 1);
  const itemExtent = Math.max(1, Number.isFinite(input.itemExtent) ? input.itemExtent : 1);
  const threshold = clampInteger(input.threshold ?? 300, 1);
  const overscanRows = clampInteger(input.overscanRows ?? 10, 0);
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
  const endRow = Math.min(totalRows, firstVisibleRow + visibleRows + overscanRows);
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

/**
 * Generic virtualization/window engine shared by directory views and result sets.
 *
 * Logical collection size is deliberately independent from the currently loaded
 * item window. This is what allows a 100k-entry source to expose correct scroll
 * geometry while only ~1k entries are resident on the frontend.
 */
export class VirtualCollection<T, K = string> {
  private options: MutableVirtualCollectionOptions<T, K>;

  private generationValue = 0;
  private logicalCountValue = 0;

  private loadedOffsetValue = 0;
  private loadedItemsValue: readonly T[] = [];
  private loadedIdentityIndex = new Map<K, number>();

  private renderWindow: VirtualWindow;
  private prefetchWindow: VirtualPrefetchWindow = { offset: 0, limit: 0, end: 0 };

  constructor(options: VirtualCollectionOptions<T, K>) {
    this.options = {
      identity: options.identity,
      itemExtent: Math.max(1, options.itemExtent),
      columns: clampInteger(options.columns ?? 1, 1),
      threshold: clampInteger(options.threshold ?? 300, 1),
      overscanRows: clampInteger(options.overscanRows ?? 10, 0),
      guardRows: clampInteger(options.guardRows ?? 4, 0),
      minimumPrefetchItems: clampInteger(options.minimumPrefetchItems ?? 320, 1),
      prefetchMultiplier: Math.max(1, options.prefetchMultiplier ?? 2),
      prefetchGuardRatio: Math.max(0, Math.min(0.49, options.prefetchGuardRatio ?? 0.2)),
    };

    this.renderWindow = calculateRawWindow({
      itemCount: 0,
      scrollTop: 0,
      viewportHeight: this.options.itemExtent,
      itemExtent: this.options.itemExtent,
      columns: this.options.columns,
      overscanRows: this.options.overscanRows,
      threshold: this.options.threshold,
    });
  }

  get generation(): number {
    return this.generationValue;
  }

  get logicalCount(): number {
    return this.logicalCountValue;
  }

  get loadedOffset(): number {
    return this.loadedOffsetValue;
  }

  get loadedItems(): readonly T[] {
    return this.loadedItemsValue;
  }

  get window(): VirtualWindow {
    return this.renderWindow;
  }

  configure(options: Partial<Omit<VirtualCollectionOptions<T, K>, 'identity'>>): void {
    if (options.itemExtent !== undefined) {
      this.options.itemExtent = Math.max(1, options.itemExtent);
    }
    if (options.columns !== undefined) {
      this.options.columns = clampInteger(options.columns, 1);
    }
    if (options.threshold !== undefined) {
      this.options.threshold = clampInteger(options.threshold, 1);
    }
    if (options.overscanRows !== undefined) {
      this.options.overscanRows = clampInteger(options.overscanRows, 0);
    }
    if (options.guardRows !== undefined) {
      this.options.guardRows = clampInteger(options.guardRows, 0);
    }
    if (options.minimumPrefetchItems !== undefined) {
      this.options.minimumPrefetchItems = clampInteger(options.minimumPrefetchItems, 1);
    }
    if (options.prefetchMultiplier !== undefined) {
      this.options.prefetchMultiplier = Math.max(1, options.prefetchMultiplier);
    }
    if (options.prefetchGuardRatio !== undefined) {
      this.options.prefetchGuardRatio = Math.max(
        0,
        Math.min(0.49, options.prefetchGuardRatio),
      );
    }
  }

  /**
   * Begins a new source generation. Async callers should capture the returned
   * generation and verify isCurrent() before committing results.
   */
  begin(logicalCount = 0): number {
    this.generationValue += 1;
    this.logicalCountValue = clampInteger(logicalCount, 0);
    this.loadedOffsetValue = 0;
    this.loadedItemsValue = [];
    this.loadedIdentityIndex.clear();
    this.renderWindow = calculateRawWindow({
      itemCount: this.logicalCountValue,
      scrollTop: 0,
      viewportHeight: this.options.itemExtent,
      itemExtent: this.options.itemExtent,
      columns: this.options.columns,
      overscanRows: this.options.overscanRows,
      threshold: this.options.threshold,
    });
    this.prefetchWindow = this.calculatePrefetch(this.renderWindow);
    return this.generationValue;
  }

  cancel(): number {
    this.generationValue += 1;
    return this.generationValue;
  }

  isCurrent(generation: number): boolean {
    return generation === this.generationValue;
  }

  setLogicalCount(count: number): boolean {
    const next = clampInteger(count, 0);
    if (next === this.logicalCountValue) return false;
    this.logicalCountValue = next;
    return true;
  }

  /**
   * Replaces the currently resident population without affecting logical size.
   * Offset is the logical index represented by items[0].
   */
  populate(offset: number, items: readonly T[], logicalCount?: number): void {
    if (logicalCount !== undefined) {
      this.setLogicalCount(logicalCount);
    }
    this.loadedOffsetValue = clampInteger(offset, 0, this.logicalCountValue);
    this.loadedItemsValue = items;
    this.rebuildIdentityIndex();
  }

  /**
   * Efficient incremental population for local/result collections whose loaded
   * window starts at zero.
   */
  append(items: readonly T[], logicalCount?: number): void {
    if (!items.length) {
      if (logicalCount !== undefined) this.setLogicalCount(logicalCount);
      return;
    }
    if (logicalCount !== undefined) {
      this.setLogicalCount(logicalCount);
    } else if (this.loadedOffsetValue === 0) {
      this.logicalCountValue = Math.max(
        this.logicalCountValue,
        this.loadedItemsValue.length + items.length,
      );
    }
    this.loadedItemsValue = [...this.loadedItemsValue, ...items];
    this.rebuildIdentityIndex();
  }

  /**
   * Updates viewport geometry. The materialized render window is retained while
   * the visible range remains inside its guard band.
   */
  updateViewport(viewport: VirtualViewport, force = false): boolean {
    const columns = clampInteger(viewport.columns ?? this.options.columns, 1);
    this.options.columns = columns;

    const visible = calculateRawWindow({
      itemCount: this.logicalCountValue,
      scrollTop: viewport.scrollTop,
      viewportHeight: viewport.viewportHeight,
      itemExtent: this.options.itemExtent,
      columns,
      overscanRows: 0,
      threshold: this.options.threshold,
    });

    if (!visible.enabled) {
      const next = calculateRawWindow({
        itemCount: this.logicalCountValue,
        scrollTop: viewport.scrollTop,
        viewportHeight: viewport.viewportHeight,
        itemExtent: this.options.itemExtent,
        columns,
        overscanRows: this.options.overscanRows,
        threshold: this.options.threshold,
      });
      const changed = force || !sameWindow(this.renderWindow, next);
      if (changed) {
        this.renderWindow = next;
        this.prefetchWindow = this.calculatePrefetch(next);
      }
      return changed;
    }

    const guardItems = this.options.guardRows * columns;
    if (
      !force &&
      this.renderWindow.enabled &&
      this.renderWindow.columns === columns &&
      visible.start >= this.renderWindow.start + guardItems &&
      visible.end <= this.renderWindow.end - guardItems
    ) {
      return false;
    }

    const next = calculateRawWindow({
      itemCount: this.logicalCountValue,
      scrollTop: viewport.scrollTop,
      viewportHeight: viewport.viewportHeight,
      itemExtent: this.options.itemExtent,
      columns,
      overscanRows: this.options.overscanRows,
      threshold: this.options.threshold,
    });

    const changed = force || !sameWindow(this.renderWindow, next);
    if (changed) {
      this.renderWindow = next;
      this.prefetchWindow = this.calculatePrefetch(next);
    }
    return changed;
  }

  /**
   * Ensures a logical item index is represented by the render window. Useful
   * for keyboard-driven result lists that do not expose their own scroll host.
   */
  revealIndex(index: number, viewportItems = 160): boolean {
    if (this.logicalCountValue <= 0) return false;
    const safeIndex = clampInteger(index, 0, this.logicalCountValue - 1);
    const columns = this.options.columns;
    const rows = Math.max(1, Math.ceil(viewportItems / columns));
    const row = Math.floor(safeIndex / columns);
    const scrollTop = row * this.options.itemExtent;
    return this.updateViewport({
      scrollTop,
      viewportHeight: rows * this.options.itemExtent,
      columns,
    });
  }

  prefetch(): VirtualPrefetchWindow {
    return this.prefetchWindow;
  }

  /**
   * True when the requested render range is still comfortably inside the
   * currently resident backend window.
   */
  isRenderCoveredByLoadedGuard(window = this.renderWindow): boolean {
    if (!this.loadedItemsValue.length) return false;
    const loadedEnd = this.loadedOffsetValue + this.loadedItemsValue.length;
    const guard = Math.max(
      this.options.columns,
      Math.floor(this.loadedItemsValue.length * this.options.prefetchGuardRatio),
    );
    return (
      window.start >= this.loadedOffsetValue + guard &&
      window.end <= loadedEnd - guard
    );
  }

  renderedItems(window = this.renderWindow): readonly T[] {
    if (!this.loadedItemsValue.length) return [];
    const loadedEnd = this.loadedOffsetValue + this.loadedItemsValue.length;
    const start = Math.max(window.start, this.loadedOffsetValue);
    const end = Math.min(window.end, loadedEnd);
    if (end <= start) return [];
    return this.loadedItemsValue.slice(
      start - this.loadedOffsetValue,
      end - this.loadedOffsetValue,
    );
  }

  indexOfIdentity(identity: K): number {
    return this.loadedIdentityIndex.get(identity) ?? -1;
  }

  snapshot(): VirtualCollectionSnapshot {
    return {
      generation: this.generationValue,
      logicalCount: this.logicalCountValue,
      loadedOffset: this.loadedOffsetValue,
      loadedCount: this.loadedItemsValue.length,
      render: this.renderWindow,
      prefetch: this.prefetchWindow,
    };
  }

  private calculatePrefetch(window: VirtualWindow): VirtualPrefetchWindow {
    const visibleSpan = Math.max(
      this.options.columns,
      window.end - window.start,
    );
    const limit = Math.max(
      this.options.minimumPrefetchItems,
      Math.ceil(visibleSpan * this.options.prefetchMultiplier),
    );
    const offset = Math.max(
      0,
      Math.min(
        Math.max(0, this.logicalCountValue - limit),
        window.start - Math.floor((limit - visibleSpan) / 2),
      ),
    );
    const end = Math.min(this.logicalCountValue, offset + limit);
    return {
      offset,
      limit: Math.max(0, end - offset),
      end,
    };
  }

  private rebuildIdentityIndex(): void {
    this.loadedIdentityIndex.clear();
    for (let index = 0; index < this.loadedItemsValue.length; index += 1) {
      this.loadedIdentityIndex.set(
        this.options.identity(this.loadedItemsValue[index]),
        this.loadedOffsetValue + index,
      );
    }
  }
}

function sameWindow(a: VirtualWindow, b: VirtualWindow): boolean {
  return (
    a.enabled === b.enabled &&
    a.start === b.start &&
    a.end === b.end &&
    a.columns === b.columns &&
    a.totalExtentPx === b.totalExtentPx
  );
}

/**
 * Backwards-compatible pure facade for callers that only need one calculation.
 * New stateful collection surfaces should own a VirtualCollection instance.
 */
export function calculateVirtualWindow(input: VirtualWindowInput): VirtualWindow {
  return calculateRawWindow(input);
}
