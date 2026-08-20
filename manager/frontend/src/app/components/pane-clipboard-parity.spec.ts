import { readFileSync } from 'node:fs';

const filePane = readFileSync(new URL('./file-pane.component.ts', import.meta.url), 'utf8');
const controller = readFileSync(new URL('./pane-clipboard.controller.ts', import.meta.url), 'utf8');

describe('pane clipboard ownership', () => {
  it('centralizes copy, cut, and paste behavior', () => {
    expect(controller).toContain('export class PaneClipboardController');
    expect(controller).toContain('copy(host: PaneClipboardHost)');
    expect(controller).toContain('cut(host: PaneClipboardHost)');
    expect(controller).toContain('async paste(host: PaneClipboardHost)');
  });

  it('keeps clipboard behavior on the shared controller used by FilePaneComponent', () => {
    expect(filePane).toContain('this.clipboardCommands.copy(this.clipboardHost)');
    expect(filePane).toContain('this.clipboardCommands.cut(this.clipboardHost)');
    expect(filePane).toContain('this.clipboardCommands.paste(this.clipboardHost)');
  });
});
