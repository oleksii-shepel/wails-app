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
import { ResilienceService } from './resilience.service';

import {
  ResultsPanelService,
  type ResultsPanelMetadata,
} from './results-panel.service';
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

  activateEntry(
    _context: PanelSourceContext,
    _entry: FileInfo,
  ): boolean | Promise<boolean> {
    return false;
  }

  canPreviewEntry(
    _context: PanelSourceContext,
    _entry: FileInfo,
  ): boolean {
    return true;
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

  override async list(
    context: PanelSourceContext,
    _showHidden: boolean,
  ): Promise<PanelSourceListing> {
    return {
      location: {
        path: context.path,
        sourceId: this.id,
        sourceKind: this.kind,
      },
      entries: [...(context.entries ?? [])],
    };
  }

  override supportsStreaming(): boolean {
    return false;
  }
}

export class RemotePanelSource extends BasePanelSource {
  readonly kind = 'remote' as const;

  constructor(
    directories: DirectorySourceService,
    private readonly remote: RemoteConnectionsService,
    private readonly resilience?: ResilienceService,
    readonly id = 'remote',
    readonly title = 'Remote',
    private readonly schemes: readonly string[] | null = null,
  ) {
    super(directories);
  }

  canHandle(context: PanelSourceContext): boolean {
    if (!this.remote.isRemoteUri(context.path)) return false;
    if (!this.schemes?.length) return true;
    const scheme = schemeOf(context.path);
    return !!scheme && this.schemes.includes(scheme);
  }

  override async list(
    context: PanelSourceContext,
    showHidden: boolean,
  ): Promise<PanelSourceListing> {
    const mount = this.remote.findByUri(context.path);
    const key = mount
      ? `provider:${this.remote.protocol(mount)}:${this.remote.rootUri(mount).toLocaleLowerCase()}`
      : `provider:${schemeOf(context.path) ?? 'remote'}:${context.path}`;

    try {
      const listing = this.resilience
        ? await this.resilience.execute(
            key,
            () => super.list(context, showHidden),
            {
              policy: {
                maxAttempts: 3,
                initialDelayMs: 180,
                maxDelayMs: 1800,
                multiplier: 2,
                jitter: 0.2,
              },
            },
          )
        : await super.list(context, showHidden);

      this.remote.reportSuccess(context.path);
      return listing;
    } catch (error) {
      this.remote.reportFailure(context.path, error);
      // The caller fails this pane load only. No shared lock/global promise is
      // held, so another pane/provider continues independently.
      throw error;
    }
  }

  capabilities(context: PanelSourceContext): SourceCapabilities {
    return this.remote.capabilitiesFor(
      context.path,
      getSourceCapabilities(context.path),
    );
  }

  providerCapabilities(context: PanelSourceContext) { return this.remote.providerCapabilitiesFor(context.path); }

  override getRootPath(context: PanelSourceContext): string | null {
    const scheme = schemeOf(context.path);
    if (!scheme) return null;
    const mount = this.remote.findByUri(context.path);
    if (mount?.publicPath) return String(mount.publicPath);
    return `${scheme}://`;
  }

  override getParentPath(context: PanelSourceContext): string {
    const root = this.getRootPath(context);
    if (!root || context.path === root) return root ?? context.path;

    const mount = this.remote.findByUri(context.path);
    const mountRoot = String(mount?.publicPath ?? '').replace(/\/+$/, '');
    const current = context.path.replace(/\/+$/, '');

    if (mountRoot && current === mountRoot) {
      return `${schemeOf(context.path)}://`;
    }
    if (mountRoot && current.startsWith(`${mountRoot}/`)) {
      const slash = current.lastIndexOf('/');
      return slash > mountRoot.length ? current.slice(0, slash) : mountRoot;
    }
    return super.getParentPath(context);
  }

  override joinChildPath(
    context: PanelSourceContext,
    childName: string,
  ): string {
    const hierarchy = this.remote.hierarchyFor(context.path);
    if (hierarchy === 'share-path' || hierarchy === 'path-backed') {
      return `${context.path.replace(/\/+$/, '')}/${childName}`;
    }
    return super.joinChildPath(context, childName);
  }
}


class BinPanelSource extends BasePanelSource {
  readonly id = 'bin';
  readonly kind = 'bin' as const;
  readonly title = 'Recycle Bin';

  canHandle(context: PanelSourceContext): boolean {
    return schemeOf(context.path) === 'bin';
  }

  capabilities(_context: PanelSourceContext): SourceCapabilities {
    return { read: true, create: false, delete: false, rename: false, trash: false };
  }

  override getRootPath(): string { return 'bin://'; }
  override getParentPath(): string { return 'bin://'; }
  override joinChildPath(): string { return 'bin://'; }
  override supportsStreaming(): boolean { return false; }

  override activateEntry(): boolean {
    // Recycle-bin entries are shell objects, not navigable filesystem paths.
    // Consume activation so the pane never falls back to native-path handling.
    return true;
  }

  override canPreviewEntry(): boolean { return false; }
}

class ResultsPanelSource extends BasePanelSource {
  readonly id = 'results';
  readonly kind = 'results' as const;
  readonly title = 'Results';

  constructor(
    directories: DirectorySourceService,
    private readonly results: ResultsPanelService,
  ) {
    super(directories);
  }

  canHandle(context: PanelSourceContext): boolean {
    return /^results:\/\//i.test(context.path);
  }

  capabilities(_context: PanelSourceContext): SourceCapabilities {
    return {
      read: true,
      create: false,
      delete: false,
      rename: false,
      trash: false,
    };
  }

