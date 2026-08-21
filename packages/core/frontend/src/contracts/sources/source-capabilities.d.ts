export interface SourceCapabilities {
    readonly read: boolean;
    readonly create: boolean;
    readonly delete: boolean;
    readonly rename: boolean;
    readonly trash: boolean;
}
export declare function getSourceCapabilities(location: string, options?: {
    inArchive?: boolean;
}): SourceCapabilities;
