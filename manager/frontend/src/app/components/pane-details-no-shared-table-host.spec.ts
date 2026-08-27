import { readFileSync } from 'node:fs';

describe('PaneDetails interaction isolation', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('does not mount the shared pane-table host or directives', () => {
    expect(source).not.toContain('reassembly-pane-table');
    expect(source).not.toContain('reassemblyPaneTable');
    expect(source).not.toContain('PaneTableComponent');
    expect(source).not.toContain('PaneTableBodyDirective');
  });

  it('does not mount the removed custom scrollbar', () => {
    expect(source).not.toContain('appVscodeScrollbar');
    expect(source).not.toContain('VscodeScrollbarDirective');
  });

  it('contains Details hit testing inside its own view', () => {
    expect(source).toContain('contain: layout paint;');
    expect(source).toContain('isolation: isolate;');
    expect(source).toContain('max-width: 100%;');
  });

  it('uses compact local header typography', () => {
    expect(source).toMatch(/&\.header-cell\s*\{[^}]*font-size:\s*10px/s);
  });
});
