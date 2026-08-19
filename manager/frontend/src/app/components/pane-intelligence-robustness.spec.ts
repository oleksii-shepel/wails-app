import {
  comparisonSignature,
  decidePaneHealth,
} from './pane-intelligence-robustness';

describe('Reassembly pane intelligence robustness', () => {
  it('disables live cooperation when a pane location disappears', () => {
    expect(decidePaneHealth({
      leftAvailable: false,
      rightAvailable: true,
      leftRootAvailable: true,
      rightRootAvailable: true,
      linkedNavigation: true,
    })).toEqual({
      keepLink: false,
      keepFollow: false,
      keepScroll: false,
      message: 'left pane location unavailable',
    });
  });

  it('disables only Link when persisted linked roots disappear', () => {
    expect(decidePaneHealth({
      leftAvailable: true,
      rightAvailable: true,
      leftRootAvailable: true,
      rightRootAvailable: false,
      linkedNavigation: true,
    })).toEqual({
      keepLink: false,
      keepFollow: true,
      keepScroll: true,
      message: 'right linked root unavailable',
    });
  });

  it('changes comparison identity when locations or visible counts change', () => {
    expect(comparisonSignature('C:/a', 'D:/b', 10, 10))
      .not.toBe(comparisonSignature('C:/a', 'D:/b', 11, 10));
    expect(comparisonSignature('C:/a', 'D:/b', 10, 10))
      .not.toBe(comparisonSignature('C:/a/x', 'D:/b', 10, 10));
  });
});