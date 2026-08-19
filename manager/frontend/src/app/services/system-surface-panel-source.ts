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

  getParentPath(): string { return rootFor(this.descriptor.kind); }

  joinChildPath(_context: PanelSourceContext, childName: string): string {
    return stableLeafPath(this.descriptor.kind, childName);
  }

  canPreviewEntry(): boolean { return false; }

  // Synthetic resources are selected and acted on through commands/context menu.
  // They are not paths that should enter the generic file viewer.
  activateEntry(): boolean { return true; }

  async list(context: PanelSourceContext): Promise<PanelSourceListing> {
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
    if (!Array.isArray(raw)) return [];
    return raw
      .filter(record => record && typeof record === 'object')
      .map((record: any, index: number) => ({
        id: String(record.id ?? record.ID ?? index),
        name: String(record.name ?? record.Name ?? record.id ?? record.ID ?? index),
        fields: (record.fields ?? record.Fields ?? {}) as Record<string, unknown>,
      }));
  }

  private toFileInfo(record: SystemSurfaceRecord): SystemSurfaceFileInfo {
    const fields = record.fields ?? {};
    return {
      name: record.name,
      path: stableLeafPath(this.descriptor.kind, record.id),
      type: FileType.FILE,
      size: Number(fields['size'] ?? fields['workingSet'] ?? 0) || 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: 'r--',
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