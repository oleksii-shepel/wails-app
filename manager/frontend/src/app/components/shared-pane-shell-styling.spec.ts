import { readFileSync } from 'node:fs';

const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
const tool = readFileSync(new URL('./tool-workspace-host.component.ts', import.meta.url), 'utf8');
const shared = readFileSync(new URL('./pane-shell.shared.css', import.meta.url), 'utf8');

describe('shared pane shell styling', () => {
  it('uses one frame stylesheet in main and Details panes', () => {
    expect(dual).toContain("styleUrls: ['./pane-shell.shared.css']");
    expect(directory).toContain("styleUrls: ['./pane-shell.shared.css']");
    expect(tool).toContain("styleUrls: ['./pane-shell.shared.css']");
  });

  it('maps main panes onto the shared slot/inner-pane contract', () => {
    expect(dual).toContain('pane-slot pane-shell-slot left-pane-slot');
    expect(dual).toContain('pane-slot pane-shell-slot right-pane-slot');
    expect(directory).toContain('class="pane pane-shell"');
    expect(directory).toContain('[class.pane-shell-active]="active"');
  });

  it('maps Details onto exactly the same slot/inner-pane contract', () => {
    expect(tool).toContain('[class.pane-shell-layout]="isDetailsWorkspaceSplit(layout)"');
    expect(tool).toContain('[class.pane-shell-slot]="isDetailsWorkspaceSplit(layout)"');
    expect(tool).toContain('[class.pane-shell]="isDetailsWorkspaceSurface(surface)"');
    expect(tool).toContain('[class.pane-shell-active]="isActiveDetailsWorkspaceSurface(surface)"');
    expect(tool).toContain('[class.pane-shell-header]="isDetailsWorkspaceSurface(surface)"');
  });

  it('keeps the main visual constants in the shared contract', () => {
    expect(shared).toContain('border-radius: 8px;');
    expect(shared).toContain('0 10px 24px');
    expect(shared).toContain('62%');
    expect(shared).toContain('min-height: 31px;');
    expect(shared).toContain('var(--vsc-tabbar-background, var(--vsc-panel-background))');
  });

  it('keeps header action buttons flat', () => {
    expect(shared).toContain('.pane-shell-header.pane-shell-header button {');
    expect(shared).toContain('border: 0;');
    expect(shared).toContain('background: transparent;');
  });
});
