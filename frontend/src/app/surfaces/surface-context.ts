import type { TabInfo, ViewerFileListItem } from '../shared/protocol-enhanced';
import type { ViewerArchiveContext } from '../viewers/viewer-archive-context';
import type { DirSurfaceVm } from './dir/dir-surface.vm';
import type { SizeManagerSurfaceVm } from './size-manager/size-manager-surface.vm';

/**
 * Common contract understood by the pane/tab infrastructure.
 *
 * Surface-specific data intentionally lives outside this base shape. A plugin
 * surface can therefore receive a useful context without extending a central
 * union every time a new surface is introduced.
 */
export interface SurfaceContext {
  paneId: 'left' | 'right';
  tab: TabInfo;
  /** Stable surface contribution id, e.g. "dir", "viewer", "console". */
  surfaceId: string;
  /** Built-ins may keep a legacy discriminator while callers migrate. */
  kind?: string;
  [key: string]: unknown;
}

/** Strongly typed extension helper for built-in and plugin-owned surfaces. */
export type SurfaceContextOf<
  TSurfaceId extends string,
  TData extends object = Record<never, never>,
> = SurfaceContext & { surfaceId: TSurfaceId } & TData;

export interface DirSurfaceContext extends SurfaceContext {
  surfaceId: 'dir';
  kind: 'dir';
  dirVm: DirSurfaceVm;
}

export interface ViewerSurfaceContext extends SurfaceContext {
  surfaceId: 'viewer';
  kind: 'viewer';
  viewer: {
    targetPath: string;
    fileList: ViewerFileListItem[];
    archiveContext: ViewerArchiveContext | null;
    viewerId: string | null;
  };
}

export interface SizeManagerSurfaceContext extends SurfaceContext {
  surfaceId: 'size-manager';
  kind: 'size-manager';
  sizeVm: SizeManagerSurfaceVm;
}

export function isSurfaceContext<TSurfaceId extends string>(
  ctx: SurfaceContext | null | undefined,
  surfaceId: TSurfaceId,
): ctx is SurfaceContext & { surfaceId: TSurfaceId } {
  return ctx?.surfaceId === surfaceId;
}
