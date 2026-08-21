export const UNKNOWN_PROVIDER_CAPABILITIES = Object.freeze({
    pagination: false,
    randomAccess: false,
    totalCount: false,
    serverSort: false,
    serverFilter: false,
    rangeRead: false,
    watch: false,
    rename: false,
    atomicMove: false,
    trash: false,
});
export function normalizeProviderCapabilities(value) {
    return {
        pagination: !!value?.pagination,
        randomAccess: !!value?.randomAccess,
        totalCount: !!value?.totalCount,
        serverSort: !!value?.serverSort,
        serverFilter: !!value?.serverFilter,
        rangeRead: !!value?.rangeRead,
        watch: !!value?.watch,
        rename: !!value?.rename,
        atomicMove: !!value?.atomicMove,
        trash: !!value?.trash,
    };
}
//# sourceMappingURL=provider-capabilities.js.map