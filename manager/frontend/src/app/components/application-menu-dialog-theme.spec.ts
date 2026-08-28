import { readFileSync } from 'node:fs';

const read = (name: string) => readFileSync(new URL(name, import.meta.url), 'utf8');

describe('application menu dialog theme convergence', () => {
  const paletteStyled = [
    './input-center.component.ts', './quick-access.component.ts', './workspace-switcher.component.ts',
    './disk-usage-dialog.component.ts', './batch-rename-dialog.component.ts', './archive-files-dialog.component.ts',
    './pane-compare-dialog.component.ts', './pane-recursive-compare-dialog.component.ts', './settings-dialog.component.ts',
    './path-history-viewer.component.ts',
  ];

  // Disk Usage hosts its close control inside the Size Manager surface,
  // so the standard-close-button check covers that surface file instead.
  const closeButtonOwners = paletteStyled
    .filter((file) => file !== './disk-usage-dialog.component.ts')
    .concat(['./../surfaces/size-manager/size-manager-surface.component.ts']);

  it('uses the Command Palette surface contract for menu-openable dialogs', () => {
    for (const file of paletteStyled) {
      const source = read(file);
      expect(source).withContext(file).toContain('palette-dialog-surface');
      expect(source).withContext(file).toContain('dialog-surface');
    }
  });

  it('uses the standard dialog close button wherever dialog chrome owns a close control', () => {
    for (const file of closeButtonOwners) {
      const source = read(file);
      expect(source).withContext(file).toContain('dialog-close');
    }
  });

  it('documents Escape, not F1, as the Keyboard Help close key', () => {
    const help = read('./keyboard-help.component.html');
    expect(help).toContain('<kbd>Esc</kbd> to close');
    expect(help).not.toContain('<kbd>F1</kbd> to close');
    expect(help).not.toContain('<kbd>F1</kbd> or <kbd>Esc</kbd> to close');
  });
});