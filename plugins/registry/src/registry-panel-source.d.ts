import type { PanelSource, PanelSourceContext, PanelSourceListing } from '@reassembly/core/contracts/sources';
import type { RegistryFileInfo, RegistrySourceCapabilities } from './registry-model';
export type RegistryPanelSourceContext = PanelSourceContext<RegistryFileInfo>;
export type RegistryPanelSourceListing = PanelSourceListing<RegistryFileInfo, 'registry'>;
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
export declare class RegistryPanelSource implements PanelSource<RegistryFileInfo, 'registry'> {
    private readonly host;
    private readonly activateValue?;
    readonly id = "registry";
    readonly kind: "registry";
    readonly title = "Registry";
    constructor(host: RegistryPanelSourceHost, activateValue?: ((context: RegistryPanelSourceContext, entry: RegistryFileInfo) => boolean | Promise<boolean>) | undefined);
    canHandle(context: RegistryPanelSourceContext): boolean;
    capabilities(context: RegistryPanelSourceContext): RegistrySourceCapabilities;
    list(context: RegistryPanelSourceContext, showHidden: boolean): Promise<RegistryPanelSourceListing>;
    getRootPath(context: RegistryPanelSourceContext): string | null;
    getParentPath(context: RegistryPanelSourceContext): string;
    joinChildPath(context: RegistryPanelSourceContext, childName: string): string;
    supportsStreaming(context: RegistryPanelSourceContext): boolean;
    activateEntry(context: RegistryPanelSourceContext, entry: RegistryFileInfo): boolean | Promise<boolean>;
    canPreviewEntry(_context: RegistryPanelSourceContext, entry: RegistryFileInfo): boolean;
}
