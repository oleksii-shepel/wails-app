import { DirectoryPaneComponent } from './directory-pane.component';

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

    expect(left.comparisonMarker('L/a')).toBe('newer');
    expect(right.comparisonMarker('R/a')).toBe('older');
  });

  it('marks only-here entries on the side where they exist', () => {
    const left = component('left');
    left.comparisonStatuses = new Map([['L/a', 'only-left'] as const]);
    expect(left.comparisonMarker('L/a')).toBe('only');
  });

  it('distinguishes type mismatch from ordinary difference', () => {
    const pane = component('left');
    pane.comparisonStatuses = new Map([
      ['L/type', 'type-different'],
      ['L/size', 'size-different'],
    ] as const);

    expect(pane.comparisonMarker('L/type')).toBe('type');
    expect(pane.comparisonMarker('L/size')).toBe('different');
  });
});
