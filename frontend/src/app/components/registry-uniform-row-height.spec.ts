import { readFileSync } from 'node:fs';

describe('Registry list row height', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('uses the same two-line row height for keys and values', () => {
    expect(pane).toContain('[class.registry-surface]="isRegistrySurface"');
    expect(pane).toContain('.list.view-list.registry-surface .entry');
    expect(pane).toContain('min-height: 38px');
    expect(pane).toContain("{{ registryPayload(entry) || '\\u00a0' }}");
  });

  it('keeps the reserved second line visually empty for keys', () => {
    expect(pane).toContain('[class.registry-payload-empty]="!registryPayload(entry)"');
    expect(pane).toContain('.registry-payload-empty');
    expect(pane).toContain('visibility: hidden');
  });
});
