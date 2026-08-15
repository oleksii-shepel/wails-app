import { describe, expect, it, vi } from 'vitest';
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
    summary: { total: 3, same: 0, onlyLeft: 1, onlyRight: 1, leftNewer: 1, rightNewer: 0, sizeDifferent: 0, typeDifferent: 0, different: 3 },
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
    const emit = vi.spyOn(component.execute, 'emit');

    component.executePlan();

    expect(emit).toHaveBeenCalledWith(expect.objectContaining({
      leftToRight: ['/left/a.txt', '/left/c.txt'],
      rightToLeft: ['/right/b.txt'],
    }));
  });
});
