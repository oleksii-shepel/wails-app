import type { ArchiveFrame } from './archive.service';
import type { FileInfo } from '../shared/protocol';

export const operationalArchiveEntries = (entries: readonly FileInfo[]) =>
  entries.filter(entry => entry.path !== '__parent__');

export const canMutateArchiveFrame = (frame: ArchiveFrame | null) =>
  !!frame && frame.nestedChain.length === 0;