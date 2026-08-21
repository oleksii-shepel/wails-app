import { FileType, type FileInfo } from '../shared/protocol';
import type { PanelSource, PanelSourceContext, PanelSourceListing } from './panel-source';
import type { SourceCapabilities } from './source-capabilities';

export type SystemSurfaceKind =
  | 'tasks'
  | 'services'
  | 'processes'
  | 'devices'
  | 'network'
  | 'environment'
  | 'shell-extensions';

export interface SystemSurfaceRecord {
  readonly id: string;
  readonly name: string;
  readonly fields?: Readonly<Record<string, unknown>>;
}

export interface SystemSurfaceColumn {
  readonly id: string;
  readonly label: string;
  readonly field: string;
  readonly width?: string;
}

export interface SystemSurfaceDescriptor {
  readonly kind: SystemSurfaceKind;
  readonly title: string;
  readonly columns: readonly SystemSurfaceColumn[];
}

export type SystemSurfaceFileInfo = FileInfo & {
  readonly systemSurfaceKind: SystemSurfaceKind;
  readonly systemRecordId: string;
  readonly systemFields: Readonly<Record<string, unknown>>;
};

const READ_ONLY: SourceCapabilities = Object.freeze({
  read: true,
  create: false,
  delete: false,
  rename: false,
  trash: false,
});

function rootFor(kind: SystemSurfaceKind): string {
  return `${kind}://`;
}

function stableLeafPath(kind: SystemSurfaceKind, id: string): string {
  return `${kind}://entry/${encodeURIComponent(id)}`;
}

const NETWORK_ROOT = 'network://';

