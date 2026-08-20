import { readFileSync } from 'node:fs';

describe('pane shell state ports', () => {
  const manager = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
  const filePane = readFileSync(new URL('./file-pane.component.ts', import.meta.url), 'utf8');

  it('makes FilePaneComponent expose the narrow shell state ports directly', () => {
    expect(filePane).toContain('PaneStatePort');
    expect(filePane).toContain('get contextState(): PaneContextState');
    expect(filePane).toContain('get tabState(): PaneTabState');
    expect(filePane).toContain('get viewerState(): PaneViewerState');
    expect(filePane).toContain('get pinState(): PanePinState');
    expect(filePane).toContain('get mutationState(): PaneMutationState');
  });

  it('keeps dual-pane shell reads on grouped state and selection ports', () => {
    expect(manager).toContain('contextState.isDirectoryTab');
    expect(manager).toContain('mutationState.canCreate');
    expect(manager).toContain('pinState.location');
    expect(manager).toContain('selection.selectedEntries');
    expect(manager).not.toContain('activePane?.isDirectoryTab');
    expect(manager).not.toContain('activePane?.pinnableLocation');
    expect(manager).not.toContain('activePane?.selectedEntries');
  });
});
