import { readFileSync } from 'node:fs';

describe('Tools menu dialog visual family', () => {
  const targets = [
    './tools-center.component.ts',
    './operation-journal.component.ts',
    './selection-basket.component.ts',
    './disk-usage-dialog.component.ts',
    './batch-rename-dialog.component.ts',
    './closed-tabs.component.ts',
    './command-palette.component.ts',
    './file-diff-dialog.component.ts',
    './pane-compare-dialog.component.ts',
    './pane-recursive-compare-dialog.component.ts',
    './pane-sync-review.component.ts',
    './remote-connect-dialog.component.ts',
    './trash-dialog.component.ts',
  ];

  it('uses the shared Remote Connections-inspired dialog theme', () => {
    for (const target of targets) {
      const source = readFileSync(new URL(target, import.meta.url), 'utf8');
      expect(source).withContext(target).toContain('tools-dialog-remote-theme.scss');
    }
  });

  it('keeps the reference dialog unchanged and codifies its visual primitives', () => {
    const theme = readFileSync(new URL('./tools-dialog-remote-theme.scss', import.meta.url), 'utf8');
    expect(theme).toContain('padding: min(12vh, 110px) 12px 20px;');
    expect(theme).toContain('background: var(--vsc-scrim-soft);');
    expect(theme).toContain('border-radius: 8px;');
    expect(theme).toContain('background: var(--vsc-menu-background, var(--vsc-panel-background));');
    expect(theme).toContain('font: 12px/1.4 var(--vsc-font-family, system-ui, sans-serif);');
  });
});
