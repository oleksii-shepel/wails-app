export type WorkspaceKind = 'files' | 'tool';
export type WorkspaceLifetime = 'persistent' | 'transient';

export type WorkspaceSurfaceRole =
  | 'primary'
  | 'secondary'
  | 'support'
  | 'result';

export interface WorkspaceSurfaceAction {
  readonly id: string;
  readonly title: string;
  readonly intent?: 'default' | 'primary' | 'danger';
  readonly requiresSelection?: boolean;
}

export interface WorkspaceSurface {
  readonly id: string;
  readonly surfaceId: string;
  readonly title: string;
  readonly role?: WorkspaceSurfaceRole;
  readonly state?: Readonly<Record<string, unknown>>;
  readonly actions?: readonly WorkspaceSurfaceAction[];
}

export interface WorkspaceSurfaceLeaf {
  readonly kind: 'surface';
  readonly surface: WorkspaceSurface;
}

export interface WorkspaceSplitLayout {
  readonly kind: 'split';
  readonly direction: 'horizontal' | 'vertical';
  readonly ratio?: number;
  readonly first: WorkspaceLayout;
  readonly second: WorkspaceLayout;
}

export interface WorkspaceStackLayout {
  readonly kind: 'stack';
  readonly activeSurfaceId: string;
  readonly surfaces: readonly WorkspaceSurface[];
}

export type WorkspaceLayout =
  | WorkspaceSurfaceLeaf
  | WorkspaceSplitLayout
  | WorkspaceStackLayout;

export interface WorkspacePresentation {
  readonly workspaceId: string;
  readonly title: string;
  readonly kind: WorkspaceKind;
  readonly lifetime: WorkspaceLifetime;
  readonly layout: WorkspaceLayout;
}

export function workspaceSurface(
  id: string,
  surfaceId: string,
  title: string,
  role: WorkspaceSurfaceRole = 'primary',
  state?: Readonly<Record<string, unknown>>,
  actions?: readonly WorkspaceSurfaceAction[],
): WorkspaceSurface {
  return {
    id,
    surfaceId,
    title,
    role,
    ...(state ? { state } : {}),
    ...(actions?.length ? { actions } : {}),
  };
}

export function surfaceLayout(
  surface: WorkspaceSurface,
): WorkspaceSurfaceLeaf {
  return { kind: 'surface', surface };
}

export function splitLayout(
  direction: 'horizontal' | 'vertical',
  first: WorkspaceLayout,
  second: WorkspaceLayout,
  ratio = 50,
): WorkspaceSplitLayout {
  return {
    kind: 'split',
    direction,
    ratio: Math.max(5, Math.min(95, ratio)),
    first,
    second,
  };
}

export function stackLayout(
  surfaces: readonly WorkspaceSurface[],
  activeSurfaceId = surfaces[0]?.id ?? '',
): WorkspaceStackLayout {
  return {
    kind: 'stack',
    activeSurfaceId,
    surfaces,
  };
}

export function fileWorkspacePresentation(
  workspaceId: string,
  splitRatio = 50,
): WorkspacePresentation {
  return {
    workspaceId,
    title: 'Files',
    kind: 'files',
    lifetime: 'persistent',
    layout: splitLayout(
      'horizontal',
      surfaceLayout(
        workspaceSurface(
          `${workspaceId}:left`,
          'directory-pane',
          'Left files',
          'primary',
          { paneId: 'left' },
        ),
      ),
      surfaceLayout(
        workspaceSurface(
          `${workspaceId}:right`,
          'directory-pane',
          'Right files',
          'secondary',
          { paneId: 'right' },
        ),
      ),
      splitRatio,
    ),
  };
}