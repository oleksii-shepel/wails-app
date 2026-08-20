import type { RegistryFileInfo, RegistrySourceCapabilities } from '../registry-model';
import type { RegistryPanelSourceHost } from '../registry-panel-source';
import type { RegistryPluginOperations } from '../registry.plugin';

export interface RegistryBackendTransport {
  call<T = unknown>(method: string, params?: Readonly<Record<string, unknown>>): Promise<T>;
}

export interface RegistryDialogHost {
  prompt(request: {
    title: string;
    message: string;
    confirmText: string;
    defaultValue?: string;
    historyKey?: string;
  }): Promise<string | null>;
  confirm(request: {
    title: string;
    message: string;
    confirmText: string;
    danger?: boolean;
  }): Promise<boolean>;
}

interface RegistryWireEntry {
  name: string;
  path: string;
  kind: string;
  valueType?: string;
  preview?: string;
}

interface RegistryWireListing {
  path: string;
  entries: RegistryWireEntry[];
}

interface RegistryListingResult {
  path: string;
  entries: RegistryFileInfo[];
}

export interface RegistryBackendHello {
  protocol: number;
  plugin: string;
  target: string;
}

interface RegistryReadResult {
  content?: string;
}

export class RegistryBackendError extends Error {
  constructor(message: string, readonly method?: string, readonly causeValue?: unknown) {
    super(message);
    this.name = 'RegistryBackendError';
  }
}

const ROOT = 'registry://';

