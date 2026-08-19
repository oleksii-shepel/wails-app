import { readFileSync } from 'node:fs';

describe('Reassembly pane intelligence command integration', () => {
  const manager = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('registers all intelligence actions as commands', () => {
    for (const id of [
      'reassembly.panes.toggleLink',
      'reassembly.panes.toggleMirror',
      'reassembly.panes.toggleFilterMirror',
      'reassembly.panes.toggleScrollSync',
      'reassembly.panes.toggleFollow',
      'reassembly.panes.matchOther',
      'reassembly.panes.swapRoots',
      'reassembly.panes.openPeer',
      'reassembly.panes.copyToPeer',
      'reassembly.panes.moveToPeer',
      'reassembly.panes.previousDifference',
      'reassembly.panes.nextDifference',
      'reassembly.panes.savePreset',
      'reassembly.panes.applyPreset',
    ]) expect(manager).toContain(id);
  });

  it('adds shortcuts only for high-frequency actions', () => {
    expect(manager).toContain('reassembly.panes.link.ctrlAltL');
    expect(manager).toContain('reassembly.panes.mirror.ctrlAltM');
    expect(manager).toContain('reassembly.panes.follow.ctrlAltF');
    expect(manager).toContain('reassembly.panes.scroll.ctrlAltS');
    expect(manager).toContain('reassembly.panes.previousDifference.ctrlAltBracketLeft');
    expect(manager).toContain('reassembly.panes.nextDifference.ctrlAltBracketRight');
  });

  it('keeps peer operations capability-aware', () => {
    expect(manager).toContain('() => this.canOpenCurrentCounterpart');
    expect(manager).toContain('() => this.canCopyCurrentToPeer');
    expect(manager).toContain('() => this.canMoveCurrentToPeer');
  });

  it('allows pane-pair preset recall from the command palette', () => {
    expect(manager).toContain('chooseAndApplyPanePreset()');
    expect(manager).toContain("'Apply Pane Pair Preset…'");
  });
});