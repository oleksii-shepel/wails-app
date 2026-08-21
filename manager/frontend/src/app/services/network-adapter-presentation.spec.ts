import { readFileSync } from 'node:fs';

const adapterService = readFileSync(
  new URL('./network-adapter.service.ts', import.meta.url),
  'utf8',
);
const dialog = readFileSync(
  new URL('../components/confirm-dialog.component.ts', import.meta.url),
  'utf8',
);

describe('Network adapter presentation', () => {
  it('uses structured adapter property sections instead of a flat message', () => {
    expect(adapterService).toContain("title: 'Adapter'");
    expect(adapterService).toContain("title: 'Addressing'");
    expect(adapterService).toContain("title: 'Hardware & link'");
    expect(adapterService).toContain("title: 'Traffic'");
    expect(adapterService).toContain("tone: 'info'");
    expect(adapterService).toContain('footerHint: false');
  });

  it('formats multivalue network fields as separate selectable lines', () => {
    expect(dialog).toContain('class="property-grid"');
    expect(dialog).toContain('class="property-value-line"');
    expect(dialog).toContain('[class.property-value--mono]="row.monospace"');
    expect(dialog).toContain('user-select: text');
  });

  it('recognizes physical, wireless, bluetooth, tunnel and virtual adapters', () => {
    for (const label of [
      'Ethernet',
      'Wi-Fi',
      'Bluetooth PAN',
      'Tunnel',
      'Hyper-V virtual adapter',
      'VMware virtual adapter',
      'VirtualBox virtual adapter',
      'WireGuard adapter',
      'VPN / virtual adapter',
    ]) {
      expect(adapterService).toContain(label);
    }
  });
});
