import { readFileSync } from 'node:fs';

const host = readFileSync(
  new URL('./pane-host.component.ts', import.meta.url),
  'utf8',
);
const network = readFileSync(
  new URL('./network-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Network standard pane integration', () => {
  it('routes Network through the standard Directory-clone branch', () => {
    expect(host).toContain(
      "rendererKind !== 'directory' && rendererKind !== 'registry' && rendererKind !== 'network'",
    );
    expect(host).toContain("*ngSwitchCase=\"'network'\"");
    expect(host).toContain('[paneId]="paneId"');
    expect(host).not.toContain('[session]="provider"');
  });

  it('does not provider-sync the Network clone', () => {
    expect(host).toContain(
      "this.rendererKind === 'directory' || this.rendererKind === 'registry' || this.rendererKind === 'network'",
    );
  });

  it('uses Network clone as active pane component', () => {
    expect(host).toContain(
      "return this.rendererKind === 'directory' || this.rendererKind === 'registry' || this.rendererKind === 'network'",
    );
  });

  it('Network exposes the same public host contract as Directory', () => {
    for (const token of [
      '@Input() paneId:',
      '@Input() active = false;',
      '@Input() showHidden = false;',
      '@Input() pluginToolbarActions:',
      '@Output() activateRequested',
      '@Output() locationChange',
      '@Output() cursorChange',
      '@Output() entryContextMenu',
      '@Output() backgroundContextMenu',
    ]) {
      expect(network).toContain(token);
    }
  });
});
