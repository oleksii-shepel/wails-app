import { readFileSync } from 'node:fs';

describe('pane intelligence integration', () => {
  const dual = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('binds comparison status maps into both panes', () => {
    expect(dual).toContain('[comparisonStatuses]="leftComparisonStatuses"');
    expect(dual).toContain('[comparisonStatuses]="rightComparisonStatuses"');
  });

  it('exposes Compare as a first-class pane intelligence mode', () => {
    expect(dual).toContain('(click)="togglePaneComparison()"');
    expect(dual).toContain('[class.active]="comparisonVisible"');
  });

  it('retains linked navigation, follow selection, mirror and synchronized scrolling', () => {
    expect(dual).toContain('toggleLinkedNavigation()');
    expect(dual).toContain('toggleFollowSelection()');
    expect(dual).toContain('toggleMirrorPresentation()');
    expect(dual).toContain('toggleSyncScroll()');
    expect(dual).toContain('onLinkedPaneLocationChange');
    expect(dual).toContain('onLinkedPaneCursorChange');
    expect(dual).toContain('onPaneViewportChange');
  });

  it('renders side-aware difference markers in list/tree rows', () => {
    expect(pane).toContain('comparisonMarker(entry.path)');
    expect(pane).toContain('comparisonMarker(row.entry.path)');
    expect(pane).toContain('comparison-marker');
    expect(pane).toContain("case 'left-newer'");
    expect(pane).toContain("case 'right-newer'");
  });
});
