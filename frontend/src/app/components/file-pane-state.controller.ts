import type { FileInfo, FileType } from '../shared/protocol';
import type { FilterQuery } from '../shared/protocol-enhanced';
import type { ViewLayout } from './file-view-container.component';

export interface FilePaneStateHost {
  entries: FileInfo[];
  filteredEntries: FileInfo[];
  selectedFiles: Set<string>;
  selectionCursorPath: string | null;
  selectionAnchorPath: string | null;
  filterQuery: FilterQuery;
  currentView: ViewLayout;
  lastPreviewTargetEmitted: string | null;

  isInArchive(): boolean;
  getSelectableEntries(): FileInfo[];
  getOptimisticHiddenPaths(): string[];
  setOptimisticHiddenPaths(paths: string[]): void;
  filterEntries(entries: FileInfo[], query: FilterQuery): FileInfo[];
  findEntry(path: string): FileInfo | undefined;
  emitPreviewTarget(path: string): void;
  scrollToCursorPath(path: string): void;
  getGridColumns(): number;
  getApproxPageSize(): number;
  persistDirTabState(): void;
}

export class FilePaneStateController {
  constructor(
    private readonly parentEntryPath: string,
    private readonly fileTypeFile: FileType,
    private readonly fileTypeSymlink: FileType,
  ) {}

  applyFilter(host: FilePaneStateHost): void {
    const optimisticHiddenPaths = new Set(host.getOptimisticHiddenPaths());
    if (optimisticHiddenPaths.size > 0) {
      const present = new Set(host.entries.map(entry => entry.path));
      for (const path of Array.from(optimisticHiddenPaths)) {
        if (!present.has(path)) optimisticHiddenPaths.delete(path);
      }
      host.setOptimisticHiddenPaths(Array.from(optimisticHiddenPaths));
    }

    const hiddenPaths = new Set(host.getOptimisticHiddenPaths());
    const baseEntries = hiddenPaths.size > 0
      ? host.entries.filter(entry => entry.path === this.parentEntryPath || !hiddenPaths.has(entry.path))
      : host.entries;

    const query = host.filterQuery;
    if (!query?.text?.trim() && !query?.criteria?.length) {
      host.filteredEntries = baseEntries;
      return;
    }

    const parentEntry = baseEntries.filter(entry => entry.path === this.parentEntryPath);
    const filtered = host.filterEntries(
      baseEntries.filter(entry => entry.path !== this.parentEntryPath),
      query,
    );
    host.filteredEntries = [...parentEntry, ...filtered];
  }

  onSelectionChange(host: FilePaneStateHost, entries: FileInfo[]): void {
    const paths = entries.map(entry => entry.path).filter(path => path !== this.parentEntryPath);
    host.selectedFiles = new Set(paths);
    if (paths.length > 0) {
      host.selectionCursorPath = paths[paths.length - 1];
      if (paths.length === 1) {
        host.selectionAnchorPath = host.selectionCursorPath;
      } else if (!host.selectionAnchorPath) {
        host.selectionAnchorPath = host.selectionCursorPath;
      }
    }

    this.emitPreviewTargetFromState(host);
    host.persistDirTabState();
  }

  getPreviewTargetPath(host: FilePaneStateHost): string {
    if (host.isInArchive()) return '';

    const effective = this.getEffectiveSelection(host);
    const selected = Array.from(effective).filter(path => path && path !== this.parentEntryPath);
    if (selected.length !== 1) return '';

    const entry = host.findEntry(selected[0]);
    if (!entry) return '';
    if (entry.type !== this.fileTypeFile && entry.type !== this.fileTypeSymlink) return '';
    return entry.path;
  }

  emitPreviewTargetFromState(host: FilePaneStateHost): void {
    const next = this.getPreviewTargetPath(host);
    if (host.lastPreviewTargetEmitted === next) return;
    host.lastPreviewTargetEmitted = next;
    host.emitPreviewTarget(next);
  }

  onSelectAll(host: FilePaneStateHost): void {
    const selectable = host.getSelectableEntries();
    host.selectedFiles = new Set(selectable.map(entry => entry.path));
    host.selectionCursorPath = selectable.length ? selectable[selectable.length - 1].path : null;
    host.selectionAnchorPath = host.selectionCursorPath;
  }

  onDeselectAll(host: FilePaneStateHost): void {
    host.selectedFiles = new Set();
  }

