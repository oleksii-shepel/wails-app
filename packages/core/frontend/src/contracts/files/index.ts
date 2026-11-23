export enum FileType {
  FILE = 'FILE',
  DIRECTORY = 'DIRECTORY',
  ARCHIVE = 'ARCHIVE',
  SYMLINK = 'SYMLINK',
}

export interface FileInfo {
  name: string;
  path: string;
  type: FileType;
  mimeType?: string;
  size: number;
  created: number;
  modified: number;
  accessed: number;
  permissions: string;
  owner?: string;
  attributes?: string;
  isHidden: boolean;
  isSymlink?: boolean;
  registryKind?: string;
  registryValueType?: string;
  registryPreview?: string;
  searchBasePath?: string;
  searchPattern?: string;
  searchParentPath?: string;
}
