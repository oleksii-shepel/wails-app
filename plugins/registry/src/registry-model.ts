import type { FileInfo } from '@reassembly/core/contracts/files';
import type { SourceCapabilities } from '@reassembly/core/contracts/sources';

export enum RegistryFileType {
  FILE = 'FILE',
  DIRECTORY = 'DIRECTORY',
  ARCHIVE = 'ARCHIVE',
  SYMLINK = 'SYMLINK',
}

export type RegistryFileInfo = FileInfo;
export type RegistrySourceCapabilities = SourceCapabilities;