export function normalizeRegistryPath(path: string): string {
  const raw = (path || ROOT).trim().replace(/\\/g, '/');
  if (!/^registry:\/\//i.test(raw)) return raw;
  return `registry://${raw.slice(raw.indexOf('://') + 3).replace(/^\/+/, '')}`;
}

export function joinRegistryPath(parts: readonly string[]): string {
  if (!parts.length) return ROOT;
  const [head, ...tail] = parts;
  let result = normalizeRegistryPath(head || ROOT);
  for (const part of tail) {
    const segment = String(part ?? '').replace(/^\/+|\/+$/g, '');
    if (!segment) continue;
    result = result === ROOT ? `${ROOT}${segment}` : `${result.replace(/\/+$/, '')}/${segment}`;
  }
  return normalizeRegistryPath(result);
}


function toRegistryFileInfo(entry: RegistryWireEntry): RegistryFileInfo {
  const kind = String(entry.kind ?? '').toLowerCase();
  return {
    name: entry.name,
    path: normalizeRegistryPath(entry.path),
    type: kind === 'key' ? 'DIRECTORY' : 'FILE',
    size: kind === 'value' ? new TextEncoder().encode(entry.preview ?? '').byteLength : 0,
    created: 0,
    modified: 0,
    accessed: 0,
    permissions: '',
    isHidden: false,
    registryKind: kind === 'key' ? 'Key' : 'Value',
    registryValueType: entry.valueType,
    registryPreview: entry.preview,
  };
}

export class RegistryBackendClient {
  private helloResult?: RegistryBackendHello;

  constructor(private readonly transport: RegistryBackendTransport) {}

  async initialize(): Promise<RegistryBackendHello> {
    if (this.helloResult) return this.helloResult;
    const hello = await this.call<RegistryBackendHello>('hello');
    if (hello.protocol !== 1) {
      throw new RegistryBackendError(`Unsupported Registry backend protocol ${hello.protocol}.`, 'hello');
    }
    if (hello.plugin !== 'reassembly.registry') {
      throw new RegistryBackendError(`Unexpected Registry backend identity: ${hello.plugin}.`, 'hello');
    }
    this.helloResult = hello;
    return hello;
  }

  private async call<T>(method: string, params?: Readonly<Record<string, unknown>>): Promise<T> {
    try {
      return await this.transport.call<T>(method, params);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      throw new RegistryBackendError(`Registry backend ${method} failed: ${message}`, method, error);
    }
  }

  async list(path: string): Promise<RegistryListingResult> {
    const result = await this.call<RegistryWireListing>('registry.list', {
      path: normalizeRegistryPath(path),
    });
    return {
      path: normalizeRegistryPath(result.path),
      entries: (result.entries ?? []).map(toRegistryFileInfo),
    };
  }

  read(path: string): Promise<RegistryReadResult> {
    return this.call<RegistryReadResult>('registry.read', {
      path: normalizeRegistryPath(path),
    });
  }

  async stat(path: string): Promise<RegistryFileInfo> {
    const normalized = normalizeRegistryPath(path).replace(/\/+$/, '');
    const body = normalized.slice(ROOT.length);
    const slash = body.lastIndexOf('/');
    const parent = slash < 0 ? ROOT : `${ROOT}${body.slice(0, slash)}`;
    const listing = await this.list(parent);
    const match = listing.entries.find(entry =>
      normalizeRegistryPath(entry.path).replace(/\/+$/, '').toLowerCase() === normalized.toLowerCase()
    );
    if (!match) throw new Error(`Registry entry not found: ${path}`);
    return match;
  }

  createKey(path: string): Promise<unknown> {
    return this.call('registry.createKey', { path: normalizeRegistryPath(path) });
  }

  write(path: string, type: string, data: string): Promise<unknown> {
    return this.call('registry.write', {
      path: normalizeRegistryPath(path),
      type,
      data,
    });
  }

  delete(path: string): Promise<unknown> {
    return this.call('registry.delete', { path: normalizeRegistryPath(path) });
  }

  rename(path: string, newName: string): Promise<unknown> {
    return this.call('registry.rename', {
      path: normalizeRegistryPath(path),
      newName,
    });
  }
}

export function createRegistryPanelSourceHost(
  backend: RegistryBackendClient,
): RegistryPanelSourceHost {
  return {
    getPathKind: path => /^registry:\/\//i.test(path) ? 'registry' : 'unknown',
    listDirectory: (path) => backend.list(path),
    getRootPath: path => /^registry:\/\//i.test(path) ? ROOT : null,
    getParentPath: path => {
      const normalized = normalizeRegistryPath(path).replace(/\/+$/, '');
      if (normalized === 'registry:' || normalized === ROOT.replace(/\/$/, '')) return ROOT;
      const body = normalized.slice(ROOT.length);
      const slash = body.lastIndexOf('/');
      return slash < 0 ? ROOT : `${ROOT}${body.slice(0, slash)}`;
    },
    joinChildPath: (path, childName) => joinRegistryPath([path, childName]),
    supportsStreaming: () => false,
    capabilities: (): RegistrySourceCapabilities => ({
      read: true,
      create: true,
      delete: true,
      rename: true,
      trash: false,
    }),
  };
}

export function createRegistryOperations(
  backend: RegistryBackendClient,
  dialogs: RegistryDialogHost,
): RegistryPluginOperations {
  return {
    joinPaths: async parts => joinRegistryPath(parts),
    createDirectory: path => backend.createKey(path),
    writeFile: (path, content) => backend.write(path, 'REG_SZ', content),
    getFileInfo: path => backend.stat(path),
    moveFile: (source, destination) => {
      const name = destination.replace(/\/+$/, '').split('/').pop() ?? '';
      return backend.rename(source, name);
    },
    readFile: path => backend.read(path),
    writeRegistryValue: (path, type, data) => backend.write(path, type, data),
    deletePath: path => backend.delete(path),
    prompt: request => dialogs.prompt(request),
    confirm: request => dialogs.confirm(request),
  };
}

export const REGISTRY_HIVES = Object.freeze([
  'HKEY_CURRENT_USER',
  'HKEY_LOCAL_MACHINE',
  'HKEY_CLASSES_ROOT',
  'HKEY_USERS',
  'HKEY_CURRENT_CONFIG',
] as const);

export type RegistryHive = typeof REGISTRY_HIVES[number];

export function registryHivePath(hive: RegistryHive): string {
  return `${ROOT}${hive}`;
}

export async function suggestRegistryChildren(
  backend: RegistryBackendClient,
  path: string,
  prefix = '',
): Promise<RegistryFileInfo[]> {
  const listing = await backend.list(path);
  const needle = prefix.trim().toLocaleLowerCase();
  return listing.entries
    .filter(entry => String(entry.registryKind ?? '').toLowerCase() === 'key')
    .filter(entry => !needle || entry.name.toLocaleLowerCase().startsWith(needle))
    .sort((a, b) => a.name.localeCompare(b.name));
}
