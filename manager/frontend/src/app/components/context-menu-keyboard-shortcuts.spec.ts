import { readFileSync } from 'node:fs';

const dual = readFileSync(
  new URL('./dual-pane.component.ts', import.meta.url),
  'utf8',
);
const host = readFileSync(
  new URL('./tool-workspace-host.component.ts', import.meta.url),
  'utf8',
);
const paneHost = readFileSync(
  new URL('./pane-host.component.ts', import.meta.url),
  'utf8',
);

describe('context menu keyboard shortcuts', () => {
  it('uses an app-owned Ctrl+. shortcut for the main pane context menu', () => {
    expect(dual).toContain("openContextMenu: 'reassembly.manager.openContextMenu'");
    expect(dual).toContain("id:'reassembly.manager.contextMenu.ctrlPeriod'");
    expect(dual).toContain("steps:[{key:'.', modifiers:['ctrl']}]");
    expect(dual).toContain('openActiveContextMenu(): void');
    expect(dual).toContain("descriptor(COMMAND.openContextMenu, 'Open Context Menu'");
  });

  it('routes keyboard-opened pane menus through the active pane anchor helper', () => {
    expect(dual).toContain('const anchor = pane.getKeyboardContextMenuAnchor?.() ?? { x: 0, y: 0 };');
    expect(dual).toContain("const event = new MouseEvent('contextmenu'");
    expect(dual).toContain('this.openEntryMenu(this.activePaneId, event, entry);');
    expect(dual).toContain('this.openBackgroundMenu(this.activePaneId, event);');
    expect(paneHost).toContain('getKeyboardContextMenuAnchor(): { x: number; y: number } | null');
  });

  it('avoids the Windows-owned F10 family in the Details workspace binding', () => {
    expect(host).toContain("id: 'reassembly.workspace.details.rowMenu.ctrlPeriod'");
    expect(host).not.toContain("rowMenu.shiftF10");
    expect(host).not.toContain("rowMenu.menuKey");
  });
});