  override async list(
    context: PanelSourceContext,
    _showHidden: boolean,
  ): Promise<PanelSourceListing> {
    const panel = this.results.get(context.path);

    return {
      location: {
        path: context.path,
        sourceId: panel ? `results.${panel.id}` : 'results.expired',
        sourceKind: this.kind,
      },
      entries: panel ? panel.entries : [],
    };
  }

  override getRootPath(context: PanelSourceContext): string | null {
    return context.path;
  }

  override getParentPath(context: PanelSourceContext): string {
    return context.path;
  }

  override joinChildPath(
    context: PanelSourceContext,
    _childName: string,
  ): string {
    return context.path;
  }

  override supportsStreaming(_context: PanelSourceContext): boolean {
    return false;
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
  private readonly sources: PanelSource[];

  constructor(
    private readonly directories: DirectorySourceService,
    remoteConnections: RemoteConnectionsService,
    private readonly resultsPanels: ResultsPanelService,
    resilience?: ResilienceService,
  ) {
    this.sources = [
      new ArchivePanelSource(this.directories),
      new BinPanelSource(this.directories),
      new RemotePanelSource(this.directories, remoteConnections, resilience),
      new ResultsPanelSource(this.directories, this.resultsPanels),
      new VirtualPanelSource(this.directories),
      new FilesystemPanelSource(this.directories),
    ];
  }

  register(source: PanelSource): void {
    if (!source?.id?.trim()) throw new Error('PanelSource id is required.');
    if (source.id === 'filesystem') {
      throw new Error('The filesystem fallback PanelSource cannot be replaced.');
    }
    this.unregister(source.id);
    // Contributions resolve before ordinary built-ins, while an active archive frame
    // remains the strongest source identity and filesystem remains the fallback.
    const archiveIndex = this.sources.findIndex(candidate => candidate.id === 'archive');
    this.sources.splice(archiveIndex >= 0 ? archiveIndex + 1 : 0, 0, source);
  }

  unregister(id: string): void {
    const index = this.sources.findIndex(source => source.id === id && source.id !== 'filesystem');
    if (index >= 0) this.sources.splice(index, 1);
  }

  listSources(): readonly PanelSource[] {
    return [...this.sources];
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
      providerCapabilities: source.providerCapabilities?.(context) ?? null,
    };
  }

  createResultsPanel(
    title: string,
    entries: readonly FileInfo[],
    metadata: ResultsPanelMetadata = {},
  ): string {
    return this.resultsPanels.create(
      title,
      entries,
      metadata,
    ).path;
  }

  createSearchResultsPanel(
    title: string,
    entries: readonly FileInfo[],
    rootPath: string,
    query: string,
    recursive = false,
  ): string {
    return this.resultsPanels.createSearch(
      title, entries, rootPath, query, recursive,
    ).path;
  }

  titleFor(path: string): string | null {
    if (/^results:\/\//i.test(path)) {
      return this.resultsPanels.title(path);
    }
    return null;
  }

  isRootPath(
    path: string,
    entries: readonly FileInfo[] = [],
  ): boolean {
    const resolved = this.resolve({
      path,
      entries,
      inArchive: false,
    });

    const root = resolved.source.getRootPath({
      path,
      entries,
      inArchive: false,
    });

    return !!root && root === path;
  }

  getParentPath(
    path: string,
    entries: readonly FileInfo[] = [],
  ): string {
    const context: PanelSourceContext = {
      path,
      entries,
      inArchive: false,
    };
    return this.resolve(context).source.getParentPath(context);
  }

  streamMode(
    path: string,
    entries: readonly FileInfo[] = [],
  ): 'backend' | 'results' | null {
    if (/^results:\/\//i.test(path)) {
      return this.resultsPanels.has(path) ? 'results' : null;
    }

    const context: PanelSourceContext = {
      path,
      entries,
      inArchive: false,
    };
    const resolved = this.resolve(context);

    return resolved.source.supportsStreaming(context)
      ? 'backend'
      : null;
  }

  resultsPanelCount(path: string): number {
    return this.resultsPanels.count(path);
  }

  resultsPanelSearchAlias(path: string): string | null {
    return this.resultsPanels.backendSearchAlias(path);
  }

  readResultsPanelWindow(
    path: string,
    offset: number,
    limit: number,
    options?: {
      readonly sort?: 'relevance' | 'name' | 'path' | 'type';
      readonly filterText?: string;
    },
  ) {
    return this.resultsPanels.readWindow(path, offset, limit, options);
  }

  async streamResultsPanel(
    path: string,
    options: {
      readonly batchSize?: number;
      readonly signal?: AbortSignal;
      readonly onBatch: (
        batch: readonly FileInfo[],
        loaded: number,
        total: number,
      ) => void;
    },
  ): Promise<number> {
    return this.resultsPanels.stream(path, options);
  }

  async listContext(
    context: PanelSourceContext,
    showHidden: boolean,
  ): Promise<PanelSourceListing> {
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

  async list(
    path: string,
    showHidden: boolean,
    entries: readonly FileInfo[] = [],
  ): Promise<PanelSourceListing> {
    return this.listContext({
      path,
      entries,
      inArchive: false,
    }, showHidden);
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
        return source.id !== 'remote'
          ? source.id
          : (scheme ? `remote.${scheme}` : source.id);
      case 'results': {
        const panel = this.resultsPanels.get(path);
        return panel ? `results.${panel.id}` : 'results.expired';
      }
      case 'virtual':
        return source.id === 'virtual'
          ? (scheme ? `virtual.${scheme}` : source.id)
          : source.id;
      default:
        return source.id;
    }
  }
}
