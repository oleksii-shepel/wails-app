import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./remote-connect-dialog.component.ts', import.meta.url),
  'utf8',
);

describe('Connect Remote dismissal', () => {
  it('does not block Escape, close-button, backdrop, or Cancel while connecting', () => {
    expect(source).toContain('(click)="onCancel()"');
    expect(source).toContain("event.key !== 'Escape'");
    expect(source).not.toMatch(/onCancel\(\): void \{\s*if \(this\.isConnecting\)/s);
  });

  it('invalidates an in-flight connection when the dialog is dismissed', () => {
    expect(source).toContain('private connectAttemptGeneration = 0;');
    expect(source).toContain('this.connectAttemptGeneration += 1;');
    expect(source).toContain('attemptGeneration !== this.connectAttemptGeneration || !this.isOpen');
  });

  it('rolls back a mount created after the dialog was dismissed', () => {
    expect(source).toContain('await this.apiService.remoteDisconnect(mount.mountPoint);');
  });

  it('still completes the queued-dialog closing animation', () => {
    expect(source).toContain('this.remoteConnectDialog.finishClose();');
  });
});
