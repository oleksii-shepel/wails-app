export interface ProviderCapabilities {
    readonly pagination: boolean;
    readonly randomAccess: boolean;
    readonly totalCount: boolean;
    readonly serverSort: boolean;
    readonly serverFilter: boolean;
    readonly rangeRead: boolean;
    readonly watch: boolean;
    readonly rename: boolean;
    readonly atomicMove: boolean;
    readonly trash: boolean;
}
export declare const UNKNOWN_PROVIDER_CAPABILITIES: ProviderCapabilities;
export declare function normalizeProviderCapabilities(value: Partial<ProviderCapabilities> | null | undefined): ProviderCapabilities;
