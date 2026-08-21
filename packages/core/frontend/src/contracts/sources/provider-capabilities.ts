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

export const UNKNOWN_PROVIDER_CAPABILITIES: ProviderCapabilities = Object.freeze({
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

export function normalizeProviderCapabilities(
  value: Partial<ProviderCapabilities> | null | undefined,
): ProviderCapabilities {
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
