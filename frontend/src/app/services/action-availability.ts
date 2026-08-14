import type { SourceCapabilities } from './source-capabilities';

export interface ActionAvailability {
  readonly createFolder: boolean;
  readonly deleteSelected: boolean;
  readonly copySelected: boolean;
  readonly cutSelected: boolean;
  readonly paste: boolean;
  readonly rename: boolean;
  readonly copyToOther: boolean;
  readonly moveToOther: boolean;
}

export function buildActionAvailability(input: {
  source: SourceCapabilities;
  destination?: SourceCapabilities | null;
  selectedCount: number;
  hasClipboard: boolean;
  clipboardOperation?: 'copy' | 'cut' | null;
}): ActionAvailability {
  const hasSelection = input.selectedCount > 0;
  const destinationWritable = !!input.destination?.create;
  return Object.freeze({
    createFolder: input.source.create,
    deleteSelected: hasSelection && input.source.delete,
    copySelected: hasSelection && input.source.read,
    cutSelected: hasSelection && input.source.read && input.source.delete,
    paste: input.hasClipboard && input.source.create,
    rename: hasSelection && input.source.rename,
    copyToOther: hasSelection && input.source.read && destinationWritable,
    moveToOther: hasSelection && input.source.read && input.source.delete && destinationWritable,
  });
}

export const NO_ACTIONS: ActionAvailability = Object.freeze({
  createFolder: false, deleteSelected: false, copySelected: false, cutSelected: false,
  paste: false, rename: false, copyToOther: false, moveToOther: false,
});
