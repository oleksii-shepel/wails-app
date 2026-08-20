import { DirectoryPaneComponent } from './directory-pane.component';
import { markerForSide } from './correspondence-policy';

describe('DirectoryPane comparison markers', () => {
  function component(side: 'left' | 'right'): DirectoryPaneComponent {
    const value = Object.create(DirectoryPaneComponent.prototype) as DirectoryPaneComponent;
    value.paneId = side;
    return value;
  }

  it('maps newer status relative to the current pane side', () => {
    const left = component('left');
    left.comparisonStatuses = new Map([['L/a', 'left-newer'] as const]);

    const right = component('right');
    right.comparisonStatuses = new Map([['R/a', 'left-newer'] as const]);

    expect(markerForSide(left.comparisonStatus('L/a')!, left.paneId)).toBe('newer');
    expect(markerForSide(right.comparisonStatus('R/a')!, right.paneId)).toBe('older');
  });

  it('marks only-here entries on the side where they exist', () => {
    const left = component('left');
    left.comparisonStatuses = new Map([['L/a', 'only-left'] as const]);
    expect(markerForSide(left.comparisonStatus('L/a')!, left.paneId)).toBe('only-here');
  });

  it('distinguishes type mismatch from ordinary difference', () => {
    const pane = component('left');
    pane.comparisonStatuses = new Map([
      ['L/type', 'type-different'],
      ['L/size', 'size-different'],
    ] as const);

    expect(pane.comparisonStatus('L/type')).toBe('type-different');
    expect(pane.comparisonStatus('L/size')).toBe('size-different');
    expect(pane.isComparisonDifferent('L/type')).toBe(true);
    expect(pane.isComparisonDifferent('L/size')).toBe(true);
  });
});
