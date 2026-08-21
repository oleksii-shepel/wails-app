import type { FileInfo } from '../files/index.js';

export interface ArchiveTransferError {
  readonly source: string;
  readonly error: string;
}

export interface ArchiveTransferResult {
  readonly operationId?: string;
  readonly total: number;
  readonly succeeded: number;
  readonly failed: number;
  readonly sourceRemoved?: boolean;
  readonly errors?: readonly ArchiveTransferError[];
}

export interface PaneViewerCommands {
  openEntry(entry: FileInfo): void | Promise<void>;
  openViewer(entry: FileInfo, viewerId?: string | null, autoFollow?: boolean): any;
  openSelectedWithViewer(viewerId: string): Promise<void>;
  openQuickView(entry: FileInfo): void;
  openActiveViewerExternally(): Promise<void>;
  returnToViewerSource(): Promise<void>;
  openResultsPanel(
    title: string,
    entries: readonly FileInfo[],
    metadata?: unknown,
  ): Promise<any>;
  openRecentFilesPanel(): Promise<void>;
  openSurfaceTab(surfaceId: string, title?: string, location?: string): Promise<void>;
}

export interface PaneCommands {
  copySelected(): void;
  cutSelected(): void;
  paste(): Promise<void>;
  deleteSelected(permanent?: boolean): Promise<void>;
  renameSelected(): Promise<void>;
  newFolder(): Promise<void>;
  newFile(): Promise<void>;
  newDocumentTemplate(kind: string): Promise<void>;
  moveSelectionToNewFolder(): Promise<void>;
  duplicateSelected(): Promise<void>;
  revealSelectedInFileManager(): Promise<void>;
  touchSelectedModifiedTime(): Promise<void>;
  editSelectedAttributes(): Promise<void>;
  showSelectedProperties(): Promise<void>;
  showSelectedChecksum(): Promise<void>;
  copySelectedPaths(): Promise<void>;
  copySelectedRelativePaths(): Promise<void>;
  copySelectedFileUris(): Promise<void>;
  copySelectedNames(): Promise<void>;
  createLinkForSelected(kind: 'symbolic' | 'hard'): Promise<void>;
  createShortcutForSelected(): Promise<void>;
  createZipFromSelection(): Promise<void>;
  extractSelectedTo(destinationDir: string, flattenPaths?: boolean): Promise<boolean>;
  transferArchiveSelectionTo(
    destinationDir: string,
    operation: 'copy' | 'move',
  ): Promise<ArchiveTransferResult | null>;
  treeLeft(): void;
  treeRight(): void;
}
