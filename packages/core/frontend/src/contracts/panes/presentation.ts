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

export interface PanePresentationPort {
  panePresentation(): PanePresentation;
  applyMirroredPresentation(
    presentation: PanePresentation,
    options?: unknown,
  ): void | Promise<void>;
}

export interface PaneViewportPort {
  viewportSnapshot(): PaneViewportSnapshot;
  syncViewportToPeer(snapshot: PaneViewportSnapshot): void;
}
