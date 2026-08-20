import type { SourceCapabilities } from '../services/source-capabilities';
import type { FileInfo } from '../shared/protocol';

/** Location/surface state used by the manager shell. */
export interface PaneContextState {
  readonly path: string;
  readonly entries: readonly FileInfo[];
  readonly visibleEntries: readonly FileInfo[];
  readonly activeSurfaceId: string;
  readonly viewMode: string;
  readonly sourceCapabilities: SourceCapabilities;
  readonly activePanelSource: unknown;
  readonly currentArchiveFrame: unknown;
  readonly isDirectoryTab: boolean;
  readonly isInArchive: boolean;
}

/** Tab metadata only; tab mutation belongs to PaneTabs. */
export interface PaneTabState {
  readonly tabs: readonly unknown[];
  readonly activeTab: any;
  readonly activeViewerTab: any;
}

/** Mounted viewer metadata only; viewer actions belong to PaneViewerCommands. */
export interface PaneViewerState {
  readonly mountedViewer: any;
  readonly viewerArchiveContext: unknown;
}

export interface PanePinState {
  readonly location: string;
  readonly surfaceId: string;
  readonly title: string;
}

/** Capability snapshot; mutation actions belong to PaneCommands. */
export interface PaneMutationState {
  readonly hasSelection: boolean;
  readonly canCreate: boolean;
  readonly canRename: boolean;
  readonly canDelete: boolean;
  readonly canCut: boolean;
  readonly canPaste: boolean;
}

export interface PaneStatePort {
  readonly contextState: PaneContextState;
  readonly tabState: PaneTabState;
  readonly viewerState: PaneViewerState;
  readonly pinState: PanePinState;
  readonly mutationState: PaneMutationState;
}
