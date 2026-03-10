import type { FileInfo } from '../../shared/protocol';
import type { ViewLayout } from '../../components/file-view-container.component';
import type { DirSurfaceRuntime } from './dir-surface-runtime';

type VisibleTreeNode = {
  name: string;
  path: string;
  type: any;
  size?: number;
  modified?: string;
  created?: string;
  level?: number;
};

type FileTreeLike = {
  getVisibleNodes(): VisibleTreeNode[];
};

type FileViewContainerLike = {
  treeComponent?: FileTreeLike;
  sortedEntries?: FileInfo[];
  scrollToCursorPath(path: string): void;
};

/**
 * DirSurfaceRuntime adapter for FilePane-driven directory surfaces.
 *
 * Important: this is the only place that should branch on the view mode
 * (tree vs list/grid/details) when computing "selectable entries".
 */
export class FilePaneDirSurfaceRuntime implements DirSurfaceRuntime {
  constructor(
    private readonly getCurrentView: () => ViewLayout,
    private readonly getFileViewContainer: () => FileViewContainerLike | undefined,
    private readonly getFilteredEntries: () => FileInfo[],
    private readonly parentEntryPath: string,
  ) {}

  getSelectableEntries(): FileInfo[] {
    if (this.getCurrentView() === 'tree') {
      const tree = this.getFileViewContainer()?.treeComponent;
      if (tree) {
        return tree
          .getVisibleNodes()
          .filter((node) => node.path !== this.parentEntryPath && (node.level ?? 0) >= 0)
          .map((node) => ({
            name: node.name,
            path: node.path,
            type: node.type,
            size: node.size,
            modified: node.modified,
            created: node.created,
          }) as unknown as FileInfo);
      }
    }

    // Use sortedEntries from the view container so index matches visual order.
    const container = this.getFileViewContainer();
    const source = (container?.sortedEntries?.length ?? 0) > 0 ? (container?.sortedEntries ?? []) : this.getFilteredEntries();
    return source.filter((e) => e.path !== this.parentEntryPath);
  }

  scrollToCursorPath(path: string): void {
    if (!path) return;
    this.getFileViewContainer()?.scrollToCursorPath(path);
  }
}

