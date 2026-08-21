import { readFileSync } from 'node:fs';

const menu = readFileSync(
  new URL('./application-menu-bar.component.ts', import.meta.url),
  'utf8',
);
const commands = readFileSync(
  new URL('./file-manager-command-registry.ts', import.meta.url),
  'utf8',
);
const dual = readFileSync(
  new URL('./dual-pane.component.ts', import.meta.url),
  'utf8',
);

describe('application menu dialog coverage', () => {
  const userOpenable = [
    'Command Palette…',
    'Global Search…',
    'Location History…',
    'Quick Access…',
    'Pinned Locations…',
    'Workspace Switcher…',
    'Operations Center…',
    'Selection Basket…',
    'Operation History…',
    'Recently Closed Tabs…',
    'Remote Connections…',
    'Connect Remote…',
    'Trash / Recycle Bin…',
    'Batch Rename…',
    'Keyboard Shortcuts…',
    'Input & Shortcuts…',
    'Plugin Settings…',
    'Settings…',
  ];

  it('surfaces every user-openable custom dialog/center in the main menu', () => {
    for (const label of userOpenable) {
      expect(menu).withContext(label).toContain(label);
    }
  });

  it('keeps transactional dialogs out of the application menu', () => {
    for (const label of [
      'Confirm Dialog',
      'Prompt',
      'Archive Move Warning',
    ]) {
      expect(menu).not.toContain(label);
    }
  });

  it('uses stable command IDs rather than injecting dialog services into the menu bar', () => {
    expect(menu).toContain('FILE_MANAGER_COMMANDS.openRemoteConnections');
    expect(menu).toContain('FILE_MANAGER_COMMANDS.openInputCenter');
    expect(menu).toContain('FILE_MANAGER_COMMANDS.openPluginSettings');
    expect(menu).not.toContain('DialogService');
  });

  it('adds the missing standalone Remote Connections command', () => {
    expect(commands).toContain(
      "openRemoteConnections:'reassembly.navigation.remoteConnections'",
    );
    expect(dual).toContain(
      'openRemoteConnections: FILE_MANAGER_COMMANDS.openRemoteConnections',
    );
    expect(dual).toContain(
      "descriptor(\n      COMMAND.openRemoteConnections,\n      'Remote Connections'",
    );
  });
});
