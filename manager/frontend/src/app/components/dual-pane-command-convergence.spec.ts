import { readFileSync } from 'node:fs';

const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

describe('dual-pane shared command ownership', () => {
  it('owns the shared file-manager command ids used by FilePaneComponent', () => {
    expect(dual).toContain("import { FILE_MANAGER_COMMANDS } from './file-manager-command-registry'");

    for (const command of [
      'openCommandPalette',
      'navigateUp',
      'refresh',
      'cursorUp',
      'cursorDown',
      'openItem',
      'selectAll',
      'deselectAll',
      'newFolder',
      'rename',
      'copy',
      'cut',
      'paste',
      'deleteSelected',
      'copyToOtherPane',
      'moveToOtherPane',
      'newTab',
      'closeTab',
      'nextTab',
      'prevTab',
      'togglePinTab',
      'quickView',
      'openWithExternal',
    ]) {
      expect(dual).toContain(`FILE_MANAGER_COMMANDS.${command}`);
    }
  });

  it('keeps pane-intelligence commands manager-specific', () => {
    expect(dual).toContain("togglePaneLink: 'reassembly.panes.toggleLink'");
    expect(dual).toContain("togglePaneMirror: 'reassembly.panes.toggleMirror'");
    expect(dual).toContain("togglePaneFollow: 'reassembly.panes.toggleFollow'");
  });
});
