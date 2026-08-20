import { readFileSync } from 'node:fs';

describe('pane shell state ports', () => {
  const manager = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
  const filePane = readFileSync(new URL('./file-pane.component.ts', import.meta.url), 'utf8');
  const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');

  it('groups shell-facing state instead of expanding the flat pane facade', () => {
    expect(host).toContain('get contextState(): PaneContextState');
    expect(host).toContain('get tabState(): PaneTabState');
    expect(host).toContain('get viewerState(): PaneViewerState');
    expect(host).toContain('get pinState(): PanePinState');
    expect(host).toContain('get mutationState(): PaneMutationState');
  });

  it('makes FilePaneComponent expose the same narrow state ports', () => {
    expect(filePane).toContain('PaneStatePort');
    expect(filePane).toContain('get contextState(): PaneContextState');
    expect(filePane).toContain('get mutationState(): PaneMutationState');
  });

  it('moves dual-pane shell reads onto grouped state and selection ports', () => {
    expect(manager).toContain('contextState.isDirectoryTab');
    expect(manager).toContain('mutationState.canCreate');
    expect(manager).toContain('pinState.location');
    expect(manager).toContain('selection.selectedEntries');
    expect(manager).not.toContain('activePane?.isDirectoryTab');
    expect(manager).not.toContain('activePane?.pinnableLocation');
    expect(manager).not.toContain('activePane?.selectedEntries');
  });
});