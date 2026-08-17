import { readFileSync } from 'node:fs';

describe('Reassembly build regression guards', () => {
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

  it('accepts Event from Angular HostListener and narrows it explicitly', () => {
    expect(app).toContain('onGlobalEscape(event: Event): void');
    expect(app).toContain('const keyboardEvent = event as KeyboardEvent');
  });

  it('does not reference an out-of-scope tabId in middle-click handling', () => {
    const start = pane.indexOf('onTabAuxClick(');
    const end = pane.indexOf('onTabStripDoubleClick', start);
    expect(pane.slice(start, end)).not.toContain('revealActiveTab(tabId)');
  });

  it('uses real PaneComparisonStatus discriminants', () => {
    expect(manager).toContain("item.status !== 'same'");
    expect(manager).toContain("case 'size-different':");
    expect(manager).toContain("case 'hash-different':");
    expect(manager).toContain("case 'content-different':");
    expect(manager).toContain("case 'type-different':");
    expect(manager).not.toContain("item.status !== 'equal'");
    expect(manager).not.toContain("case 'different':");
    expect(manager).not.toContain("case 'type-mismatch':");
  });

  it('coordinates linked navigation through a public pane method', () => {
    expect(pane).toContain('async navigateLinkedLocation(');
    expect(manager).toContain('targetPane.navigateLinkedLocation(peer');
    expect(manager).not.toContain('targetPane.load(peer');
  });
});
