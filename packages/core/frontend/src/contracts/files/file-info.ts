/**
 * Canonical file entry contract shared by Manager, plugins, pane contracts,
 * and panel sources.
 *
 * Keep this structurally aligned with the Wails FileInfo wire model. Provider-
 * specific metadata belongs in explicit optional fields or derived interfaces;
 * do not add a catch-all index signature because it makes the contract stricter
 * for ordinary FileInfo values.
 */
export enum FileType {
  FILE = 'FILE',
  DIRECTORY = 'DIRECTORY',
  ARCHIVE = 'ARCHIVE',
  SYMLINK = 'SYMLINK',
}

/** Backward-compatible name used by early core consumers. */
export type FileEntryType = FileType;

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
  isHidden: boolean;
  isSymlink?: boolean;

  // Shared provider metadata already consumed by Manager/plugin surfaces.
  registryKind?: string;
  registryValueType?: string;
  registryPreview?: string;
  searchBasePath?: string;
  searchPattern?: string;
  searchParentPath?: string;

  // Archive/provider extensions. Optional so plain wire FileInfo remains valid.
  isNested?: boolean;
  nestedChain?: string[];
}

export interface FileProbe {
  mimeType: string;
  encoding?: string;
  isText: boolean;
  isArchive: boolean;
}
