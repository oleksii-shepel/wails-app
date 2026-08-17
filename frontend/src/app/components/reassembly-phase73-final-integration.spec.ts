import { readFileSync } from 'node:fs';

describe('Phase 73 final integration', () => {
  const app = readFileSync(
    new URL('../app.component.ts', import.meta.url),
    'utf8',
  );
  const pane = readFileSync(
    new URL('./reassembly-directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const manager = readFileSync(
    new URL('./reassembly-dual-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('keeps the previously fixed production-build regressions closed', () => {
    expect(app).toContain('onGlobalEscape(event: Event): void');
    expect(manager).not.toContain("item.status !== 'equal'");
    expect(manager).not.toContain("case 'different':");
    expect(manager).not.toContain("case 'type-mismatch':");
    expect(manager).not.toContain('targetPane.load(peer');

    const start = pane.indexOf('onTabAuxClick(');
    const end = pane.indexOf('onTabStripDoubleClick', start);
    expect(pane.slice(start, end)).not.toContain('revealActiveTab(tabId)');
  });

  it('contains each Phase 73 high-frequency shortcut exactly once', () => {
    for (const id of [
      'reassembly.panes.link.ctrlAltL',
      'reassembly.panes.mirror.ctrlAltM',
      'reassembly.panes.follow.ctrlAltF',
      'reassembly.panes.scroll.ctrlAltS',
      'reassembly.panes.previousDifference.ctrlAltBracketLeft',
      'reassembly.panes.nextDifference.ctrlAltBracketRight',
    ]) {
      expect(manager.split(id).length - 1).toBe(1);
    }
  });

  it('does not render per-row comparison decoration', () => {
    expect(pane).not.toContain('compare-badge');
    expect(pane).not.toContain('compareOverlay');
    expect(manager).not.toContain('[compareOverlay]');
  });

  it('retains difference navigation without row-decoration plumbing', () => {
    expect(manager).toContain('navigateDifference(delta: -1 | 1)');
    expect(manager).toContain('comparisonIsCurrent()');
    expect(manager).toContain('invalidatePaneComparison');
  });

  it('retains persisted pane intelligence and command integration', () => {
    expect(manager).toContain('persistPaneIntelligence()');
    expect(manager).toContain('restorePaneIntelligenceForWorkspace');
    expect(manager).toContain('chooseAndApplyPanePreset()');
    expect(manager).toContain('COMMAND.togglePaneLink');
    expect(manager).toContain('COMMAND.copyToPeer');
  });

  it('retains defensive location-health recovery', () => {
    expect(manager).toContain('ensurePaneIntelligenceHealth');
    expect(manager).toContain('paneIntelligenceEpoch');
    expect(pane).toContain('currentLocationAvailable()');
    expect(pane).toContain('recoverVisibleCursor()');
  });
  it('keeps cursor recovery typed as FileInfo instead of string-or-entry', () => {
    expect(pane).toContain('const fallback: FileInfo =');
    expect(pane).toContain('this.cursorPath !== null');
    expect(pane).not.toContain('this.cursorPath &&\n      visible.find');
  });

});
