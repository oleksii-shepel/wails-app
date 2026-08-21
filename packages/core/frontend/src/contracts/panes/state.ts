import type { FileInfo } from '../files/index.js';
import type { SourceCapabilities } from '../sources/index.js';

export interface PaneState {
  readonly path: string;
  readonly entries: FileInfo[];
  readonly tabs: readonly unknown[];
  readonly activeTab: any;
  readonly activeViewerTab: any;
  readonly activeSurfaceId: string;
  readonly viewMode: string;
  readonly visibleEntries: readonly FileInfo[];
  readonly sourceCapabilities: SourceCapabilities;
  readonly activePanelSource: any;
  readonly mountedViewer: any;
  readonly viewerArchiveContext: any;
  readonly currentArchiveFrame: any;
  readonly pinnableLocation: string;
  readonly pinnableSurfaceId: string;
  readonly pinnableTitle: string;
  readonly hasSelection: boolean;
  readonly canCreate: boolean;
  readonly canRename: boolean;
  readonly canDelete: boolean;
  readonly canCut: boolean;
  readonly canPaste: boolean;
  readonly isInArchive: boolean;
  readonly isDirectoryTab: boolean;
  lastAction: string;
}

export interface PaneContextState {
  readonly path: string;
  readonly entries: readonly FileInfo[];
  readonly visibleEntries: readonly FileInfo[];
  readonly activeSurfaceId: string;
  readonly viewMode: string;
  readonly sourceCapabilities: SourceCapabilities;
  readonly activePanelSource: any;
  readonly currentArchiveFrame: any;
  readonly isDirectoryTab: boolean;
  readonly isInArchive: boolean;
}

export interface PaneTabState {
  readonly tabs: readonly unknown[];
  readonly activeTab: any;
  readonly activeViewerTab: any;
}

export interface PaneViewerState {
  readonly mountedViewer: any;
  readonly viewerArchiveContext: any;
}

export interface PanePinState {
  readonly location: string;
  readonly surfaceId: string;
  readonly title: string;
}

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
