import { markerForSide, nextDifferenceIndex } from './correspondence-policy';

describe('Reassembly correspondence policy', () => {
  it('maps newer/older per side', () => {
    expect(markerForSide('left-newer','left')).toBe('newer');
    expect(markerForSide('left-newer','right')).toBe('older');
  });

  it('marks missing items only on the owning side', () => {
    expect(markerForSide('only-left','left')).toBe('only-here');
    expect(markerForSide('only-right','right')).toBe('only-here');
  });

  it('wraps difference navigation', () => {
    expect(nextDifferenceIndex(-1,3,1)).toBe(0);
    expect(nextDifferenceIndex(-1,3,-1)).toBe(2);
    expect(nextDifferenceIndex(2,3,1)).toBe(0);
  });
});