import { readFileSync } from 'node:fs';

const shared = readFileSync(new URL('./pane-shell.shared.css', import.meta.url), 'utf8');
const directory = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
const tool = readFileSync(new URL('./tool-workspace-host.component.ts', import.meta.url), 'utf8');

describe('inactive pane caption styling', () => {
  it('uses one disabled-caption rule for pane chrome', () => {
    expect(shared).toContain('.pane-shell-caption {');
    expect(shared).toContain('opacity: .68;');
    expect(shared).toContain('.tab.active .pane-shell-caption');
    expect(shared).toContain('.pane-shell.pane-shell-active > .pane-shell-header .pane-shell-caption');
  });

  it('applies the rule to main-window tab captions', () => {
    expect(directory).toContain('class="tab-title pane-shell-caption"');
  });

  it('applies the same rule to Details/tool pane captions', () => {
    expect(tool).toContain('<strong class="pane-shell-caption">');
    expect(tool).not.toContain('color: var(--vsc-description-foreground, var(--vsc-foreground-dim));\n      font-size: 11px;');
  });
});
