import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';
import { FileType } from '../shared/protocol';
import type { PaneComparisonResult } from '../services/pane-comparison.service';
import { PaneSyncReviewComponent } from './pane-sync-review.component';

function result(): PaneComparisonResult {
  const file = (name: string, path: string) => ({
    name, path, type: FileType.FILE, size: 1, modified: 1, created: 1, accessed: 1, permissions: '', isHidden: false,
  });
  return {
    items: [
      { key: 'a', name: 'a.txt', status: 'only-left', left: file('a.txt', '/left/a.txt'), right: null },
      { key: 'b', name: 'b.txt', status: 'only-right', left: null, right: file('b.txt', '/right/b.txt') },
      { key: 'c', name: 'c.txt', status: 'left-newer', left: file('c.txt', '/left/c.txt'), right: file('c.txt', '/right/c.txt') },
    ],
    summary: { total: 3, same: 0, onlyLeft: 1, onlyRight: 1, leftNewer: 1, rightNewer: 0, sizeDifferent: 0, hashDifferent: 0, contentDifferent: 0, typeDifferent: 0, different: 3 },
  };
}

describe('PaneSyncReviewComponent plan defaults', () => {
  it('auto-approves missing items but skips overwrite candidates', () => {
    const component = new PaneSyncReviewComponent();
    component.result = result();
    component.ngOnChanges();

    expect(component.items.map(item => item.direction)).toEqual(['left-to-right', 'right-to-left', 'skip']);
  });

  it('emits only explicitly approved directions', () => {
    const component = new PaneSyncReviewComponent();
    component.result = result();
    component.ngOnChanges();
    component.setDirection(component.items[2], 'left-to-right');
    const emit = spyOnFn(component.execute, 'emit');

    component.executePlan();

    expect(emit).toHaveBeenCalledWith(expect.objectContaining({
      leftToRight: ['/left/a.txt', '/left/c.txt'],
      rightToLeft: ['/right/b.txt'],
      preset: 'custom',
    }));
  });
});

  it('builds an explicit bidirectional plan from one-sided and newer-side differences', () => {
    const component = new PaneSyncReviewComponent();
    component.result = result();
    component.ngOnChanges();

    component.applyPreset('bidirectional');

    expect(component.items.map(item => item.direction)).toEqual(['left-to-right', 'right-to-left', 'left-to-right']);
    expect(component.preset).toBe('bidirectional');
  });

  it('keeps ambiguous content differences as conflicts in bidirectional mode', () => {
    const component = new PaneSyncReviewComponent();
    const base = result();
    const left = base.items[2].left!;
    const right = base.items[2].right!;
    component.result = {
      items: [{ key: 'c', name: 'c.txt', status: 'content-different', left, right }],
      summary: { total: 1, same: 0, onlyLeft: 0, onlyRight: 0, leftNewer: 0, rightNewer: 0, sizeDifferent: 0, hashDifferent: 0, contentDifferent: 1, typeDifferent: 0, different: 1 },
    };
    component.ngOnChanges();

    component.applyPreset('bidirectional');

    expect(component.conflictCount).toBe(1);
    expect(component.items[0].direction).toBe('skip');
  });
