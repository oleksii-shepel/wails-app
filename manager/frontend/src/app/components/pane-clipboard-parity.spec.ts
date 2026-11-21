import { readFileSync } from 'node:fs';

const directoryPane = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
const binPane = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');
const controller = readFileSync(new URL('./pane-clipboard.controller.ts', import.meta.url), 'utf8');

describe('pane clipboard parity', () => {
  it('centralizes copy, cut, and paste behavior', () => {
    expect(controller).toContain('export class PaneClipboardController');
    expect(controller).toContain('copy(host: PaneClipboardHost)');
    expect(controller).toContain('cut(host: PaneClipboardHost)');
    expect(controller).toContain('async paste(host: PaneClipboardHost)');
  });

  it('uses the shared controller from the pane implementations', () => {
    for (const pane of [directoryPane, binPane]) {
      expect(pane).toContain('this.clipboardCommands.copy(this.clipboardHost)');
      expect(pane).toContain('this.clipboardCommands.cut(this.clipboardHost)');
      expect(pane).toContain('this.clipboardCommands.paste(this.clipboardHost)');
    }
  });
});
