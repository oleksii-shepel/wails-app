import { readFileSync } from 'node:fs';

describe('Reassembly pane intelligence persistence integration', () => {
  const manager = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('restores intelligence after workspace pane state', () => {
    expect(manager).toContain(
      'this.restorePaneIntelligenceForWorkspace(workspace.id)',
    );
  });

  it('persists mode changes and linked-root swaps', () => {
    expect(manager).toContain('this.persistPaneIntelligence()');
    expect(manager).toContain("this.linkStatus = 'linked roots swapped'");
  });

  it('probes both preset roots before changing either pane', () => {
    expect(manager).toContain('leftPane.canNavigateToLocation(preset.roots.left)');
    expect(manager).toContain('rightPane.canNavigateToLocation(preset.roots.right)');
  });
});