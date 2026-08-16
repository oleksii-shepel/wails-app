import { Injectable } from '@angular/core';

import type { FileInfo } from '../shared/protocol-enhanced';
import { DirectorySourceService } from './directory-source.service';
import {
  type PanelSource,
  type PanelSourceContext,
  type PanelSourceKind,
  type PanelSourceListing,
  type ResolvedPanelSource,
} from './panel-source';
import { RemoteConnectionsService } from './remote-connections.service';
import {
  getSourceCapabilities,
  type SourceCapabilities,
} from './source-capabilities';

const ARCHIVE_READ_ONLY: SourceCapabilities = Object.freeze({
  read: true,
  create: false,
  delete: false,
  rename: false,
  trash: false,
});

function schemeOf(path: string): string | null {
  const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec((path ?? '').trim());
  return match?.[1]?.toLowerCase() ?? null;
}

abstract class BasePanelSource implements PanelSource {
  abstract readonly id: string;
  abstract readonly kind: PanelSourceKind;
  abstract readonly title: string;

  constructor(
    protected readonly directories: DirectorySourceService,
  ) {}

  abstract canHandle(context: PanelSourceContext): boolean;
  abstract capabilities(context: PanelSourceContext): SourceCapabilities;

  async list(
    context: PanelSourceContext,
    showHidden: boolean,
  ): Promise<PanelSourceListing> {
    const listing = await this.directories.listDirectory(
      context.path,
      showHidden,
    );

    return {
      location: {
        path: listing.path || context.path,
        sourceId: this.id,
        sourceKind: this.kind,
      },
      entries: listing.entries,
    };
  }

  getRootPath(context: PanelSourceContext): string | null {
    return this.directories.getRootPath(
      context.path,
      [...(context.entries ?? [])],
    );
  }

  getParentPath(context: PanelSourceContext): string {
    return this.directories.getParentPath(context.path);
  }

  joinChildPath(
    context: PanelSourceContext,
    childName: string,
  ): string {
    return this.directories.joinChildPath(context.path, childName);
  }

  supportsStreaming(context: PanelSourceContext): boolean {
    return this.directories.supportsStreaming(context.path);
  }
}

class ArchivePanelSource extends BasePanelSource {
  readonly id = 'archive';
  readonly kind = 'archive' as const;
  readonly title = 'Archive';

  canHandle(context: PanelSourceContext): boolean {
    return !!context.inArchive;
  }

  capabilities(context: PanelSourceContext): SourceCapabilities {
    return {
      ...ARCHIVE_READ_ONLY,
      delete: !!context.archiveTopLevel,
    };
  }

  override async list(): Promise<PanelSourceListing> {
    throw new Error(
      'Archive listing is owned by the archive frame navigator.',
    );
  }

  override supportsStreaming(): boolean {
    return false;
  }
}

class RegistryPanelSource extends BasePanelSource {
  readonly id = 'registry';
  readonly kind = 'registry' as const;
  readonly title = 'Registry';

  canHandle(context: PanelSourceContext): boolean {
    return this.directories.getPathKind(
      context.path,
      [...(context.entries ?? [])],
    ) === 'registry';
  }

  capabilities(context: PanelSourceContext): SourceCapabilities {
    return getSourceCapabilities(context.path);
  }
}

class RemotePanelSource extends BasePanelSource {
  readonly id = 'remote';
  readonly kind = 'remote' as const;
  readonly title = 'Remote';

  constructor(
    directories: DirectorySourceService,
    private readonly remote: RemoteConnectionsService,
  ) {
    super(directories);
  }

  canHandle(context: PanelSourceContext): boolean {
    return this.remote.isRemoteUri(context.path);
  }

  capabilities(context: PanelSourceContext): SourceCapabilities {
    return this.remote.capabilitiesFor(
      context.path,
      getSourceCapabilities(context.path),
    );
  }
}

class VirtualPanelSource extends BasePanelSource {
  readonly id = 'virtual';
  readonly kind = 'virtual' as const;
  readonly title = 'Virtual';

  canHandle(context: PanelSourceContext): boolean {
    const scheme = schemeOf(context.path);
    return scheme === 'search' || scheme === 'http' || scheme === 'https';
  }

  capabilities(context: PanelSourceContext): SourceCapabilities {
    return getSourceCapabilities(context.path);
  }
}

class FilesystemPanelSource extends BasePanelSource {
  readonly id = 'filesystem';
  readonly kind = 'filesystem' as const;
  readonly title = 'Files';

  canHandle(): boolean {
    return true;
  }

  capabilities(context: PanelSourceContext): SourceCapabilities {
    return getSourceCapabilities(context.path);
  }
}

@Injectable({ providedIn: 'root' })
export class PanelSourceRegistryService {
  private readonly sources: readonly PanelSource[];

  constructor(
    private readonly directories: DirectorySourceService,
    remoteConnections: RemoteConnectionsService,
  ) {
    this.sources = [
      new ArchivePanelSource(this.directories),
      new RegistryPanelSource(this.directories),
      new RemotePanelSource(this.directories, remoteConnections),
      new VirtualPanelSource(this.directories),
      new FilesystemPanelSource(this.directories),
    ];
  }

  resolve(context: PanelSourceContext): ResolvedPanelSource {
    const source =
      this.sources.find(candidate => candidate.canHandle(context)) ??
      this.sources[this.sources.length - 1];

    return {
      source,
      location: {
        path: context.path,
        sourceId: this.sourceId(source, context.path),
        sourceKind: source.kind,
      },
      capabilities: source.capabilities(context),
    };
  }

  async list(
    path: string,
    showHidden: boolean,
    entries: readonly FileInfo[] = [],
  ): Promise<PanelSourceListing> {
    const context: PanelSourceContext = {
      path,
      entries,
      inArchive: false,
    };

    const resolved = this.resolve(context);
    const listing = await resolved.source.list(context, showHidden);

    return {
      ...listing,
      location: {
        ...listing.location,
        sourceId: this.sourceId(resolved.source, listing.location.path),
      },
    };
  }

  async listDirectory(
    path: string,
    showHidden: boolean,
    entries: readonly FileInfo[] = [],
  ): Promise<{ path: string; entries: FileInfo[] }> {
    const listing = await this.list(path, showHidden, entries);
    return {
      path: listing.location.path,
      entries: [...listing.entries],
    };
  }

  private sourceId(source: PanelSource, path: string): string {
    const scheme = schemeOf(path);

    switch (source.kind) {
      case 'filesystem':
        return !scheme || scheme === 'filesystem'
          ? 'filesystem.local'
          : `filesystem.${scheme}`;
      case 'remote':
        return scheme ? `remote.${scheme}` : source.id;
      case 'virtual':
        return scheme ? `virtual.${scheme}` : source.id;
      default:
        return source.id;
    }
  }
}
