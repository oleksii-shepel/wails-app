import type { FileInfo } from '../files';

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
  readonly rename: boolean;
  readonly trash: boolean;
  readonly search?: boolean;
}

export const UNKNOWN_PROVIDER_CAPABILITIES: ProviderCapabilities = Object.freeze({
  pagination: false,
  rangeRead: false,
  watch: false,
  rename: false,
  trash: false,
  search: false,
});

export function normalizeProviderCapabilities(
  raw?: Partial<ProviderCapabilities> | null,
): ProviderCapabilities {
  return Object.freeze({
    pagination: !!raw?.pagination,
    rangeRead: !!raw?.rangeRead,
    watch: !!raw?.watch,
    rename: !!raw?.rename,
    trash: !!raw?.trash,
    search: !!raw?.search,
  });
}

export function getSourceCapabilities(
  path: string,
  options?: { readonly inArchive?: boolean },
): SourceCapabilities {
  const normalized = (path ?? '').trim();
  if (options?.inArchive) {
    return Object.freeze({
      read: true,
      create: true,
      delete: true,
      rename: true,
      trash: false,
    });
  }

  const schemeMatch = /^([a-z][a-z0-9+.-]*):\/\//i.exec(normalized);
  const scheme = schemeMatch?.[1]?.toLowerCase() ?? '';

  switch (scheme) {
    case '':
    case 'filesystem':
    case 'file':
      return Object.freeze({
        read: true,
        create: true,
        delete: true,
        rename: true,
        trash: true,
      });
    case 'smb':
    case 'registry':
      return Object.freeze({
        read: true,
        create: true,
        delete: true,
        rename: true,
        trash: false,
      });
    case 'bin':
    case 'autorun':
    case 'network':
    case 'environment':
    case 'tasks':
    case 'services':
    case 'processes':
    case 'devices':
    case 'shell-extensions':
    case 'results':
    case 'search':
    case 'http':
    case 'https':
    case 'gdrive':
    case 'dropbox':
    case 'onedrive':
    case 's3':
      return Object.freeze({
        read: true,
        create: false,
        delete: false,
        rename: false,
        trash: false,
      });
    default:
      return Object.freeze({
        read: true,
        create: false,
        delete: false,
        rename: false,
        trash: false,
      });
  }
}

export type PanelSourceKind =
  | 'filesystem'
  | 'archive'
  | 'remote'
  | 'bin'
  | 'results'
  | 'virtual'
  | 'registry'
  | 'autorun'
  | 'network'
  | 'environment'
  | 'system-table'
  | (string & {});

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
  providerCapabilities?(context: PanelSourceContext): ProviderCapabilities;
  list(context: PanelSourceContext, showHidden: boolean): Promise<PanelSourceListing>;
  getRootPath(context: PanelSourceContext): string | null;
  getParentPath(context: PanelSourceContext): string;
  joinChildPath(context: PanelSourceContext, childName: string): string;
  supportsStreaming(context: PanelSourceContext): boolean;
  activateEntry?(context: PanelSourceContext, entry: FileInfo): boolean | Promise<boolean>;
  canPreviewEntry?(context: PanelSourceContext, entry: FileInfo): boolean;
}

export interface ResolvedPanelSource {
  readonly source: PanelSource;
  readonly location: PanelSourceLocation;
  readonly capabilities: SourceCapabilities;
  readonly providerCapabilities: ProviderCapabilities | null;
}
