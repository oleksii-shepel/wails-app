import { readFileSync } from 'node:fs';

describe('PanelSourcePaneSession return contracts', () => {
  const source = readFileSync(
    new URL('./panel-source-pane-session.ts', import.meta.url),
    'utf8',
  );

  it('keeps navigateTo as Promise<void> while load returns success', () => {
    expect(source).toContain('async navigateTo(path: string): Promise<void>');
    expect(source).toContain("await this.load(path, { history: 'record', preserveSelection: false });");
    expect(source).toContain('): Promise<boolean> {');
  });
});