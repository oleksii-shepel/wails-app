import { readFileSync } from 'node:fs';

const adapterService = readFileSync(
  new URL('../services/network-adapter.service.ts', import.meta.url),
  'utf8',
);
const networkPane = readFileSync(
  new URL('./network-pane.component.ts', import.meta.url),
  'utf8',
);
const dualPane = readFileSync(
  new URL('./dual-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Network adapter context menu relevance', () => {
  it('does not mix normal file commands into the adapter menu', () => {
    expect(dualPane).toContain("/^network:\\/\\/adapters\\/?$/i");
    expect(dualPane).toContain("title: entry.name || 'Network adapter'");
    expect(dualPane).toContain('items: normalizeContextMenuItems(adapterItems)');
  });

  it('primes context data from the exact adapter that was right-clicked', () => {
    expect(networkPane).toContain('this.networkAdapters.rememberAdapter(entry);');
  });

  it('splits enable and disable into mutually relevant commands', () => {
    expect(adapterService).toContain("label: 'Enable adapter'");
    expect(adapterService).toContain("label: 'Disable adapter'");
    expect(adapterService).toContain('fields => hasIndex(fields) && isDisabled(fields)');
    expect(adapterService).toContain('fields => hasIndex(fields) && isEnabled(fields)');
  });

  it('shows field-dependent copy commands only when the field exists', () => {
    expect(adapterService).toContain("has('address')");
    expect(adapterService).toContain("has('mac')");
    expect(adapterService).toContain("has('gateway')");
    expect(adapterService).toContain("has('dnsServers')");
  });

  it('shows DHCP and connection actions only when operationally relevant', () => {
    expect(adapterService).toContain('isOperational(fields) && this.dhcpFromFields(fields)');
    expect(adapterService).toContain("isOperational(fields) && has('address')(fields)");
    expect(adapterService).toContain("label: 'Show connections'");
  });
});
