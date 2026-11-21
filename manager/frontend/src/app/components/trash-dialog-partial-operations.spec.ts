import '@angular/compiler';
import type { TrashEntry } from '../services/api.service';
import { TrashDialogComponent } from './trash-dialog.component';

describe('Trash dialog partial operations', () => {
  it('refreshes after a partial restore and leaves failed items selected', async () => {
    const entries = ['good', 'blocked', 'later'].map(id =>
      ({ id, name: id, canRestore: true } as TrashEntry),
    );
    const restore = jasmine.createSpy('restore').and.callFake(async (id: string) => {
      if (id === 'blocked') throw new Error('Destination exists');
      return '/restored/' + id;
    });
    const notifyMutated = jasmine.createSpy('notifyMutated');
    const component = Object.create(TrashDialogComponent.prototype) as TrashDialogComponent;
    Object.assign(component, {
      api: {
        restoreTrashEntry: restore,
        listTrashEntries: async () => [entries[1]],
      },
      cdr: { markForCheck() {} },
      dialog: { activeRequest$: { value: {} }, notifyMutated },
      entries,
      selectedIds: new Set(entries.map(entry => entry.id)),
      listingRevision: 0,
      loading: false,
      confirmationPending: false,
    });

    await component.restoreSelected();

    expect(restore.calls.allArgs()).toEqual([['good'], ['blocked'], ['later']]);
    expect(notifyMutated).toHaveBeenCalledTimes(1);
    expect(component.entries.map(entry => entry.id)).toEqual(['blocked']);
    expect([...component.selectedIds]).toEqual(['blocked']);
    expect(component.error).toContain('Could not restore 1 of 3');
    expect(component.loading).toBeFalse();
  });
});
