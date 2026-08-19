import { readFileSync } from 'node:fs';

describe('DualPaneComponent shortcut ownership', () => {
  const source = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

  it('does not keep Ctrl+/ as an alternate shortcut-reference binding', () => {
    expect(source).not.toContain("id:'reassembly.help.shortcutReference.ctrlSlash'");
    expect(source).not.toContain("id:'reassembly.help.shortcutReference.ctrlShiftSlash'");
  });

  it('mounts the shortcut reference independently from the input center', () => {
    expect(source).toContain('<app-keyboard-help #keyboardHelp></app-keyboard-help>');
    expect(source).toContain('keyboardHelp?: KeyboardHelpComponent');
    expect(source).toContain("descriptor(COMMAND.showShortcutReference, 'Shortcut Reference', 'Help', () => this.showShortcutReference())");
    expect(source).toContain('this.shortcutReferenceVisible = !this.shortcutReferenceVisible');
    expect(source).toContain('[visible]="shortcutHelpVisible"');
  });

  it('keeps the command palette global and API-compatible', () => {
    expect(source).toContain("id:'reassembly.navigation.commandPalette.ctrlShiftP'");
    expect(source).toContain("contextId:'global'");
    expect(source).toContain('this.openCommandPaletteSurface()');
    expect(source).toContain("typeof palette.show === 'function'");
  });

  it('restores F1 to the compact shortcut reference and moves Input & Shortcuts to Ctrl+Alt+K', () => {
    expect(source).toContain("id:'reassembly.help.shortcutReference.f1'");
    expect(source).toContain("commandId:COMMAND.showShortcutReference, contextId:'global'");
    expect(source).toContain("id:'reassembly.help.inputCenter.ctrlAltK'");
    expect(source).toContain("commandId:COMMAND.toggleShortcutHelp, contextId:'global'");
    expect(source).toContain("key:'k', modifiers:['alt','ctrl']");
    expect(source).toContain("descriptor(COMMAND.toggleShortcutHelp, 'Input & Shortcuts', 'Help'");
    expect(source).not.toContain("id:'reassembly.help.shortcuts.f1'");
  });

  it('does not assign Ctrl+Shift+H to two manager commands', () => {
    expect(source).toContain("id:'reassembly.workspace.toggleHidden.ctrlShiftH'");
    expect(source).toContain("id:'reassembly.edit.operationHistory.ctrlShiftH'");
    expect(source).toContain("key:'h', modifiers:['ctrl','alt']");
    expect((source.match(/key:'h', modifiers:\['ctrl','shift'\]/g) ?? []).length).toBe(1);
  });

  it('makes viewer and tree arrow ownership mutually exclusive', () => {
    expect(source).toContain("when:'viewerSurface'");
    expect(source).toContain("when:'!typing && !viewerSurface'");
  });

});
