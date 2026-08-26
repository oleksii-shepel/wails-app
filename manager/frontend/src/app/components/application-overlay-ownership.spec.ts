import { readFileSync } from 'node:fs';

const app = readFileSync(new URL('../app.component.ts', import.meta.url), 'utf8');
const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

const globalOverlays = [
  'app-command-palette',
  'app-global-search',
  'app-tools-center',
  'app-disk-usage-dialog',
  'app-selection-basket',
  'app-workspace-switcher',
  'app-path-history',
  'app-quick-access',
  'app-remote-connections',
  'app-closed-tabs',
  'app-operation-journal',
  'app-keyboard-help',
  'app-input-center',
];

describe('application overlay ownership', () => {
  it('mounts user-openable overlays at the application shell', () => {
    for (const selector of globalOverlays) {
      expect(app).withContext(selector).toContain(`<${selector}`);
    }
  });

  it('keeps the pane manager free of global dialog ownership', () => {
    for (const selector of globalOverlays) {
      expect(dual).withContext(selector).not.toContain(`<${selector}`);
    }
  });

  it('keeps pane-local context menus inside DualPane', () => {
    expect(dual).toContain('<app-context-menu');
  });
});
