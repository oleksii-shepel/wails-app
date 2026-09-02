import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./tool-workspace-host.component.ts', import.meta.url),
  'utf8',
);

describe('Details workspace pane styling', () => {
  it('styles Tree and Details as pane surfaces rather than cards', () => {
    expect(source).toContain('[class.details-workspace-pane-surface]="isDetailsWorkspaceSurface(surface)"');
    expect(source).toContain('.details-workspace-pane-surface {');
    expect(source).toContain('border-radius: 0;');
    expect(source).toContain('background: var(--vsc-editor-background, var(--vsc-panel-background));');
  });

  it('uses the same blue active-pane palette as the main window', () => {
    expect(source).toContain('.details-workspace-pane-surface.workspace-pane-active {');
    expect(source).toContain('var(--vsc-accent-blue, var(--vsc-focus-border)) 62%');
    expect(source).toContain('var(--vsc-foreground-dim, var(--vsc-border-bright)) 22%');
  });

  it('uses a one-pixel pane separator instead of card spacing', () => {
    expect(source).toContain('.split.details-workspace-split {');
    expect(source).toContain('gap: 1px;');
    expect(source).toContain('padding: 0;');
    expect(source).toContain('background: var(--vsc-border, var(--vsc-control-background));');
  });

  it('keeps focus on the pane state instead of drawing a second focus rectangle', () => {
    expect(source).toContain('[data-details-workspace-pane] {');
    expect(source).toContain('outline: none;');
    expect(source).not.toContain('[data-details-workspace-pane]:focus-visible,');
  });

  it('uses the main tab-strip palette for Details pane headers', () => {
    expect(source).toContain('.details-workspace-pane-surface > .surface-header {');
    expect(source).toContain('background: var(--vsc-tabbar-background, var(--vsc-panel-background));');
    expect(source).toContain('border-bottom-color: var(--vsc-border, var(--vsc-control-background));');
  });
});
