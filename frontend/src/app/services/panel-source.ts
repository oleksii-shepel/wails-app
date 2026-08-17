import type { FileInfo } from '../shared/protocol-enhanced';
import type { SourceCapabilities } from './source-capabilities';

export type PanelSourceKind =
  | 'filesystem'
  | 'archive'
  | 'registry'
  | 'remote'
  | 'results'
  | 'virtual';

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

  list(
    context: PanelSourceContext,
    showHidden: boolean,
  ): Promise<PanelSourceListing>;

  getRootPath(context: PanelSourceContext): string | null;
  getParentPath(context: PanelSourceContext): string;
  joinChildPath(context: PanelSourceContext, childName: string): string;
  supportsStreaming(context: PanelSourceContext): boolean;

  /** Optional provider-owned activation for synthetic leaf entries. */
  activateEntry?(context: PanelSourceContext, entry: FileInfo): boolean;

  /** Return false when an entry is synthetic and must never enter a file viewer. */
  canPreviewEntry?(context: PanelSourceContext, entry: FileInfo): boolean;
}

export interface ResolvedPanelSource {
  readonly source: PanelSource;
  readonly location: PanelSourceLocation;
  readonly capabilities: SourceCapabilities;
}