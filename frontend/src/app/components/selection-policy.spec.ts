import {
  reconcileVisibleSelection,
  selectPathRange,
  togglePathSelection,
} from './selection-policy';

describe('reassembly selection policy', () => {
  it('preserves the Shift anchor across Ctrl toggles', () => {
    const first = togglePathSelection(new Set(['a']), 'c', 'a');
    expect([...first.selected]).toEqual(['a', 'c']);
    expect(first.anchor).toBe('a');

    const second = togglePathSelection(first.selected, 'c', first.anchor);
    expect([...second.selected]).toEqual(['a']);
    expect(second.anchor).toBe('a');
  });

  it('creates an anchor when Ctrl selection starts without one', () => {
    const result = togglePathSelection(new Set(), 'b', null);
    expect([...result.selected]).toEqual(['b']);
    expect(result.anchor).toBe('b');
  });

  it('selects a deterministic range in projected order', () => {
    expect([...selectPathRange(['a', 'b', 'c', 'd'], 'b', 'd')!])
      .toEqual(['b', 'c', 'd']);
    expect([...selectPathRange(['a', 'b', 'c', 'd'], 'd', 'b')!])
      .toEqual(['b', 'c', 'd']);
  });

  it('drops invisible selections so operations cannot target filtered items', () => {
    const result = reconcileVisibleSelection({
      selected: new Set(['a', 'hidden', 'c']),
      cursor: 'hidden',
      anchor: 'hidden',
      visiblePaths: ['a', 'b', 'c'],
    });

    expect([...result.selected]).toEqual(['a', 'c']);
    expect(result.cursor).toBe('a');
    expect(result.anchor).toBe('a');
  });

  it('clears cursor and anchor for an empty projection', () => {
    const result = reconcileVisibleSelection({
      selected: new Set(['a']),
      cursor: 'a',
      anchor: 'a',
      visiblePaths: [],
    });

    expect(result.selected.size).toBe(0);
    expect(result.cursor).toBeNull();
    expect(result.anchor).toBeNull();
  });
});