  getEffectiveSelection(host: FilePaneStateHost): Set<string> {
    if (host.selectedFiles.size > 0) return host.selectedFiles;
    if (host.selectionCursorPath) return new Set([host.selectionCursorPath]);
    return new Set();
  }

  onToggleCurrentAndMoveDown(host: FilePaneStateHost): void {
    const selectable = host.getSelectableEntries();
    const index = this.getCursorIndex(host, selectable);
    if (index < 0) return;

    const path = selectable[index].path;
    const next = new Set(host.selectedFiles);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    host.selectedFiles = next;

    const nextIndex = Math.min(index + 1, selectable.length - 1);
    host.selectionCursorPath = selectable[nextIndex]?.path ?? path;
    host.selectionAnchorPath = host.selectionAnchorPath ?? path;
    this.emitPreviewTargetFromState(host);
  }

  onSelectCurrentAndMoveDown(host: FilePaneStateHost): void {
    const selectable = host.getSelectableEntries();
    const index = this.getCursorIndex(host, selectable);
    if (index < 0) return;

    const path = selectable[index].path;
    const next = new Set(host.selectedFiles);
    next.add(path);
    host.selectedFiles = next;

    const nextIndex = Math.min(index + 1, selectable.length - 1);
    host.selectionCursorPath = selectable[nextIndex]?.path ?? path;
    host.selectionAnchorPath = path;
    this.emitPreviewTargetFromState(host);
  }

  onMarkCurrentItem(host: FilePaneStateHost): void {
    if (!host.selectionCursorPath) return;

    const path = host.selectionCursorPath;
    const next = new Set(host.selectedFiles);
    if (next.has(path)) next.delete(path);
    else next.add(path);
    host.selectedFiles = next;

    const selectable = host.getSelectableEntries();
    const index = this.getCursorIndex(host, selectable);
    if (index >= 0 && index < selectable.length - 1) {
      host.selectionCursorPath = selectable[index + 1].path;
      host.scrollToCursorPath(host.selectionCursorPath);
    }

    this.emitPreviewTargetFromState(host);
  }

  onExtendSelection(host: FilePaneStateHost, delta: -1 | 1): void {
    const selectable = host.getSelectableEntries();
    const index = this.getCursorIndex(host, selectable);
    if (index < 0) return;

    const targetIndex = Math.max(0, Math.min(selectable.length - 1, index + delta));
    const anchorPath = host.selectionAnchorPath ?? selectable[index].path;
    const anchorIndex = selectable.findIndex(entry => entry.path === anchorPath);
    const start = Math.min(anchorIndex === -1 ? index : anchorIndex, targetIndex);
    const end = Math.max(anchorIndex === -1 ? index : anchorIndex, targetIndex);

    host.selectedFiles = new Set(selectable.slice(start, end + 1).map(entry => entry.path));
    host.selectionCursorPath = selectable[targetIndex].path;
    host.selectionAnchorPath = anchorPath;
    this.emitPreviewTargetFromState(host);
  }

  onMoveCursorUp(host: FilePaneStateHost): void {
    const cols = host.getGridColumns();
    const selectable = host.getSelectableEntries();
    this.moveCursorTo(host, this.getCursorIndex(host, selectable) - cols);
  }

  onMoveCursorDown(host: FilePaneStateHost): void {
    const cols = host.getGridColumns();
    const selectable = host.getSelectableEntries();
    this.moveCursorTo(host, this.getCursorIndex(host, selectable) + cols);
  }

  onMoveCursorLeft(host: FilePaneStateHost): void {
    if (host.currentView !== 'grid') return;
    const selectable = host.getSelectableEntries();
    this.moveCursorTo(host, this.getCursorIndex(host, selectable) - 1);
  }

  onMoveCursorRight(host: FilePaneStateHost): void {
    if (host.currentView !== 'grid') return;
    const selectable = host.getSelectableEntries();
    this.moveCursorTo(host, this.getCursorIndex(host, selectable) + 1);
  }

  onMoveCursorHome(host: FilePaneStateHost): void {
    this.moveCursorTo(host, 0);
  }

  onMoveCursorEnd(host: FilePaneStateHost): void {
    const selectable = host.getSelectableEntries();
    this.moveCursorTo(host, selectable.length - 1);
  }

  onMoveCursorPageUp(host: FilePaneStateHost): void {
    const pageSize = host.getApproxPageSize();
    const selectable = host.getSelectableEntries();
    this.moveCursorTo(host, this.getCursorIndex(host, selectable) - pageSize);
  }

