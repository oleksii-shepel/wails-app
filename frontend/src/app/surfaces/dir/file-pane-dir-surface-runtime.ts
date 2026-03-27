import type { FileInfo } from '../../shared/protocol';
import type { TreeNode } from '../../components/file-tree.component';
import type { DirSurfaceRuntime } from './dir-surface-runtime';

type FileViewContainerLike = {
  getSelectableEntries(parentEntryPath: string, includeParent?: boolean): FileInfo[];
  getVisibleTreeNodes(): TreeNode[];
  focusTreeNode(path: string): void;
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
    private readonly getFileViewContainer: () => FileViewContainerLike | undefined,
    private readonly parentEntryPath: string,
  ) {}

  getSelectableEntries(): FileInfo[] {
    return this.getFileViewContainer()?.getSelectableEntries(this.parentEntryPath) ?? [];
  }

  getCursorEntries(): FileInfo[] {
    return this.getFileViewContainer()?.getSelectableEntries(this.parentEntryPath, true) ?? [];
  }

  scrollToCursorPath(path: string): void {
    if (!path) return;
    this.getFileViewContainer()?.scrollToCursorPath(path);
  }
}
