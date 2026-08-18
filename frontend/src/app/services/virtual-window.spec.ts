import { calculateVirtualWindow } from './virtual-window';

describe('Reassembly virtual window', () => {
  it('does not virtualize small collections', () => {
    expect(calculateVirtualWindow({
      itemCount: 120,
      scrollTop: 0,
      viewportHeight: 600,
      itemExtent: 30,
    })).toEqual(jasmine.objectContaining({
      enabled: false,
      start: 0,
      end: 120,
    }));
  });

  it('renders only a bounded list window with overscan', () => {
    const window = calculateVirtualWindow({
      itemCount: 100_000,
      scrollTop: 30_000,
      viewportHeight: 600,
      itemExtent: 30,
      overscanRows: 10,
    });

    expect(window.enabled).toBeTrue();
    expect(window.end - window.start).toBeLessThan(100);
    expect(window.beforePx).toBeGreaterThan(0);
    expect(window.afterPx).toBeGreaterThan(0);
  });

  it('virtualizes grid rows rather than individual grid cells', () => {
    const window = calculateVirtualWindow({
      itemCount: 10_000,
      scrollTop: 10_000,
      viewportHeight: 600,
      itemExtent: 109,
      columns: 8,
      overscanRows: 3,
    });

    expect(window.start % 8).toBe(0);
    expect(window.columns).toBe(8);
  });
});
