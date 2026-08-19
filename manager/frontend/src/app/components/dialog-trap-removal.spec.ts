import { readFileSync } from 'node:fs';

describe('dialog trap directive removal', () => {
  const files = [
    './confirm-dialog.component.ts',
    './settings-dialog.component.ts',
    './path-history-viewer.component.ts',
    './archive-move-warning.component.ts',
    './remote-connect-dialog.component.ts',
    './batch-rename-dialog.component.ts',
    './closed-tabs.component.ts',
    './operation-journal.component.ts',
    './remote-connections.component.ts',
    './tools-center.component.ts',
    './trash-dialog.component.ts',
    './workspace-switcher.component.ts',
    './path-history.component.ts',
    './quick-access.component.ts',
    './command-palette.component.ts',
    './global-search.component.ts',
    './input-center.component.ts',
    './keyboard-help.component.ts',
  ];

  it('does not reference focus-trap or ModalSurface directives', () => {
    for (const file of files) {
      const source = readFileSync(new URL(file, import.meta.url), 'utf8');
      expect(source).not.toContain('DialogFocusTrapDirective');
      expect(source).not.toContain('appTrapFocus');
      expect(source).not.toContain('ModalSurfaceDirective');
      expect(source).not.toContain('ModalSurface');
      expect(source).not.toContain('focus-trap.utils');
    }
  });
});
