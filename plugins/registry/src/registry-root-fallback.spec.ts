import { RegistryPaneComponent } from './registry-pane.component';

describe('RegistryPane root availability', () => {
  it('shows the five Windows root hives while the provider root listing settles', () => {
    const pane = new RegistryPaneComponent();
    pane.path = 'registry://';
    pane.entries = [];

    expect(pane.rootEntries.map(entry => entry.name)).toEqual([
      'HKEY_CLASSES_ROOT',
      'HKEY_CURRENT_CONFIG',
      'HKEY_CURRENT_USER',
      'HKEY_LOCAL_MACHINE',
      'HKEY_USERS',
    ]);
    expect(pane.effectiveVisibleEntryCount).toBe(5);
  });

  it('uses provider entries once they arrive', () => {
    const pane = new RegistryPaneComponent();
    pane.path = 'registry://';
    pane.entries = [{
      name: 'HKEY_USERS',
      path: 'registry://HKEY_USERS',
      type: 'DIRECTORY',
      size: 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
      registryKind: 'Key',
    }];

    expect(pane.rootEntries).toBe(pane.entries);
  });
});