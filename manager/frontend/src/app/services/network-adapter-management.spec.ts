import { readFileSync } from 'node:fs';

const pane = readFileSync(
  new URL('../components/network-pane.component.ts', import.meta.url),
  'utf8',
);
const service = readFileSync(
  new URL('./network-adapter.service.ts', import.meta.url),
  'utf8',
);

describe('Network adapter management', () => {
  it('adds adapter-specific toolbar operations', () => {
    for (const label of [
      'Properties',
      'Renew DHCP',
      'Diagnose',
      'Connections',
      'Copy IP',
      'Copy MAC',
      'Copy gateway',
      'Copy DNS',
      'Network Settings',
    ]) {
      expect(pane).toContain(`label: '${label}'`);
    }
    expect(pane).toContain("label: enabled ? 'Disable' : 'Enable'");
  });

  it('uses the pane status line while adapter operations run', () => {
    expect(pane).toContain('networkOperationStatus');
    expect(pane).toContain('Renewing DHCP lease for');
    expect(pane).toContain('Diagnosing ${adapter.name}…');
  });

  it('opens adapter properties on double click', () => {
    expect(pane).toContain('this.networkAdapters.isAdapter(entry)');
    expect(pane).toContain('this.networkAdapters.showProperties(entry)');
  });

  it('registers adapter context-menu operations', () => {
    for (const id of [
      'builtin.network.adapter.properties',
      'builtin.network.adapter.toggle',
      'builtin.network.adapter.renew',
      'builtin.network.adapter.diagnose',
      'builtin.network.adapter.copy-ip',
      'builtin.network.adapter.copy-mac',
      'builtin.network.adapter.copy-gateway',
      'builtin.network.adapter.copy-dns',
      'builtin.network.adapter.settings',
    ]) {
      expect(service).toContain(`id: '${id}'`);
    }
  });

  it('refreshes adapters when a native mutation completes', () => {
    expect(service).toContain("new CustomEvent('network-adapter-changed')");
    expect(pane).toContain("window.addEventListener(");
  });
});
