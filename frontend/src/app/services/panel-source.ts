import type { FileInfo } from '../shared/protocol-enhanced';
import type { SourceCapabilities } from './source-capabilities';

export type PanelSourceKind =
  | 'filesystem'
  | 'archive'
  | 'registry'
  | 'remote'
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
}

export interface ResolvedPanelSource {
  readonly source: PanelSource;
  readonly location: PanelSourceLocation;
  readonly capabilities: SourceCapabilities;
}
