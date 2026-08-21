import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./network-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Network toolbar and columns', () => {
  it('uses Manager-owned Network toolbar actions', () => {
    expect(source).toContain('[actions]="networkToolbarActions"');
    for (const label of ['Open', 'Refresh', 'Copy location', 'Copy endpoint']) {
      expect(source).toContain(`label: '${label}'`);
    }
  });

  it('keeps toolbar text-only and left aligned', () => {
    expect(source).toContain('[showIcons]="false"');
    expect(source).toContain('align="left"');
  });

  it('does not append generic system columns to Network-owned columns', () => {
    expect(source).toContain('get pluginPanelColumns(): PanelColumnContribution[]');
    expect(source).toContain('Generic system-surface columns would duplicate');
    expect(source).toContain('return [];');
  });

  it('keeps plugin actions after Network built-ins', () => {
    expect(source).toContain('...this.pluginToolbarActions.map');
  });
});
