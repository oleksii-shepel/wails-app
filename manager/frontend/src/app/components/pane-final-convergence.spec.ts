import { readFileSync } from 'node:fs';

describe('final pane convergence', () => {
  const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
  const paneHost = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');

  it('renders PaneHostComponent directly', () => {
    expect(dual).toContain("import { PaneHostComponent } from './pane-host.component';");
    expect(dual).toContain('<app-pane-host');
    expect(dual).not.toContain('FilePaneComponent');
    expect(dual).not.toContain('<app-file-pane');
  });

  it('exposes the narrow shell capability ports from PaneHostComponent', () => {
    for (const port of ['navigation', 'selection', 'commands', 'tabCommands', 'viewerCommands', 'presentation', 'viewport']) {
      expect(paneHost).toContain(`get ${port}`);
    }
  });

  it('routes pane clipboard behavior through the shared controller', () => {
    const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
    const bin = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');
    for (const pane of [directory, bin]) {
      expect(pane).toContain('PaneClipboardController');
      expect(pane).toContain('this.clipboardCommands.copy(this.clipboardHost)');
    }
  });
});
