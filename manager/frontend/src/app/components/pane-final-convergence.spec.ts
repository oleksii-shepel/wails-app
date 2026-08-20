import { readFileSync } from 'node:fs';

describe('final pane convergence', () => {
  const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
  const pane = readFileSync(new URL('./file-pane.component.ts', import.meta.url), 'utf8');

  it('renders FilePaneComponent directly', () => {
    expect(dual).toContain("import { FilePaneComponent } from './file-pane.component';");
    expect(dual).toContain('<app-file-pane');
    expect(dual).not.toContain('PaneHostComponent');
    expect(dual).not.toContain('<app-pane-host');
  });

  it('exposes the narrow shell capability ports from FilePaneComponent', () => {
    for (const port of ['navigation', 'selection', 'commands', 'tabCommands', 'viewerCommands', 'presentation', 'viewport']) {
      expect(pane).toContain(`get ${port}`);
    }
  });

  it('wires shared file tools into the surviving pane', () => {
    expect(pane).toContain('PaneFileToolsController');
    expect(pane).toContain('fileTools.calculateSelectedSize');
    expect(pane).toContain('fileTools.showSelectedProperties');
  });
});
