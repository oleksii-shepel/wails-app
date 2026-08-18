import { readFileSync } from 'node:fs';

describe('Reassembly pane mirroring contract', () => {
  const pane = readFileSync(new URL('./directory-pane.component.ts', import.meta.url), 'utf8');
  const manager = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');

  it('publishes pane presentation changes', () => {
    expect(pane).toContain('presentationChange = new EventEmitter<PanePresentation>()');
    expect(pane).toContain('emitPresentationChange()');
  });

  it('applies mirrored presentation without recursively re-emitting it', () => {
    expect(pane).toContain('applyingMirroredPresentation = true');
    expect(pane).toContain('if (this.applyingMirroredPresentation) return');
  });

  it('keeps filter mirroring optional', () => {
    expect(manager).toContain('mirrorFilter = false');
    expect(manager).toContain('filter: this.mirrorFilter');
  });

  it('falls back to a nearest corresponding peer location', () => {
    expect(manager).toContain('mirroredPeerCandidates(');
    expect(manager).toContain('nearest peer:');
  });
});
