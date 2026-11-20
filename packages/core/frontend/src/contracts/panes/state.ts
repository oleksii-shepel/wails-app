import type { FileInfo } from '../files';
import type { SourceCapabilities } from '../sources';

export type DirectoryView = 'list' | 'grid' | 'details' | 'tree';
export type SortField = 'name' | 'size' | 'modified' | 'type';
export type SortDirection = 'asc' | 'desc';

export interface PanePresentation {
  readonly viewMode: DirectoryView;
  readonly sortField: SortField;
  readonly sortDirection: SortDirection;
  readonly directoriesFirst: boolean;
  readonly filterText: string;
  readonly filterVisible: boolean;
}

export interface PaneViewportSnapshot {
  readonly topName: string | null;
  readonly ratio: number;
}

export interface QuickViewSource {
  readonly paneId?: 'left' | 'right';
  readonly sourcePane: 'left' | 'right';
  readonly sourceTabId: string;
  readonly sourceDir: string;
  readonly entry?: FileInfo;
  readonly fileList?: readonly { readonly path: string; readonly name: string }[];
}

export interface PaneContextState {
  readonly path: string;
  readonly entries: readonly FileInfo[];
  readonly visibleEntries: readonly FileInfo[];
  readonly activeSurfaceId: string;
  readonly viewMode: DirectoryView;
  readonly sourceCapabilities: SourceCapabilities;
  readonly activePanelSource: any;
  readonly currentArchiveFrame: any;
  readonly isDirectoryTab: boolean;
  readonly isInArchive: boolean;
}

export interface PaneTabState {
  readonly tabs: readonly any[];
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

export interface PaneState extends PaneContextState, PaneTabState, PaneViewerState, PaneMutationState {
  readonly paneId: 'left' | 'right';
  readonly cursorEntry: FileInfo | null;
  readonly selectedPaths: ReadonlySet<string>;
  readonly selectedEntries: readonly FileInfo[];
  readonly pinnableLocation: string;
  readonly pinnableSurfaceId: string;
  readonly pinnableTitle: string;
}

export interface PanePeerState {
  readonly path: string;
  readonly entries: readonly FileInfo[];
  readonly cursorEntry: FileInfo | null;
  readonly selectedPaths: ReadonlySet<string>;
  readonly sourceCapabilities: SourceCapabilities;
  readonly isDirectoryTab: boolean;
  readonly isInArchive: boolean;
}
