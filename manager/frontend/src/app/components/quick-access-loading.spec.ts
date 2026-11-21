import '@angular/compiler';
import { QuickAccessComponent } from './quick-access.component';

function setup(getSpecialFolders: () => Promise<unknown[]> = async () => []) {
  const component = Object.create(QuickAccessComponent.prototype) as QuickAccessComponent;
  Object.assign(component, {
    api: { getSpecialFolders, getDrives: async () => ({ schemes: [] }) },
    history: { getAllHistory: () => [] },
    pinnedLocations: { load: () => [
      { id: 'dir-pin', label: 'Files', location: '/work', surfaceId: 'dir' },
      { id: 'tool-pin', label: 'Tools', location: '/work', surfaceId: 'tool' },
    ] },
    connections: { refresh: async () => [], mounts: [] },
    cdr: { markForCheck() {} },
    input: { activateContext() {}, deactivateContext() {} },
    loadRevision: 0,
    currentPath: '',
    sections: [],
    filterText: '',
  });
  return component;
}

describe('Quick Access loading', () => {
  it('shows pinned surfaces in their own section even when they appear under Smart', async () => {
    const component = setup();
    await component.open('pins');

    expect(component.activeSection?.id).toBe('pins');
    expect(component.activeItems.map(item => item.id)).toEqual(['pin:dir-pin', 'pin:tool-pin']);
  });

  it('does not repopulate the dialog after closing during a request', async () => {
    let resolve!: (folders: unknown[]) => void;
    const component = setup(() => new Promise(done => { resolve = done; }));
    const open = component.open();
    expect(component.loading).toBeTrue();
    component.close();
    resolve([]);
    await open;

    expect(component.visible).toBeFalse();
    expect(component.loading).toBeFalse();
    expect(component.sections).toEqual([]);
  });
});
