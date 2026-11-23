import type { FileInfo } from '../files/index';

export interface SourceCapabilities {
  readonly read: boolean;
  readonly create: boolean;
  readonly delete: boolean;
  readonly rename: boolean;
  readonly trash: boolean;
}

export interface ProviderCapabilities {
  readonly pagination: boolean;
  readonly rangeRead: boolean;
  readonly watch: boolean;
  readonly create: boolean;
  readonly delete: boolean;
  readonly rename: boolean;
  readonly copy: boolean;
  readonly move: boolean;
  readonly search: boolean;
  readonly hash: boolean;
  readonly trash: boolean;
}

export const UNKNOWN_PROVIDER_CAPABILITIES: ProviderCapabilities = Object.freeze({
  pagination: false,
  rangeRead: false,
  watch: false,
  create: false,
  delete: false,
  rename: false,
  copy: false,
  move: false,
  search: false,
  hash: false,
  trash: false,
});

export function normalizeProviderCapabilities(
  raw?: Partial<Record<keyof ProviderCapabilities, unknown>> | null,
): ProviderCapabilities {
  if (!raw || typeof raw !== 'object') return UNKNOWN_PROVIDER_CAPABILITIES;
  return {
    pagination: raw.pagination === true,
    rangeRead: raw.rangeRead === true,
    watch: raw.watch === true,
    create: raw.create === true,
    delete: raw.delete === true,
    rename: raw.rename === true,
    copy: raw.copy === true,
    move: raw.move === true,
    search: raw.search === true,
    hash: raw.hash === true,
    trash: raw.trash === true,
  };
}

export function getSourceCapabilities(
  location: string,
  options?: { inArchive?: boolean },
): SourceCapabilities {
  const trimmed = (location ?? '').trim().toLowerCase();
  if (options?.inArchive) {
    return { read: true, create: true, delete: true, rename: true, trash: false };
  }
  const match = /^([a-z][a-z0-9+.-]*):\/\//.exec(trimmed);
  const scheme = match?.[1] ?? '';

  if (scheme === 'smb') {
    return { read: true, create: true, delete: true, rename: true, trash: false };
  }
  if (scheme === 'registry') {
    return { read: true, create: true, delete: true, rename: true, trash: false };
  }
  if (
    scheme === 'gdrive' ||
    scheme === 'dropbox' ||
    scheme === 'onedrive' ||
    scheme === 's3' ||
    scheme === 'search' ||
    scheme === 'http' ||
    scheme === 'https'
  ) {
    return { read: true, create: false, delete: false, rename: false, trash: false };
  }
  if (scheme === 'bin') {
    return { read: true, create: false, delete: true, rename: false, trash: false };
  }
  if (scheme === 'autorun' || scheme === 'network' || scheme === 'environment') {
    return { read: true, create: false, delete: false, rename: false, trash: false };
  }
  return { read: true, create: true, delete: true, rename: true, trash: true };
}

export type PanelSourceKind =
  | 'filesystem'
  | 'archive'
  | 'remote'
  | 'search'
  | 'results'
  | 'system'
  | 'plugin'
  | string;

export interface PanelSourceContext {
  readonly path: string;
  readonly entries?: readonly FileInfo[];
  readonly inArchive?: boolean;
  readonly archiveTopLevel?: boolean;
}

export interface PanelSourceLocation {
  readonly path: string;
  readonly sourceId: string;
  readonly sourceKind: PanelSourceKind;
}

export interface PanelSourceListing {
  readonly location: PanelSourceLocation;
  readonly entries: readonly FileInfo[];
}

export interface PanelSource {
  readonly id: string;
  readonly kind: PanelSourceKind;
  readonly title: string;
  canHandle(context: PanelSourceContext): boolean;
  capabilities(context: PanelSourceContext): SourceCapabilities;
  list(context: PanelSourceContext, showHidden: boolean): Promise<PanelSourceListing>;
  getRootPath(context: PanelSourceContext): string | null;
  getParentPath(context: PanelSourceContext): string;
  joinChildPath?(context: PanelSourceContext, childName: string): string;
  supportsStreaming(context: PanelSourceContext): boolean;
  activateEntry?(context: PanelSourceContext, entry: FileInfo): boolean | Promise<boolean>;
  canPreviewEntry?(context: PanelSourceContext, entry: FileInfo): boolean;
  providerCapabilities?(context: PanelSourceContext): ProviderCapabilities | null;
}

export interface ResolvedPanelSource {
  readonly source: PanelSource;
  readonly location: PanelSourceLocation;
  readonly capabilities: SourceCapabilities;
  readonly providerCapabilities?: ProviderCapabilities | null;
}
