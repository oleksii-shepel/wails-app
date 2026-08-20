export enum RegistryFileType {
  FILE = 'FILE',
  DIRECTORY = 'DIRECTORY',
  ARCHIVE = 'ARCHIVE',
  SYMLINK = 'SYMLINK',
}

export interface RegistryFileInfo {
  name: string;
  path: string;
  /** Host FileType is intentionally structural at the plugin boundary. */
  type: any;
  mimeType?: string;
  size: number;
  created: number;
  modified: number;
  accessed: number;
  permissions: string;
  isHidden: boolean;
  isSymlink?: boolean;
  registryKind?: string;
  registryValueType?: string;
  registryPreview?: string;
  searchBasePath?: string;
  searchPattern?: string;
  searchParentPath?: string;
}

export interface RegistrySourceCapabilities {
  read: boolean;
  create: boolean;
  delete: boolean;
  rename: boolean;
  trash: boolean;
}