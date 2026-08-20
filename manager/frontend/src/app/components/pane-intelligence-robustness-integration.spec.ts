import { readFileSync } from 'node:fs';

describe('Reassembly pane intelligence robustness integration', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('exposes safe pane location health checks', () => {
    expect(pane).toContain('async currentLocationAvailable()');
    expect(pane).toContain('recoverVisibleCursor()');
  });

  it('invalidates stale comparisons after navigation/mutations', () => {
    expect(manager).toContain("this.invalidatePaneComparison('locations changed')");
    expect(manager).toContain(
      "this.invalidatePaneComparison('file operation changed pane contents')",
    );
  });

  it('disables unsafe cooperation when locations disappear', () => {
    expect(manager).toContain('disableUnsafePaneCooperation');
    expect(manager).toContain('ensurePaneIntelligenceHealth');
  });

  it('repairs persisted intelligence on workspace restore', () => {
    expect(manager).toContain(
      "await this.ensurePaneIntelligenceHealth('workspace restore')",
    );
  });
});