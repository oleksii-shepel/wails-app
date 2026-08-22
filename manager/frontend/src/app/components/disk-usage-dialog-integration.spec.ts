import { readFileSync } from 'node:fs';

const dualPane = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const dialog = readFileSync(new URL('./disk-usage-dialog.component.ts', import.meta.url), 'utf8');

describe('Disk Usage dialog integration', () => {
  it('routes the file-manager Disk Usage command to the dialog, not Tools Center', () => {
    expect(dualPane).toContain("descriptor(COMMAND.diskUsage, 'Disk Usage', 'Tools', () => this.openDiskUsageDialog()");
    expect(dualPane).not.toContain("descriptor(COMMAND.diskUsage, 'Disk Usage', 'Tools', () => this.openTools('disk')");
  });

  it('hosts the existing Size Manager implementation inside an app-level modal', () => {
    expect(dialog).toContain('<app-size-manager-surface [ctx]="ctx">');
    expect(dialog).toContain('role="dialog"');
    expect(dialog).toContain('aria-modal="true"');
  });

  it('destroys the utility context when the modal closes', () => {
    expect(dialog).toContain('this.ctx = null;');
    expect(dualPane).toContain('(closed)="closeDiskUsageDialog()"');
  });
});
