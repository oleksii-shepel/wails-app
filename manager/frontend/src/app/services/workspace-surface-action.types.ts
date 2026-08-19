import type { WorkspacePresentation, WorkspaceSurface } from './workspace-surface.model';

export interface WorkspaceSurfaceActionEvent {
  readonly workspace: WorkspacePresentation;
  readonly surface: WorkspaceSurface;
  readonly action: string;
  readonly payload?: unknown;
}

export interface WorkspaceSurfaceActionContext {
  readonly event: WorkspaceSurfaceActionEvent;
  readonly services: WorkspaceSurfaceActionHostServices;
}

export interface WorkspaceSurfaceActionHostServices {
  readonly refreshSurface: (
    surface: WorkspaceSurface,
  ) => void | Promise<void>;

  readonly patchSurface: (
    surfaceId: string,
    patch: Readonly<Record<string, unknown>>,
  ) => void;

  readonly replaceSurfaceState: (
    surfaceId: string,
    state: Readonly<Record<string, unknown>>,
  ) => void;

  readonly findSurfaceByKind: (
    surfaceKind: string,
  ) => WorkspaceSurface | null;

  readonly getActivePane: () => unknown;
  readonly closeToolWorkspace: () => void;
  readonly markForCheck: () => void;
}

export interface WorkspaceSurfaceActionContribution {
  readonly id: string;
  readonly surfaceId?: string;
  readonly action: string;

  /**
   * Score: <= 0 means not applicable. Higher wins.
   * Contributions with an exact surfaceId naturally have precedence through score().
   */
  readonly score?: (event: WorkspaceSurfaceActionEvent) => number;

  execute(
    context: WorkspaceSurfaceActionContext,
  ): void | Promise<void>;
}