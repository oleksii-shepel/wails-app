import type {
  RegistryFileInfo,
  RegistrySourceCapabilities,
} from './registry-model';

export interface RegistryPanelSourceContext {
  readonly path: string;
  readonly entries?: readonly RegistryFileInfo[];
  readonly inArchive?: boolean;
  readonly archiveTopLevel?: boolean;
}

export interface RegistryPanelSourceListing {
  readonly location: {
    readonly path: string;
    readonly sourceId: string;
    readonly sourceKind: 'registry';
  };
  readonly entries: readonly RegistryFileInfo[];
}

export interface RegistryPanelSourceHost {
  getPathKind(path: string, entries: readonly RegistryFileInfo[]): string;
  listDirectory(path: string, showHidden: boolean): Promise<{
    path: string;
    entries: readonly RegistryFileInfo[];
  }>;
  getRootPath(path: string, entries: readonly RegistryFileInfo[]): string | null;
  getParentPath(path: string): string;
  joinChildPath(path: string, childName: string): string;
  supportsStreaming(path: string): boolean;
  capabilities(path: string): RegistrySourceCapabilities;
}

/**
 * Registry-owned PanelSource. Manager supplies only generic directory/VFS
 * capabilities through RegistryPanelSourceHost.
 */
export class RegistryPanelSource {
  readonly id = 'registry';
  readonly kind = 'registry' as const;
  readonly title = 'Registry';

  constructor(
    private readonly host: RegistryPanelSourceHost,
    private readonly activateValue?: (
      context: RegistryPanelSourceContext,
      entry: RegistryFileInfo,
    ) => boolean | Promise<boolean>,
  ) {}

  canHandle(context: RegistryPanelSourceContext): boolean {
    return this.host.getPathKind(
      context.path,
      context.entries ?? [],
    ) === 'registry';
  }

  capabilities(context: RegistryPanelSourceContext): RegistrySourceCapabilities {
    return this.host.capabilities(context.path);
  }

  async list(
    context: RegistryPanelSourceContext,
    showHidden: boolean,
  ): Promise<RegistryPanelSourceListing> {
    const listing = await this.host.listDirectory(context.path, showHidden);
    return {
      location: {
        path: listing.path || context.path,
        sourceId: this.id,
        sourceKind: this.kind,
      },
      entries: listing.entries,
    };
  }

  getRootPath(context: RegistryPanelSourceContext): string | null {
    return this.host.getRootPath(context.path, context.entries ?? []);
  }

  getParentPath(context: RegistryPanelSourceContext): string {
    return this.host.getParentPath(context.path);
  }

  joinChildPath(
    context: RegistryPanelSourceContext,
    childName: string,
  ): string {
    return this.host.joinChildPath(context.path, childName);
  }

  supportsStreaming(context: RegistryPanelSourceContext): boolean {
    return this.host.supportsStreaming(context.path);
  }

  activateEntry(
    context: RegistryPanelSourceContext,
    entry: RegistryFileInfo,
  ): boolean | Promise<boolean> {
    if (entry.path === '__parent__') return false;
    const kind = String(entry.registryKind ?? '').toLowerCase();
    if (kind !== 'value') return false;
    return this.activateValue?.(context, entry) ?? false;
  }

  canPreviewEntry(
    _context: RegistryPanelSourceContext,
    entry: RegistryFileInfo,
  ): boolean {
    const kind = String(entry.registryKind ?? '').toLowerCase();
    return kind !== 'key' && entry.path !== '__parent__';
  }
}