  onMoveCursorPageDown(host: FilePaneStateHost): void {
    const pageSize = host.getApproxPageSize();
    const selectable = host.getSelectableEntries();
    this.moveCursorTo(host, this.getCursorIndex(host, selectable) + pageSize);
  }

  onSelectByMask(host: FilePaneStateHost, mask: string): void {
    const matcher = this.createMaskMatcher(mask);
    const next = new Set(host.selectedFiles);
    for (const entry of host.getSelectableEntries()) {
      if (matcher(entry.name)) next.add(entry.path);
    }
    host.selectedFiles = next;
    this.syncCursorFromSelection(host);
  }

  onUnselectByMask(host: FilePaneStateHost, mask: string): void {
    const matcher = this.createMaskMatcher(mask);
    const next = new Set(host.selectedFiles);
    for (const entry of host.getSelectableEntries()) {
      if (matcher(entry.name)) next.delete(entry.path);
    }
    host.selectedFiles = next;
    this.syncCursorFromSelection(host);
  }

  onInvertSelection(host: FilePaneStateHost): void {
    const selectable = host.getSelectableEntries();
    const next = new Set<string>();
    for (const entry of selectable) {
      if (!host.selectedFiles.has(entry.path)) {
        next.add(entry.path);
      }
    }
    host.selectedFiles = next;
    this.syncCursorFromSelection(host);
  }

  onSelectSameExtension(host: FilePaneStateHost): void {
    const current = this.getCurrentCursorEntry(host);
    if (!current) return;

    const extension = this.getExtension(current.name);
    if (!extension) return;

    const next = new Set(host.selectedFiles);
    for (const entry of host.getSelectableEntries()) {
      if (this.getExtension(entry.name) === extension) {
        next.add(entry.path);
      }
    }
    host.selectedFiles = next;
    this.syncCursorFromSelection(host);
  }

  private moveCursorTo(host: FilePaneStateHost, index: number): void {
    const selectable = host.getSelectableEntries();
    if (!selectable.length) return;
    const clamped = Math.max(0, Math.min(selectable.length - 1, index));
    host.selectionCursorPath = selectable[clamped].path;
    host.selectionAnchorPath = host.selectionCursorPath;
    host.scrollToCursorPath(host.selectionCursorPath);
    this.emitPreviewTargetFromState(host);
    host.persistDirTabState();
  }

  private syncCursorFromSelection(host: FilePaneStateHost): void {
    const selectable = host.getSelectableEntries();
    if (!selectable.length || host.selectedFiles.size === 0) {
      host.selectionCursorPath = selectable.length ? selectable[0].path : null;
      host.selectionAnchorPath = host.selectionCursorPath;
      this.emitPreviewTargetFromState(host);
      return;
    }

    const firstSelected = selectable.find(entry => host.selectedFiles.has(entry.path));
    if (firstSelected) {
      host.selectionCursorPath = firstSelected.path;
      host.selectionAnchorPath = host.selectionAnchorPath ?? firstSelected.path;
    }
    this.emitPreviewTargetFromState(host);
  }

  private getCursorIndex(host: FilePaneStateHost, entries: FileInfo[]): number {
    if (!entries.length) return -1;

    if (host.selectionCursorPath) {
      const index = entries.findIndex(entry => entry.path === host.selectionCursorPath);
      if (index !== -1) return index;
    }

    const firstSelected = entries.findIndex(entry => host.selectedFiles.has(entry.path));
    if (firstSelected !== -1) {
      host.selectionCursorPath = entries[firstSelected].path;
      return firstSelected;
    }

    host.selectionCursorPath = entries[0].path;
    return 0;
  }

  private getCurrentCursorEntry(host: FilePaneStateHost): FileInfo | null {
    const entries = host.getSelectableEntries();
    const index = this.getCursorIndex(host, entries);
    return index >= 0 ? entries[index] : null;
  }

  private createMaskMatcher(mask: string): (value: string) => boolean {
    const escaped = mask
      .replace(/[.+^${}()|[\]\\]/g, '\\$&')
      .replace(/\*/g, '.*')
      .replace(/\?/g, '.');
    const regex = new RegExp(`^${escaped}$`, 'i');
    return (value: string) => regex.test(value);
  }

  private getExtension(name: string): string {
    const index = name.lastIndexOf('.');
    if (index <= 0 || index === name.length - 1) return '';
    return name.slice(index + 1).toLowerCase();
  }
}