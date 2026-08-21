import type { FileInfo } from '../files/index.js';
import type { SourceCapabilities } from '../sources/index.js';

export interface PanePeerState {
  readonly path: string;
  readonly entries: readonly FileInfo[];
  readonly cursorEntry: FileInfo | null;
  readonly selectedPaths: ReadonlySet<string>;
  readonly sourceCapabilities: SourceCapabilities;
  readonly isDirectoryTab: boolean;
  readonly isInArchive: boolean;
}

export interface PanePeerNavigation {
  refresh(): Promise<void>;
  canNavigateToLocation(path: string): boolean | Promise<boolean>;
  navigateLinkedLocation(
    path: string,
    options?: { history?: 'record' | 'none'; preserveCursor?: boolean },
  ): Promise<boolean>;
}

export interface PanePeerSelection {
  selectPeerByName(name: string): boolean;
  selectPaths(paths: readonly string[]): void;
}

export interface PanePeerPort {
  readonly peerState: PanePeerState;
  readonly peerNavigation: PanePeerNavigation;
  readonly peerSelection: PanePeerSelection;
}
