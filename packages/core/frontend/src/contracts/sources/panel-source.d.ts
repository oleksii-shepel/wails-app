import type { FileInfo } from '../files/index.js';
import type { ProviderCapabilities } from './provider-capabilities.js';
import type { SourceCapabilities } from './source-capabilities.js';
export type BuiltinPanelSourceKind = 'filesystem' | 'archive' | 'remote' | 'results' | 'virtual';
export type PanelSourceKind = BuiltinPanelSourceKind | (string & {});
export interface PanelSourceContext<TEntry extends FileInfo = FileInfo> {
    readonly path: string;
    readonly entries?: readonly TEntry[];
    readonly inArchive?: boolean;
    readonly archiveTopLevel?: boolean;
}
export interface PanelSourceLocation<TKind extends PanelSourceKind = PanelSourceKind> {
    readonly path: string;
    readonly sourceId: string;
    readonly sourceKind: TKind;
}
export interface PanelSourceListing<TEntry extends FileInfo = FileInfo, TKind extends PanelSourceKind = PanelSourceKind> {
    readonly location: PanelSourceLocation<TKind>;
    readonly entries: readonly TEntry[];
}
export interface PanelSource<TEntry extends FileInfo = FileInfo, TKind extends PanelSourceKind = PanelSourceKind> {
    readonly id: string;
    readonly kind: TKind;
    readonly title: string;
    canHandle(context: PanelSourceContext<TEntry>): boolean;
    capabilities(context: PanelSourceContext<TEntry>): SourceCapabilities;
    providerCapabilities?(context: PanelSourceContext<TEntry>): ProviderCapabilities | null;
    list(context: PanelSourceContext<TEntry>, showHidden: boolean): Promise<PanelSourceListing<TEntry, TKind>>;
    getRootPath(context: PanelSourceContext<TEntry>): string | null;
    getParentPath(context: PanelSourceContext<TEntry>): string;
    joinChildPath(context: PanelSourceContext<TEntry>, childName: string): string;
    supportsStreaming(context: PanelSourceContext<TEntry>): boolean;
    activateEntry?(context: PanelSourceContext<TEntry>, entry: TEntry): boolean | Promise<boolean>;
    canPreviewEntry?(context: PanelSourceContext<TEntry>, entry: TEntry): boolean;
}
export interface ResolvedPanelSource<TEntry extends FileInfo = FileInfo, TKind extends PanelSourceKind = PanelSourceKind> {
    readonly source: PanelSource<TEntry, TKind>;
    readonly location: PanelSourceLocation<TKind>;
    readonly capabilities: SourceCapabilities;
    readonly providerCapabilities: ProviderCapabilities | null;
}
