import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const appRoot = join(__dirname, '..');

const presentationFiles = [
  'components/pane-details.component.ts',
  'components/file-entry.component.ts',
  'components/directory-pane.component.ts',
  'components/registry-pane.component.ts',
  'components/autorun-pane.component.ts',
  'components/environment-pane.component.ts',
  'components/network-pane.component.ts',
  'components/bin-pane.component.ts',
  'components/dual-pane.component.ts',
  'components/pane-file-tools.controller.ts',
  'components/operation-center.component.ts',
  'components/operation-journal.component.ts',
  'components/closed-tabs.component.ts',
  'components/tools-center.component.ts',
  'components/path-history.component.ts',
  'components/path-history-viewer.component.ts',
  'components/quick-access.component.ts',
  'components/selection-basket.component.ts',
  'components/trash-dialog.component.ts',
  'components/plugin-settings-panel.component.ts',
  'services/plugin-manager.service.ts',
  'services/builtin-workspace-surface-actions.service.ts',
  'surfaces/builtins/duplicate-finder-surface-tab.component.ts',
];

describe('GUI local date/time architecture', () => {
  it('does not format GUI dates directly in presentation consumers', () => {
    for (const relative of presentationFiles) {
      const source = readFileSync(join(appRoot, relative), 'utf8');
      expect(source).not.toContain('.toLocaleDateString(');
      expect(source).not.toContain('.toLocaleTimeString(');
      expect(source).not.toContain("| date:");
    }
  });

  it('never prefills a local date/time editor from a UTC ISO string', () => {
    for (const relative of presentationFiles) {
      const source = readFileSync(join(appRoot, relative), 'utf8');
      expect(source).not.toContain("toISOString().slice(0, 19)");
    }
  });
});
