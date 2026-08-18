import { readFileSync } from 'node:fs';

describe('Phase 78 Workspace & Session Resilience', () => {
  const manager = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const workspace = readFileSync(
    new URL('../services/workspace.service.ts', import.meta.url),
    'utf8',
  );

  it('adds Ctrl+Shift+T reopen', () => {
    expect(manager).toContain('reassembly.manager.reopenClosedTab.ctrlShiftT');
    expect(manager).toContain("'Reopen Closed Tab'");
  });

  it('persists recently closed tabs by workspace', () => {
    expect(workspace).toContain('RECENTLY_CLOSED_TABS_KEY');
    expect(workspace).toContain('workspaceId');
    expect(workspace).toContain('persistRecentlyClosedTabs()');
  });

  it('restores arbitrary older entries from the recovery browser', () => {
    expect(workspace).toContain('reopenClosedTabById(');
    expect(manager).toContain('reopenClosedTabEntry(');
  });

  it('restores full TabInfo instead of only a filesystem path', () => {
    expect(workspace).toContain('tab: this.cloneTab(tab)');
    expect(workspace).toContain('this.cloneTab(entry.tab)');
  });

  it('keeps the unwanted outer active pane border removed', () => {
    expect(pane).not.toContain('.pane.active-pane');
  });
});
