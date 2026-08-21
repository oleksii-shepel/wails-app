import { Injectable } from '@angular/core';

import { FileType, type FileInfo } from '../shared/protocol';
import { ConfirmDialogService } from './confirm-dialog.service';
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

  constructor(
    private readonly pluginManager: PluginManagerService,
    private readonly confirmDialog: ConfirmDialogService,
  ) {}

  ensureContextMenuRegistered(): void {
    if (this.contextMenuRegistered) return;
    this.contextMenuRegistered = true;

    const score = (currentDirectory: string, targetPath: string, value: number) =>
      currentDirectory.toLowerCase() === 'network://adapters' &&
      /^network:\/\/entry\//i.test(targetPath)
        ? value
        : 0;

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.properties',
      label: 'Properties',
      score: context => score(context.currentDirectory, context.targetPath, 100),
      execute: context => this.showPropertiesByPath(context.targetPath),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.toggle',
      label: 'Enable / Disable adapter',
      score: context => score(context.currentDirectory, context.targetPath, 96),
      execute: context => this.toggleByPath(context.targetPath),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.renew',
      label: 'Renew DHCP',
      score: context => score(context.currentDirectory, context.targetPath, 92),
      execute: context => this.renewByPath(context.targetPath),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.diagnose',
      label: 'Diagnose',
      score: context => score(context.currentDirectory, context.targetPath, 88),
      execute: context => this.diagnoseByPath(context.targetPath),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.copy-ip',
      label: 'Copy IP address',
      score: context => score(context.currentDirectory, context.targetPath, 80),
      execute: context => this.copyFieldByPath(context.targetPath, 'address'),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.copy-mac',
      label: 'Copy MAC address',
      score: context => score(context.currentDirectory, context.targetPath, 79),
      execute: context => this.copyFieldByPath(context.targetPath, 'mac'),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.copy-gateway',
      label: 'Copy gateway',
      score: context => score(context.currentDirectory, context.targetPath, 78),
      execute: context => this.copyFieldByPath(context.targetPath, 'gateway'),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.copy-dns',
      label: 'Copy DNS servers',
      score: context => score(context.currentDirectory, context.targetPath, 77),
      execute: context => this.copyFieldByPath(context.targetPath, 'dns'),
    });

    this.pluginManager.registerContextMenuItem({
      id: 'builtin.network.adapter.settings',
      label: 'Open Network Settings',
      score: context => score(context.currentDirectory, context.targetPath, 70),
      execute: () => this.openSettings(),
    });
  }

  isAdapter(entry: FileInfo | null | undefined): boolean {
    return this.text(this.fields(entry), 'kind').toLowerCase() === 'adapter';
  }

  index(entry: FileInfo | null | undefined): number | null {
    const value = Number(this.fields(entry)?.['index']);
    return Number.isFinite(value) && value > 0 ? Math.trunc(value) : null;
  }

  enabled(entry: FileInfo): boolean {
    const fields = this.fields(entry);
    const admin = this.text(fields, 'adminStatus').toLowerCase();
    if (admin) return admin === 'enabled';
    return this.text(fields, 'status').toLowerCase() === 'up';
  }

  dhcpEnabled(entry: FileInfo): boolean {
    const value = this.fields(entry)?.['dhcpEnabled'];
    return value === true || String(value).toLowerCase() === 'true';
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
    await this.confirmDialog.alert({
      title: `${entry.name} — Network Adapter`,
      message: this.formatProperties(entry.name, this.fields(entry) ?? {}),
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
      message: summary,
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
      await navigator.clipboard.writeText(normalized);
      return true;
    } catch {
      const runtime = (window as unknown as {
        runtime?: {
          ClipboardSetText?: (text: string) => Promise<boolean>;
        };
      }).runtime;
      return typeof runtime?.ClipboardSetText === 'function'
        ? await runtime.ClipboardSetText(normalized)
        : false;
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
    await this.confirmDialog.alert({
      title: `${record.name} — Network Adapter`,
      message: this.formatProperties(record.name, record.fields ?? {}),
    });
  }

  private async toggleByPath(path: string): Promise<void> {
    const record = await this.recordForPath(path);
    if (!record) return;
    const entry = this.recordEntry(record);
    await this.setEnabled(entry, !this.enabled(entry));
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

  private async copyFieldByPath(path: string, field: string): Promise<void> {
    const record = await this.recordForPath(path);
    if (!record) return;
    const value = record.fields?.[field];
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
      return {
        id: recordId,
        name: String(record?.name ?? record?.Name ?? recordId),
        fields: (record?.fields ?? record?.Fields ?? {}) as AdapterFields,
      };
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

  private formatProperties(name: string, fields: AdapterFields): string {
    const lines = [
      ['Name', name],
      ['Description', this.text(fields, 'description')],
      ['Status', this.text(fields, 'status')],
      ['Administrative state', this.text(fields, 'adminStatus')],
      ['IP addresses', this.listText(fields['addresses'])],
      ['Gateway', this.text(fields, 'gateway')],
      ['DNS servers', this.listText(fields['dnsServers']) || this.text(fields, 'dns')],
      ['DNS suffix', this.text(fields, 'dnsSuffix')],
      ['MAC address', this.text(fields, 'mac')],
      ['Link speed', this.formatSpeed(fields['linkSpeed'])],
      ['MTU', this.text(fields, 'mtu')],
      ['DHCP', this.booleanText(fields['dhcpEnabled'])],
      ['Received', this.formatBytes(fields['bytesReceived'])],
      ['Sent', this.formatBytes(fields['bytesSent'])],
      ['Receive errors', this.text(fields, 'receiveErrors')],
      ['Send errors', this.text(fields, 'sendErrors')],
      ['Receive discards', this.text(fields, 'receiveDiscards')],
      ['Send discards', this.text(fields, 'sendDiscards')],
    ] as const;

    return lines
      .filter(([, value]) => String(value ?? '').trim() !== '')
      .map(([label, value]) => `${label}: ${value}`)
      .join('\n');
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
