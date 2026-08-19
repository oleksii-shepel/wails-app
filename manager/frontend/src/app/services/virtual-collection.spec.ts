import { VirtualCollection } from './virtual-window';

interface Item {
  id: string;
}

function items(count: number, offset = 0): Item[] {
  return Array.from({ length: count }, (_, index) => ({
    id: `item-${offset + index}`,
  }));
}

describe('VirtualCollection', () => {
  it('keeps logical extent independent from loaded population', () => {
    const collection = new VirtualCollection<Item, string>({
      identity: item => item.id,
      itemExtent: 40,
      threshold: 100,
      overscanRows: 20,
      guardRows: 8,
    });

    collection.setLogicalCount(100_000);
    collection.populate(40_000, items(1_024, 40_000), 100_000);
    collection.updateViewport({
      scrollTop: 40_200 * 40,
      viewportHeight: 800,
    }, true);

    expect(collection.logicalCount).toBe(100_000);
    expect(collection.loadedItems.length).toBe(1_024);
    expect(collection.window.totalExtentPx).toBe(4_000_000);
  });

  it('retains the render window while visible rows remain inside the guard', () => {
    const collection = new VirtualCollection<Item, string>({
      identity: item => item.id,
      itemExtent: 40,
      threshold: 100,
      overscanRows: 64,
      guardRows: 24,
    });

    collection.setLogicalCount(10_000);
    collection.populate(0, items(10_000), 10_000);

    expect(collection.updateViewport({
      scrollTop: 4_000,
      viewportHeight: 800,
    }, true)).toBeTrue();

    const first = collection.window;
    expect(collection.updateViewport({
      scrollTop: 4_200,
      viewportHeight: 800,
    })).toBeFalse();
    expect(collection.window).toEqual(first);
  });

  it('computes a bounded backend prefetch window around the render window', () => {
    const collection = new VirtualCollection<Item, string>({
      identity: item => item.id,
      itemExtent: 44,
      threshold: 100,
      overscanRows: 48,
      guardRows: 24,
      minimumPrefetchItems: 1_024,
      prefetchMultiplier: 4,
    });

    collection.setLogicalCount(100_000);
    collection.updateViewport({
      scrollTop: 50_000 * 44,
      viewportHeight: 880,
    }, true);

    const prefetch = collection.prefetch();
    expect(prefetch.limit).toBeGreaterThanOrEqual(1_024);
    expect(prefetch.offset).toBeGreaterThan(0);
    expect(prefetch.end).toBeLessThanOrEqual(100_000);
  });

  it('supports generation cancellation', () => {
    const collection = new VirtualCollection<Item, string>({
      identity: item => item.id,
      itemExtent: 40,
    });

    const generation = collection.begin(1_000);
    expect(collection.isCurrent(generation)).toBeTrue();

    collection.cancel();
    expect(collection.isCurrent(generation)).toBeFalse();
  });

  it('indexes stable item identity across a loaded backend window', () => {
    const collection = new VirtualCollection<Item, string>({
      identity: item => item.id,
      itemExtent: 40,
    });

    collection.setLogicalCount(100_000);
    collection.populate(20_000, items(1_024, 20_000), 100_000);

    expect(collection.indexOfIdentity('item-20000')).toBe(20_000);
    expect(collection.indexOfIdentity('item-21023')).toBe(21_023);
    expect(collection.indexOfIdentity('missing')).toBe(-1);
  });

  it('returns only loaded items intersecting the logical render window', () => {
    const collection = new VirtualCollection<Item, string>({
      identity: item => item.id,
      itemExtent: 40,
      threshold: 100,
      overscanRows: 10,
      guardRows: 4,
    });

    collection.setLogicalCount(100_000);
    collection.populate(50_000, items(1_024, 50_000), 100_000);
    collection.updateViewport({
      scrollTop: 50_100 * 40,
      viewportHeight: 800,
    }, true);

    const rendered = collection.renderedItems();
    expect(rendered.length).toBeGreaterThan(0);
    expect(rendered.length).toBeLessThan(1_024);
  });
});