function normalizeNetworkPath(path: string): string {
  const raw = String(path || NETWORK_ROOT).trim().replace(/\\/g, '/');
  if (!/^network:\/\//i.test(raw)) return NETWORK_ROOT;
  const body = raw.slice(raw.indexOf('://') + 3)
    .replace(/^\/+/, '')
    .replace(/\/+$/, '');
  return body ? `${NETWORK_ROOT}${body}` : NETWORK_ROOT;
}

const ENVIRONMENT_ROOT = 'environment://';

function normalizeEnvironmentPath(path: string): string {
  const raw = String(path || ENVIRONMENT_ROOT).trim().replace(/\\/g, '/');
  if (!/^environment:\/\//i.test(raw)) return ENVIRONMENT_ROOT;
  const body = raw.slice(raw.indexOf('://') + 3)
    .replace(/^\/+/, '')
    .replace(/\/+$/, '');
  return body ? `${ENVIRONMENT_ROOT}${body}` : ENVIRONMENT_ROOT;
}

function environmentRootEntries(): SystemSurfaceFileInfo[] {
  const scopes = [
    {
      id: 'process',
      name: 'Process variables',
      scope: 'Process',
      description: 'Environment inherited by this Manager process',
    },
    {
      id: 'user',
      name: 'User variables',
      scope: 'User',
      description: 'Variables persisted for the current Windows user',
    },
    {
      id: 'system',
      name: 'System variables',
      scope: 'System',
      description: 'Machine-wide Windows environment variables',
    },
  ] as const;

  return scopes.map(item => ({
    name: item.name,
    path: `${ENVIRONMENT_ROOT}${item.id}`,
    type: FileType.DIRECTORY,
    size: 0,
    created: 0,
    modified: 0,
    accessed: 0,
    permissions: 'r-x',
    isHidden: false,
    systemSurfaceKind: 'environment',
    systemRecordId: item.id,
    systemFields: {
      scope: item.scope,
      value: item.description,
      valueType: 'Scope',
      environmentNodeKind: 'scope',
    },
  }));
}

function fieldText(
  fields: Readonly<Record<string, unknown>> | undefined,
  field: string,
): string {
  const value = fields?.[field];
  return value == null ? '' : String(value);
}

function normalizeSystemRecords(raw: unknown): SystemSurfaceRecord[] {
  if (!Array.isArray(raw)) return [];
  return raw
    .filter(record => record && typeof record === 'object')
    .map((record: any, index: number) => ({
      id: String(record.id ?? record.ID ?? index),
      name: String(record.name ?? record.Name ?? record.id ?? record.ID ?? index),
      fields: (record.fields ?? record.Fields ?? {}) as Record<string, unknown>,
    }));
}

async function networkBackendRecords(
  method: 'NetworkListComputers' | 'NetworkListShares',
  argument?: string,
): Promise<SystemSurfaceRecord[]> {
  const app = (window as any)?.go?.app?.App;
  const fn = app?.[method];
  if (typeof fn !== 'function') return [];

  const raw = argument === undefined
    ? await fn.call(app)
    : await fn.call(app, argument);
  return normalizeSystemRecords(raw);
}

function networkRootEntries(): SystemSurfaceFileInfo[] {
  const categories = [
    {
      id: 'computers',
      name: 'Computers',
      kind: 'Computer group',
      status: 'Browse network computers',
    },
    {
      id: 'remote',
      name: 'Remote locations',
      kind: 'Location group',
      status: 'Connected remote filesystems',
    },
    {
      id: 'adapters',
      name: 'Network adapters',
      kind: 'Adapter group',
      status: 'Local interfaces',
    },
    {
      id: 'connections',
      name: 'Active connections',
      kind: 'Connection group',
      status: 'TCP and UDP endpoints',
    },
  ] as const;

  return categories.map(category => ({
    name: category.name,
    path: `${NETWORK_ROOT}${category.id}`,
    type: FileType.DIRECTORY,
    size: 0,
    created: 0,
    modified: 0,
    accessed: 0,
    permissions: 'r-x',
    isHidden: false,
    systemSurfaceKind: 'network',
    systemRecordId: category.id,
    systemFields: {
      kind: category.kind,
      status: category.status,
      local: '',
      remote: '',
      networkNodeKind: 'category',
    },
  }));
}

export class SystemSurfacePanelSource implements PanelSource {
  readonly id: string;
  readonly kind = 'virtual' as const;
  readonly title: string;

  constructor(readonly descriptor: SystemSurfaceDescriptor) {
    this.id = `builtin.system.${descriptor.kind}`;
    this.title = descriptor.title;
  }

  canHandle(context: PanelSourceContext): boolean {
    return context.path.toLowerCase().startsWith(rootFor(this.descriptor.kind));
  }

  capabilities(): SourceCapabilities { return READ_ONLY; }
  supportsStreaming(): boolean { return false; }
  getRootPath(): string { return rootFor(this.descriptor.kind); }

  getParentPath(context: PanelSourceContext): string {
    if (this.descriptor.kind === 'network') {
      const normalized = normalizeNetworkPath(context.path);
      if (normalized === NETWORK_ROOT) return NETWORK_ROOT;

      const body = normalized.slice(NETWORK_ROOT.length).replace(/\/+$/, '');
      const slash = body.lastIndexOf('/');
      return slash < 0 ? NETWORK_ROOT : `${NETWORK_ROOT}${body.slice(0, slash)}`;
    }

    if (this.descriptor.kind === 'environment') {
      const normalized = normalizeEnvironmentPath(context.path);
      return normalized === ENVIRONMENT_ROOT
        ? ENVIRONMENT_ROOT
        : ENVIRONMENT_ROOT;
    }

    return rootFor(this.descriptor.kind);
  }

  joinChildPath(context: PanelSourceContext, childName: string): string {
    if (this.descriptor.kind === 'network') {
      const base = normalizeNetworkPath(context.path).replace(/\/+$/, '');
      return `${base}/${encodeURIComponent(childName)}`;
    }

    if (this.descriptor.kind === 'environment') {
      const base = normalizeEnvironmentPath(context.path).replace(/\/+$/, '');
      return `${base}/${encodeURIComponent(childName)}`;
    }

    return stableLeafPath(this.descriptor.kind, childName);
  }

  canPreviewEntry(): boolean { return false; }

  activateEntry(_context: PanelSourceContext, entry: FileInfo): boolean {
    // Directories are navigated by DirectoryPaneComponent. Synthetic leaves
    // remain command/context-menu targets and must not enter the file viewer.
    return entry.type !== FileType.DIRECTORY;
  }

  async list(context: PanelSourceContext): Promise<PanelSourceListing> {
    if (this.descriptor.kind === 'network') {
      return this.listNetwork(context);
    }
    if (this.descriptor.kind === 'environment') {
      return this.listEnvironment(context);
    }

    const records = await this.records();
    return {
      location: {
        path: rootFor(this.descriptor.kind),
        sourceId: this.id,
        sourceKind: this.kind,
      },
      entries: records.map(record => this.toFileInfo(record)),
    };
  }

  async records(): Promise<SystemSurfaceRecord[]> {
    const app = (window as any)?.go?.app?.App;
    if (!app?.SystemSurfaceList) return [];
    const raw = await app.SystemSurfaceList(this.descriptor.kind);
    return normalizeSystemRecords(raw);
  }

  private async listEnvironment(
    context: PanelSourceContext,
  ): Promise<PanelSourceListing> {
    const path = normalizeEnvironmentPath(context.path);
    let entries: SystemSurfaceFileInfo[];

    if (path === ENVIRONMENT_ROOT) {
      entries = environmentRootEntries();
    } else {
      const scope = path.slice(ENVIRONMENT_ROOT.length).toLowerCase();
      const expectedScope =
        scope === 'process' ? 'process' :
        scope === 'user' ? 'user' :
        scope === 'system' ? 'system' :
        '';

      if (!expectedScope) {
        entries = [];
      } else {
        const records = (await this.records()).filter(record =>
          fieldText(record.fields, 'scope').toLowerCase() === expectedScope,
        );
        entries = records.map(record =>
          this.toFileInfo(
            record,
            `${ENVIRONMENT_ROOT}${scope}/entry/${encodeURIComponent(record.id)}`,
          ),
        );
      }
    }

    return {
      location: {
        path,
        sourceId: this.id,
        sourceKind: this.kind,
      },
      entries,
    };
  }

  private async listNetwork(
    context: PanelSourceContext,
  ): Promise<PanelSourceListing> {
    const path = normalizeNetworkPath(context.path);
    let entries: SystemSurfaceFileInfo[];

    if (path === NETWORK_ROOT) {
      entries = networkRootEntries();
    } else if (path === `${NETWORK_ROOT}adapters`) {
      const records = (await this.records()).filter(
        record => fieldText(record.fields, 'kind').toLowerCase() === 'adapter',
      );
      entries = records.map(record => this.toNetworkLeaf(record));
    } else if (path === `${NETWORK_ROOT}connections`) {
      const records = (await this.records()).filter(record => {
        const kind = fieldText(record.fields, 'kind').toLowerCase();
        return kind === 'tcp' || kind === 'udp';
      });
      entries = records.map(record => this.toNetworkLeaf(record));
    } else if (path === `${NETWORK_ROOT}computers`) {
      entries = (await networkBackendRecords('NetworkListComputers'))
        .map(record => this.toNetworkComputer(record));
    } else if (path.startsWith(`${NETWORK_ROOT}computers/`)) {
      const encoded = path.slice(`${NETWORK_ROOT}computers/`.length)
        .split('/')[0] ?? '';
      const computer = decodeURIComponent(encoded);
      entries = (await networkBackendRecords('NetworkListShares', computer))
        .map(record => this.toNetworkShare(record, computer));
    } else if (path === `${NETWORK_ROOT}remote`) {
      entries = await this.networkRemoteEntries();
    } else {
      entries = [];
    }

    return {
      location: {
        path,
        sourceId: this.id,
        sourceKind: this.kind,
      },
      entries,
    };
  }

  private toNetworkLeaf(record: SystemSurfaceRecord): SystemSurfaceFileInfo {
    return this.toFileInfo(record, stableLeafPath('network', record.id));
  }

  private toNetworkComputer(record: SystemSurfaceRecord): SystemSurfaceFileInfo {
    const fields = record.fields ?? {};
    return this.toFileInfo(
      {
        ...record,
        fields: {
          ...fields,
          kind: 'Computer',
          status: fieldText(fields, 'status') || 'Discovered',
        },
      },
      `${NETWORK_ROOT}computers/${encodeURIComponent(record.name)}`,
      FileType.DIRECTORY,
    );
  }

  private toNetworkShare(
    record: SystemSurfaceRecord,
    computer: string,
  ): SystemSurfaceFileInfo {
    const fields = record.fields ?? {};
    const unc = fieldText(fields, 'remote') ||
      `//${computer}/${record.name}`;
    return this.toFileInfo(
      {
        ...record,
        fields: {
          ...fields,
          kind: 'Share',
          status: fieldText(fields, 'status') || 'Available',
          remote: unc,
        },
      },
      unc.replace(/\\/g, '/'),
      FileType.DIRECTORY,
    );
  }

  private async networkRemoteEntries(): Promise<SystemSurfaceFileInfo[]> {
    const app = (window as any)?.go?.app?.App;
    if (!app?.RemoteListConnections) return [];

    const raw = await app.RemoteListConnections();
    if (!Array.isArray(raw)) return [];

    return raw.map((mount: any, index: number) => {
      const name = String(
        mount?.name ?? mount?.Name ??
        mount?.publicPath ?? mount?.PublicPath ??
        `Remote ${index + 1}`,
      );
      const publicPath = String(
        mount?.publicPath ?? mount?.PublicPath ?? '',
      ).trim();
      const host = String(mount?.host ?? mount?.Host ?? '').trim();
      const protocol = String(
        mount?.protocol ?? mount?.Protocol ?? '',
      ).trim().toUpperCase();

      return this.toFileInfo(
        {
          id: publicPath || name,
          name,
          fields: {
            kind: 'Remote',
            status: 'Connected',
            local: protocol,
            remote: publicPath || host,
          },
        },
        publicPath || `${NETWORK_ROOT}remote/${encodeURIComponent(name)}`,
        FileType.DIRECTORY,
      );
    });
  }

  private toFileInfo(
    record: SystemSurfaceRecord,
    path = stableLeafPath(this.descriptor.kind, record.id),
    type: FileType = FileType.FILE,
  ): SystemSurfaceFileInfo {
    const fields = record.fields ?? {};
    return {
      name: record.name,
      path,
      type,
      size: Number(fields['size'] ?? fields['workingSet'] ?? 0) || 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: type === FileType.DIRECTORY ? 'r-x' : 'r--',
      isHidden: false,
      systemSurfaceKind: this.descriptor.kind,
      systemRecordId: record.id,
      systemFields: fields,
    };
  }

}

export function systemField(entry: FileInfo, field: string): string {
  const value = (entry as Partial<SystemSurfaceFileInfo>).systemFields?.[field];
  if (value === null || value === undefined) return '';
  if (typeof value === 'boolean') return value ? 'Yes' : 'No';
  return String(value);
}