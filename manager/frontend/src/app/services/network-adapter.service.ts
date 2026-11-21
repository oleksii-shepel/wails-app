import { writeClipboardText } from '../utils/clipboard-text';
import { Injectable } from '@angular/core';

import { FileType, type FileInfo } from '../shared/protocol';
import { ConfirmDialogService, type ConfirmDialogDetailSection } from './confirm-dialog.service';
import { PluginManagerService } from './plugin-manager.service';

type AdapterFields = Readonly<Record<string, unknown>>;

interface SystemSurfaceRecord {
  readonly id: string;
  readonly name: string;
  readonly fields?: AdapterFields;
}

interface AdapterDiagnostic {
  readonly id?: string;
  readonly name?: string;
  readonly fields?: Readonly<{
    healthy?: boolean;
    summary?: string;
    checks?: readonly string[];
  }>;
}

@Injectable({ providedIn: 'root' })
export class NetworkAdapterService {
  private contextMenuRegistered = false;
  private readonly adapterContext = new Map<string, {
    name: string;
    fields: AdapterFields;
  }>();

  constructor(
    private readonly pluginManager: PluginManagerService,
    private readonly confirmDialog: ConfirmDialogService,
  ) {}

  ensureContextMenuRegistered(): void {
    if (this.contextMenuRegistered) return;
    this.contextMenuRegistered = true;

    const adapterFor = (currentDirectory: string, targetPath: string) => {
      if (currentDirectory.toLowerCase() !== 'network://adapters') return null;
      return this.adapterContext.get(targetPath) ?? null;
    };

    const score = (
      currentDirectory: string,
      targetPath: string,
      value: number,
      predicate: (fields: AdapterFields) => boolean = () => true,
    ): number => {
      const adapter = adapterFor(currentDirectory, targetPath);
      return adapter && predicate(adapter.fields) ? value : 0;
    };

    const has = (field: string) => (fields: AdapterFields) =>
      this.fieldValue(fields, field) !== '';
    const hasIndex = (fields: AdapterFields) => this.indexFromFields(fields) != null;
    const isEnabled = (fields: AdapterFields) => this.enabledFromFields(fields);
    const isDisabled = (fields: AdapterFields) => !this.enabledFromFields(fields);
    const isOperational = (fields: AdapterFields) =>
      this.text(fields, 'status').toLowerCase() === 'up';
    const canRenewDHCP = (fields: AdapterFields) =>
      hasIndex(fields) && isOperational(fields) && this.dhcpFromFields(fields);
    const canShowConnections = (fields: AdapterFields) =>
      isOperational(fields) && has('address')(fields);

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.properties',
      label: 'Properties',
      score: context => score(context.currentDirectory, context.targetPath, 100),
      execute: context => this.showPropertiesByPath(context.targetPath),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.enable',
      label: 'Enable adapter',
      score: context => score(
        context.currentDirectory,
        context.targetPath,
        96,
        fields => hasIndex(fields) && isDisabled(fields),
      ),
      execute: context => this.setEnabledByPath(context.targetPath, true),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.disable',
      label: 'Disable adapter',
      score: context => score(
        context.currentDirectory,
        context.targetPath,
        96,
        fields => hasIndex(fields) && isEnabled(fields),
      ),
      execute: context => this.setEnabledByPath(context.targetPath, false),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.renew',
      label: 'Renew DHCP',
      score: context => score(
        context.currentDirectory,
        context.targetPath,
        92,
        canRenewDHCP,
      ),
      execute: context => this.renewByPath(context.targetPath),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.diagnose',
      label: 'Diagnose',
      score: context => score(
        context.currentDirectory,
        context.targetPath,
        88,
        hasIndex,
      ),
      execute: context => this.diagnoseByPath(context.targetPath),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.connections',
      label: 'Show connections',
      score: context => score(
        context.currentDirectory,
        context.targetPath,
        84,
        canShowConnections,
      ),
      execute: context => {
        const adapter = this.adapterContext.get(context.targetPath);
        const address = adapter ? this.fieldValue(adapter.fields, 'address') : '';
        if (address) {
          window.dispatchEvent(new CustomEvent('network-adapter-connections', {
            detail: { address },
          }));
        }
      },
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.copy-ip',
      label: 'Copy IP address',
      score: context => score(context.currentDirectory, context.targetPath, 80, has('address')),
      execute: context => this.copyFieldByPath(context.targetPath, 'address'),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.copy-mac',
      label: 'Copy MAC address',
      score: context => score(context.currentDirectory, context.targetPath, 79, has('mac')),
      execute: context => this.copyFieldByPath(context.targetPath, 'mac'),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.copy-gateway',
      label: 'Copy gateway',
      score: context => score(context.currentDirectory, context.targetPath, 78, has('gateway')),
      execute: context => this.copyFieldByPath(context.targetPath, 'gateway'),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.copy-dns',
      label: 'Copy DNS servers',
      score: context => score(context.currentDirectory, context.targetPath, 77, fields =>
        has('dns')(fields) || has('dnsServers')(fields),
      ),
      execute: context => this.copyFieldByPath(context.targetPath, 'dnsServers', 'dns'),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.settings',
      label: 'Open Network Settings',
      score: context => score(context.currentDirectory, context.targetPath, 70),
      execute: () => this.openSettings(),
    });
  }

  rememberAdapter(entry: FileInfo): void {
    if (!this.isAdapter(entry)) return;
    const fields = this.fields(entry) ?? {};
    this.adapterContext.set(entry.path, { name: entry.name, fields });
  }

  isAdapter(entry: FileInfo | null | undefined): boolean {
    return this.text(this.fields(entry), 'kind').toLowerCase() === 'adapter';
  }

  index(entry: FileInfo | null | undefined): number | null {
    return this.indexFromFields(this.fields(entry));
  }

  enabled(entry: FileInfo): boolean {
    return this.enabledFromFields(this.fields(entry) ?? {});
  }

  dhcpEnabled(entry: FileInfo): boolean {
    return this.dhcpFromFields(this.fields(entry) ?? {});
  }

  value(entry: FileInfo, field: string): string {
    const value = this.fields(entry)?.[field];
    if (Array.isArray(value)) return value.map(String).join(', ');
    return value == null ? '' : String(value);
  }

  primaryAddress(entry: FileInfo): string {
    return this.value(entry, 'address');
  }

  async showProperties(entry: FileInfo): Promise<void> {
    const fields = this.fields(entry) ?? {};
    await this.confirmDialog.alert({
      title: `${entry.name} — Network Adapter`,
      message: this.adapterSubtitle(fields),
      tone: 'info',
      footerHint: false,
      details: this.adapterPropertySections(entry.name, fields),
    });
  }

  async setEnabled(entry: FileInfo, enabled: boolean): Promise<void> {
    const index = this.requireIndex(entry);
    const verb = enabled ? 'enable' : 'disable';
    const confirmed = await this.confirmDialog.confirm({
      title: `${enabled ? 'Enable' : 'Disable'} Network Adapter`,
      message: `Are you sure you want to ${verb} ${entry.name}?${
        enabled ? '' : '\n\nExisting network connections through this adapter may be interrupted.'
      }`,
      confirmText: enabled ? 'Enable' : 'Disable',
      danger: !enabled,
    });
    if (!confirmed) return;

    const app = this.app();
    if (typeof app?.NetworkAdapterSetEnabled !== 'function') {
      throw new Error('NetworkAdapterSetEnabled binding is unavailable.');
    }
    await app.NetworkAdapterSetEnabled(index, enabled);
    this.changed();
  }

  async renewDHCP(entry: FileInfo): Promise<void> {
    if (!this.dhcpEnabled(entry)) {
      await this.confirmDialog.alert({
        title: 'Renew DHCP',
        message: `${entry.name} is not configured to use DHCP.`,
      });
      return;
    }

    const index = this.requireIndex(entry);
    const app = this.app();
    if (typeof app?.NetworkAdapterRenewDHCP !== 'function') {
      throw new Error('NetworkAdapterRenewDHCP binding is unavailable.');
    }
    await app.NetworkAdapterRenewDHCP(index);
    this.changed();
  }

  async diagnose(entry: FileInfo): Promise<void> {
    const index = this.requireIndex(entry);
    const app = this.app();
    if (typeof app?.NetworkAdapterDiagnose !== 'function') {
      throw new Error('NetworkAdapterDiagnose binding is unavailable.');
    }

    const result = await app.NetworkAdapterDiagnose(index) as AdapterDiagnostic;
    const healthy = result?.fields?.healthy === true;
    const summary = String(result?.fields?.summary ?? 'No diagnostic details were returned.');

    await this.confirmDialog.alert({
      title: `${entry.name} — ${healthy ? 'Connectivity OK' : 'Connectivity Issues'}`,
      message: '',
      tone: healthy ? 'success' : 'warning',
      footerHint: false,
      details: [{
        title: 'Diagnostics',
        rows: summary
          .split('\n')
          .map((line, index) => ({
            label: index === 0 ? 'Result' : '',
            value: line,
          })),
      }],
    });
  }

  async openSettings(): Promise<void> {
    const app = this.app();
    if (typeof app?.NetworkOpenSettings !== 'function') {
      throw new Error('NetworkOpenSettings binding is unavailable.');
    }
    await app.NetworkOpenSettings();
  }

  async copyValue(value: string): Promise<boolean> {
    const normalized = value.trim();
    if (!normalized) return false;

    try {
      await writeClipboardText(normalized);
      return true;
    } catch {
      return false;
    }
  }

  formatSpeed(bitsPerSecond: unknown): string {
    const value = Number(bitsPerSecond);
    if (!Number.isFinite(value) || value <= 0) return '';
    if (value >= 1_000_000_000) {
      return `${(value / 1_000_000_000).toFixed(value >= 10_000_000_000 ? 0 : 1)} Gbps`;
    }
    if (value >= 1_000_000) {
      return `${(value / 1_000_000).toFixed(value >= 100_000_000 ? 0 : 1)} Mbps`;
    }
    if (value >= 1_000) return `${(value / 1_000).toFixed(0)} Kbps`;
    return `${value.toFixed(0)} bps`;
  }

  private async showPropertiesByPath(path: string): Promise<void> {
    const record = await this.recordForPath(path);
    if (!record) return;
    const fields = record.fields ?? {};
    await this.confirmDialog.alert({
      title: `${record.name} — Network Adapter`,
      message: this.adapterSubtitle(fields),
      tone: 'info',
      footerHint: false,
      details: this.adapterPropertySections(record.name, fields),
    });
  }

  private async setEnabledByPath(path: string, enabled: boolean): Promise<void> {
    const record = await this.recordForPath(path);
    if (!record) return;
    await this.setEnabled(this.recordEntry(record), enabled);
  }

  private async renewByPath(path: string): Promise<void> {
    const record = await this.recordForPath(path);
    if (!record) return;
    await this.renewDHCP(this.recordEntry(record));
  }

  private async diagnoseByPath(path: string): Promise<void> {
    const record = await this.recordForPath(path);
    if (!record) return;
    await this.diagnose(this.recordEntry(record));
  }

  private async copyFieldByPath(
    path: string,
    field: string,
    fallbackField?: string,
  ): Promise<void> {
    const record = await this.recordForPath(path);
    if (!record) return;
    const fields = record.fields ?? {};
    let value = fields[field];
    if (this.fieldValue(fields, field) === '' && fallbackField) {
      value = fields[fallbackField];
    }
    const text = Array.isArray(value) ? value.map(String).join(', ') : String(value ?? '');
    await this.copyValue(text);
  }

  private async recordForPath(path: string): Promise<SystemSurfaceRecord | null> {
    const id = this.decodeRecordId(path);
    if (!id) return null;
    const app = this.app();
    if (typeof app?.SystemSurfaceList !== 'function') return null;

    const raw = await app.SystemSurfaceList('network');
    if (!Array.isArray(raw)) return null;

    for (const candidate of raw) {
      const record = candidate as any;
      const recordId = String(record?.id ?? record?.ID ?? '');
      if (recordId !== id) continue;
      const normalized = {
        id: recordId,
        name: String(record?.name ?? record?.Name ?? recordId),
        fields: (record?.fields ?? record?.Fields ?? {}) as AdapterFields,
      };
      this.adapterContext.set(path, {
        name: normalized.name,
        fields: normalized.fields ?? {},
      });
      return normalized;
    }
    return null;
  }

  private decodeRecordId(path: string): string {
    const match = /^network:\/\/entry\/(.+)$/i.exec(path.trim());
    if (!match) return '';
    try {
      return decodeURIComponent(match[1]);
    } catch {
      return match[1];
    }
  }

  private recordEntry(record: SystemSurfaceRecord): FileInfo {
    return {
      name: record.name,
      path: `network://entry/${encodeURIComponent(record.id)}`,
      type: FileType.FILE,
      size: 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: 'r--',
      isHidden: false,
      systemFields: record.fields ?? {},
    } as FileInfo;
  }

  private fields(entry: FileInfo | null | undefined): AdapterFields | undefined {
    return (entry as FileInfo & {
      systemFields?: AdapterFields;
    } | null | undefined)?.systemFields;
  }

  private requireIndex(entry: FileInfo): number {
    const index = this.index(entry);
    if (index == null) {
      throw new Error(`Network adapter index is unavailable for ${entry.name}.`);
    }
    return index;
  }

  private text(fields: AdapterFields | undefined, key: string): string {
    const value = fields?.[key];
    return value == null ? '' : String(value);
  }

  private adapterSubtitle(fields: AdapterFields): string {
    const description = this.text(fields, 'description');
    const status = this.text(fields, 'status');
    return [description, status].filter(Boolean).join(' · ');
  }

  private adapterPropertySections(
    name: string,
    fields: AdapterFields,
  ): readonly ConfirmDialogDetailSection[] {
    const sections: ConfirmDialogDetailSection[] = [];

    const identity = this.rows([
      ['Name', name],
      ['Description', this.text(fields, 'description')],
      ['Interface name', this.text(fields, 'interfaceName')],
      ['Status', this.text(fields, 'status')],
      ['Administrative state', this.text(fields, 'adminStatus')],
      ['Interface type', this.adapterType(fields)],
      ['Interface index', this.text(fields, 'index')],
    ]);
    if (identity.length) sections.push({ title: 'Adapter', rows: identity });

    const addressing = this.rows([
      ['IP addresses', this.stringList(fields['addresses']), true],
      ['Gateway', this.stringList(fields['gateways']) || this.text(fields, 'gateway'), true],
      ['DNS servers', this.stringList(fields['dnsServers']) || this.text(fields, 'dns'), true],
      ['DNS suffix', this.text(fields, 'dnsSuffix'), true],
      ['DHCP', this.booleanText(fields['dhcpEnabled'])],
    ]);
    if (addressing.length) sections.push({ title: 'Addressing', rows: addressing });

    const hardware = this.rows([
      ['MAC address', this.text(fields, 'mac'), true],
      ['Link speed', this.formatSpeed(fields['linkSpeed'])],
      ['Transmit speed', this.formatSpeed(fields['transmitSpeed'])],
      ['Receive speed', this.formatSpeed(fields['receiveSpeed'])],
      ['MTU', this.text(fields, 'mtu')],
      ['Flags', this.text(fields, 'flags')],
    ]);
    if (hardware.length) sections.push({ title: 'Hardware & link', rows: hardware });

    const traffic = this.rows([
      ['Received', this.formatBytes(fields['bytesReceived'])],
      ['Sent', this.formatBytes(fields['bytesSent'])],
      ['Receive errors', this.text(fields, 'receiveErrors')],
      ['Send errors', this.text(fields, 'sendErrors')],
      ['Receive discards', this.text(fields, 'receiveDiscards')],
      ['Send discards', this.text(fields, 'sendDiscards')],
    ]);
    if (traffic.length) sections.push({ title: 'Traffic', rows: traffic });

    return sections;
  }

  private rows(
    definitions: readonly (readonly [string, string | readonly string[], boolean?])[],
  ): ConfirmDialogDetailSection['rows'] {
    return definitions
      .filter(([, value]) => Array.isArray(value) ? value.length > 0 : String(value ?? '').trim() !== '')
      .map(([label, value, monospace]) => ({ label, value, monospace }));
  }

  private stringList(value: unknown): string | readonly string[] {
    if (Array.isArray(value)) {
      return value.map(String).map(item => item.trim()).filter(Boolean);
    }
    return String(value ?? '').trim();
  }

  private adapterType(fields: AdapterFields): string {
    const description = this.text(fields, 'description').toLowerCase();
    if (description.includes('bluetooth')) return 'Bluetooth PAN';
    if (description.includes('hyper-v')) return 'Hyper-V virtual adapter';
    if (description.includes('vmware')) return 'VMware virtual adapter';
    if (description.includes('virtualbox')) return 'VirtualBox virtual adapter';
    if (description.includes('wireguard')) return 'WireGuard adapter';
    if (description.includes('vpn') || description.includes('tap') || description.includes('tun')) {
      return 'VPN / virtual adapter';
    }

    const raw = Number(fields['ifType']);
    const friendly = new Map<number, string>([
      [6, 'Ethernet'],
      [24, 'Loopback'],
      [53, 'PPP / VPN'],
      [71, 'Wi-Fi'],
      [131, 'Tunnel'],
      [144, 'IEEE 1394'],
    ]).get(raw);
    if (friendly) return friendly;
    return Number.isFinite(raw) && raw > 0 ? `Type ${raw}` : '';
  }

  private fieldValue(fields: AdapterFields, field: string): string {
    const value = fields[field];
    if (Array.isArray(value)) return value.map(String).filter(Boolean).join(', ');
    return value == null ? '' : String(value).trim();
  }

  private indexFromFields(fields: AdapterFields | undefined): number | null {
    const value = Number(fields?.['index']);
    return Number.isFinite(value) && value > 0 ? Math.trunc(value) : null;
  }

  private enabledFromFields(fields: AdapterFields): boolean {
    const admin = this.text(fields, 'adminStatus').toLowerCase();
    if (admin) return admin === 'enabled';
    return this.text(fields, 'status').toLowerCase() === 'up';
  }

  private dhcpFromFields(fields: AdapterFields): boolean {
    const value = fields['dhcpEnabled'];
    return value === true || String(value).toLowerCase() === 'true';
  }

  private listText(value: unknown): string {
    return Array.isArray(value) ? value.map(String).join(', ') : String(value ?? '');
  }

  private booleanText(value: unknown): string {
    if (value === true || String(value).toLowerCase() === 'true') return 'Yes';
    if (value === false || String(value).toLowerCase() === 'false') return 'No';
    return '';
  }

  private formatBytes(value: unknown): string {
    const bytes = Number(value);
    if (!Number.isFinite(bytes) || bytes < 0) return '';
    if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(2)} GB`;
    if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(1)} MB`;
    if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${bytes.toFixed(0)} B`;
  }

  private app(): any {
    return (window as any)?.go?.app?.App;
  }

  private changed(): void {
    window.dispatchEvent(new CustomEvent('network-adapter-changed'));
  }
}