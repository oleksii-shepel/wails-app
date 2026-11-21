import { FileType } from '../shared/protocol';
import type { PaneComparisonResult } from '../services/pane-comparison.service';
import type { PaneSyncPlan } from '../services/pane-sync-review-dialog.service';
import { PaneSyncReviewDialogService } from '../services/pane-sync-review-dialog.service';
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

function mountComponent(testResult: PaneComparisonResult) {
  const dialogs = new PaneSyncReviewDialogService();
  const plans: PaneSyncPlan[] = [];
  dialogs.openDialog({
    getResult: () => testResult,
    getHistory: () => [],
    onExecute: (plan) => { plans.push(plan); },
  });
  const component = new PaneSyncReviewComponent(dialogs);
  component.ngDoCheck();
  return { component, plans };
}

describe('PaneSyncReviewComponent plan defaults', () => {
  it('auto-approves missing items but skips overwrite candidates', () => {
    const { component } = mountComponent(result());

    expect(component.items.map(item => item.direction)).toEqual(['left-to-right', 'right-to-left', 'skip']);
  });

  it('emits only explicitly approved directions', () => {
    const { component, plans } = mountComponent(result());
    component.setDirection(component.items[2], 'left-to-right');

    component.executePlan();

    expect(plans).toHaveSize(1);
    expect(plans[0]).toEqual(jasmine.objectContaining({
      leftToRight: ['/left/a.txt', '/left/c.txt'],
      rightToLeft: ['/right/b.txt'],
      preset: 'custom',
    }));
  });

  it('builds an explicit bidirectional plan from one-sided and newer-side differences', () => {
    const { component } = mountComponent(result());

    component.applyPreset('bidirectional');

    expect(component.items.map(item => item.direction)).toEqual(['left-to-right', 'right-to-left', 'left-to-right']);
    expect(component.preset).toBe('bidirectional');
  });

  it('keeps ambiguous content differences as conflicts in bidirectional mode', () => {
    const base = result();
    const left = base.items[2].left!;
    const right = base.items[2].right!;
    const { component } = mountComponent({
      items: [{ key: 'c', name: 'c.txt', status: 'content-different', left, right }],
      summary: { total: 1, same: 0, onlyLeft: 0, onlyRight: 0, leftNewer: 0, rightNewer: 0, sizeDifferent: 0, hashDifferent: 0, contentDifferent: 1, typeDifferent: 0, different: 1 },
    });

    component.applyPreset('bidirectional');

    expect(component.conflictCount).toBe(1);
    expect(component.items[0].direction).toBe('skip');
  });
});
