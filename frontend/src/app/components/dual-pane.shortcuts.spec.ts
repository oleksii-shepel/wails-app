import { readFileSync } from 'node:fs';

describe('DualPaneComponent shortcut ownership', () => {
  const source = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

  it('owns Ctrl+/ in the live dual-pane shell', () => {
    expect(source).toContain("id:'reassembly.help.shortcutReference.ctrlSlash'");
    expect(source).toContain("commandId:COMMAND.showShortcutReference");
    expect(source).toContain("contextId:'global'");
    expect(source).toContain("key:'/', modifiers:['ctrl']");
    expect(source).toContain("id:'reassembly.help.shortcutReference.ctrlShiftSlash'");
  });

  it('mounts the shortcut reference independently from the F1 input center', () => {
    expect(source).toContain('<app-keyboard-help #keyboardHelp></app-keyboard-help>');
    expect(source).toContain('keyboardHelp?: KeyboardHelpComponent');
    expect(source).toContain("descriptor(COMMAND.showShortcutReference, 'Shortcut Reference', 'Help', () => this.keyboardHelp?.show())");
    expect(source).toContain('[visible]="shortcutHelpVisible"');
  });

  it('keeps the command palette global and API-compatible', () => {
    expect(source).toContain("id:'reassembly.navigation.commandPalette.ctrlShiftP'");
    expect(source).toContain("contextId:'global'");
    expect(source).toContain('this.openCommandPaletteSurface()');
    expect(source).toContain("typeof palette.show === 'function'");
  });

  it('keeps F1 and Ctrl+/ on separate commands', () => {
    expect(source).toContain("id:'reassembly.help.shortcuts.f1'");
    expect(source).toContain('commandId:COMMAND.toggleShortcutHelp');
    expect(source).toContain('commandId:COMMAND.showShortcutReference');
  });
});
