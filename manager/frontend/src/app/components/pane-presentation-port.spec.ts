import { readFileSync } from 'node:fs';

describe('pane presentation port convergence', () => {
  const contract = readFileSync(new URL('./pane-component.contract.ts', import.meta.url), 'utf8');
  const filePane = readFileSync(new URL('./file-pane.component.ts', import.meta.url), 'utf8');
  const host = readFileSync(new URL('./pane-host.component.ts', import.meta.url), 'utf8');

  it('keeps presentation and viewport as separate capabilities', () => {
    expect(contract).toContain('interface PanePresentationPort');
    expect(contract).toContain('interface PaneViewportPort');
    const presentation = contract.slice(
      contract.indexOf('interface PanePresentationPort'),
      contract.indexOf('interface PaneViewportPort'),
    );
    expect(presentation).not.toContain('viewportSnapshot');
    expect(presentation).not.toContain('syncViewportToPeer');
  });

  it('lets FilePaneComponent participate in presentation mirroring directly', () => {
    expect(filePane).toContain('PanePresentationPort');
    expect(filePane).toContain('panePresentation(): PanePresentation');
    expect(filePane).toContain('applyMirroredPresentation(');
    expect(filePane).toContain('presentationChange = new EventEmitter<PanePresentation>()');
  });

  it('does not recursively publish mirrored presentation changes', () => {
    expect(filePane).toContain('applyingMirroredPresentation = true');
    expect(filePane).toContain('if (this.applyingMirroredPresentation');
  });

  it('exposes viewport separately from presentation on the transitional host', () => {
    expect(host).toContain('get presentation(): PanePresentationPort');
    expect(host).toContain('get viewport(): PaneViewportPort');
  });
});