import { readFileSync } from 'node:fs';

const directoryPane = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
const filePane = readFileSync(new URL('./file-pane.component.ts', import.meta.url), 'utf8');
const controller = readFileSync(new URL('./pane-clipboard.controller.ts', import.meta.url), 'utf8');

describe('pane clipboard parity', () => {
  it('centralizes copy, cut, and paste behavior', () => {
    expect(controller).toContain('export class PaneClipboardController');
    expect(controller).toContain('copy(host: PaneClipboardHost)');
    expect(controller).toContain('cut(host: PaneClipboardHost)');
    expect(controller).toContain('async paste(host: PaneClipboardHost)');
  });

  it('uses the shared controller from both pane implementations', () => {
    expect(directoryPane).toContain('this.clipboardCommands.copy(this.clipboardHost)');
    expect(directoryPane).toContain('this.clipboardCommands.cut(this.clipboardHost)');
    expect(directoryPane).toContain('this.clipboardCommands.paste(this.clipboardHost)');
    expect(filePane).toContain('this.clipboardCommands.copy(this.clipboardHost)');
    expect(filePane).toContain('this.clipboardCommands.cut(this.clipboardHost)');
    expect(filePane).toContain('this.clipboardCommands.paste(this.clipboardHost)');
  });
});