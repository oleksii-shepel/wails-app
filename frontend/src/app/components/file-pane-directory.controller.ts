import { EventsOff, EventsOn } from 'wailsjs/runtime';
import type { FileInfo } from '../shared/protocol';
import { normalizePath, pathIdentityKey, samePath } from './address-bar.utils';
import type { FileViewDirectorySource } from '../services/directory-source.service';
import type { DirListStreamController, DirStreamBatchEvent, DirStreamDoneEvent, DirStreamErrorEvent, DirStreamMetaEvent } from '../surfaces/dir/dir-list-stream.controller';
import type { WorkspaceService } from '../services/workspace.service';
import type { PathHistoryService } from '../services/path-history.service';

declare const window: any;

export interface FilePaneDirectoryHost {
  get paneId(): 'left' | 'right';
  get currentPath(): string;
  set currentPath(value: string);
  get fsParentDir(): string;
  set fsParentDir(value: string);
  get currentTabId(): string;
  get showHidden(): boolean;
  get entries(): FileInfo[];
  set entries(value: FileInfo[]);
  get filteredEntries(): FileInfo[];
  get selectedFiles(): Set<string>;
  set selectedFiles(value: Set<string>);
  get selectionCursorPath(): string | null;
  set selectionCursorPath(value: string | null);
  get selectionAnchorPath(): string | null;
  set selectionAnchorPath(value: string | null);
  get loading(): boolean;
  set loading(value: boolean);
  get error(): string | null;
  set error(value: string | null);
  workspaceService: WorkspaceService;
  pathHistoryService: PathHistoryService;
  directorySource: FileViewDirectorySource;
  dirStream: DirListStreamController;
  isInArchive(): boolean;
  exitArchive(): void;
  clearArchiveState(): void;
  withParentEntry(raw: FileInfo[], inArchive: boolean, fsPath: string): FileInfo[];
  applyFilter(): void;
  setShowHiddenLoaded(showHidden: boolean): void;
  emitPathChange(path: string): void;
  runInAngular(update: () => void): void;
  onRefresh(): void;
  hasSavedDirTabState(): boolean;
  restoreActiveDirTabState(): void;
  persistDirTabState(): void;
  scrollToCursorPath(path: string): void;
  focusTreeNode(path: string): void;
  shouldSuppressFsRefresh(): boolean;
}

export class FilePaneDirectoryController {
  private fsChangeDebounceTimer: ReturnType<typeof setTimeout> | null = null;
  private activeDirStreamPreserveUI = false;
  private pendingFocusPath: string | null = null;

  bind(host: FilePaneDirectoryHost): void {
    EventsOn('fs:directoryChanged', (event: { paneId?: string; path?: string } | null) => {
      host.runInAngular(() => this.handleFsDirectoryChanged(host, event));
    });

    host.dirStream.bind(host.paneId, {
      onMeta: (event, isFirst) => host.runInAngular(() => this.handleDirStreamMeta(host, event, isFirst)),
      onBatch: (event, baseEntries) => host.runInAngular(() => this.handleDirStreamBatch(host, event, baseEntries)),
      onDone: (event, baseEntries) => host.runInAngular(() => this.handleDirStreamDone(host, event, baseEntries)),
      onError: (event) => host.runInAngular(() => this.handleDirStreamError(host, event)),
    });
  }

  async unbind(host: FilePaneDirectoryHost): Promise<void> {
    if (this.fsChangeDebounceTimer) {
      clearTimeout(this.fsChangeDebounceTimer);
      this.fsChangeDebounceTimer = null;
    }
    await host.dirStream.unbind();
    EventsOff('fs:directoryChanged');
  }

  async loadDirectory(host: FilePaneDirectoryHost, path: string, opts?: { preserveUI?: boolean; focusPath?: string }): Promise<void> {
    await host.dirStream.cancelActive();
    host.exitArchive();
    host.clearArchiveState();
    this.activeDirStreamPreserveUI = !!opts?.preserveUI;
    this.pendingFocusPath = opts?.focusPath ?? null;
    host.loading = true;
    host.error = null;

    if (!this.activeDirStreamPreserveUI) {
      host.selectedFiles = new Set<string>();
      host.selectionCursorPath = null;
      host.selectionAnchorPath = null;
      host.entries = host.withParentEntry([], false, path);
      host.applyFilter();
    }

    try {
      if (!host.directorySource.supportsStreaming(path)) {
        const listing = await host.directorySource.listDirectory(path, host.showHidden);
        this.handleDirectDirectoryLoad(host, listing.path, listing.entries);
        return;
      }

      const streamId = await host.dirStream.start(path, host.showHidden);
      await host.dirStream.waitFor(streamId);
    } catch (error: any) {
      if (error?.message === 'Directory stream cancelled') {
        return;
      }
      host.error = error.message || 'Failed to load directory';
    } finally {
      this.activeDirStreamPreserveUI = false;
      this.pendingFocusPath = null;
      host.loading = false;
    }
  }

  private handleDirectDirectoryLoad(host: FilePaneDirectoryHost, path: string, entries: FileInfo[]): void {
    host.currentPath = path;
    host.fsParentDir = path;
    host.setShowHiddenLoaded(!!host.showHidden);
    host.workspaceService.updateTabPath(host.paneId, host.currentTabId, path);
    host.pathHistoryService.addPath(path, host.paneId);
    host.emitPathChange(path);

    const nextEntries = host.withParentEntry(entries, false, path);
    host.entries = nextEntries;

    if (host.selectedFiles.size > 0) {
      const present = new Set(nextEntries.map(entry => entry.path));
      const nextSelected = new Set(host.selectedFiles);
      for (const candidate of Array.from(nextSelected)) {
        if (!present.has(candidate)) nextSelected.delete(candidate);
      }
      host.selectedFiles = nextSelected;
    }

    host.applyFilter();

    if (this.pendingFocusPath) {
      const focusNorm = normalizePath(this.pendingFocusPath);
      const target = host.filteredEntries.find(entry => pathIdentityKey(entry.path) === pathIdentityKey(focusNorm));
      if (target) {
        host.selectionCursorPath = target.path;
        host.selectionAnchorPath = target.path;
        host.selectedFiles = new Set([target.path]);
        setTimeout(() => host.scrollToCursorPath(target.path), 0);
      }
      this.pendingFocusPath = null;
    } else if (host.hasSavedDirTabState()) {
      host.restoreActiveDirTabState();
    } else if (!host.selectionCursorPath) {
      const firstReal = host.filteredEntries.find(entry => entry.path !== '..');
      if (firstReal) {
        host.selectionCursorPath = firstReal.path;
        host.selectionAnchorPath = firstReal.path;
      }
    }

    host.persistDirTabState();
    setTimeout(() => host.focusTreeNode(path), 0);
    void window.go?.app?.App?.SetPaneDirectory?.(host.paneId, path);
  }

  private handleFsDirectoryChanged(host: FilePaneDirectoryHost, event: { paneId?: string; path?: string } | null): void {
    if (!event || event.paneId !== host.paneId) return;
    if (host.isInArchive()) return;
    if (host.shouldSuppressFsRefresh()) return;

    const current = (host.currentPath || '').replace(/\\/g, '/');
    const changed = (event.path || '').replace(/\\/g, '/');
    if (!current || !changed || !samePath(current, changed)) return;

    if (this.fsChangeDebounceTimer) {
      clearTimeout(this.fsChangeDebounceTimer);
    }
    this.fsChangeDebounceTimer = setTimeout(() => {
      this.fsChangeDebounceTimer = null;
      host.onRefresh();
    }, 150);
  }

  private handleDirStreamMeta(host: FilePaneDirectoryHost, event: DirStreamMetaEvent, isFirstMeta: boolean): void {
    host.currentPath = event.path;
    host.fsParentDir = event.path;
    host.setShowHiddenLoaded(!!event.showHidden);
    host.workspaceService.updateTabPath(host.paneId, host.currentTabId, event.path);
    host.pathHistoryService.addPath(event.path, host.paneId);
    if (isFirstMeta) {
      host.emitPathChange(event.path);
    }
    if (!this.activeDirStreamPreserveUI) {
      host.entries = host.withParentEntry([], false, event.path);
      host.applyFilter();
    }

    void window.go?.app?.App?.SetPaneDirectory?.(host.paneId, event.path);
  }

  private handleDirStreamBatch(host: FilePaneDirectoryHost, event: DirStreamBatchEvent, baseEntries: FileInfo[]): void {
    host.entries = host.withParentEntry(baseEntries, false, host.currentPath || event.path);
    host.applyFilter();
  }

  private handleDirStreamDone(host: FilePaneDirectoryHost, event: DirStreamDoneEvent, baseEntries: FileInfo[]): void {
    host.entries = host.withParentEntry(baseEntries, false, event.path || host.currentPath);

    if (host.selectedFiles.size > 0) {
      const present = new Set(host.entries.map(entry => entry.path));
      const nextSelected = new Set(host.selectedFiles);
      for (const path of Array.from(nextSelected)) {
        if (!present.has(path)) nextSelected.delete(path);
      }
      host.selectedFiles = nextSelected;
    }

    host.applyFilter();

    if (this.pendingFocusPath) {
      const focusNorm = normalizePath(this.pendingFocusPath);
      const target = host.filteredEntries.find(entry => pathIdentityKey(entry.path) === pathIdentityKey(focusNorm));
      if (target) {
        host.selectionCursorPath = target.path;
        host.selectionAnchorPath = target.path;
        host.selectedFiles = new Set([target.path]);
        setTimeout(() => host.scrollToCursorPath(target.path), 0);
      }
      this.pendingFocusPath = null;
    } else if (host.hasSavedDirTabState()) {
      host.restoreActiveDirTabState();
    } else if (!host.selectionCursorPath) {
      const firstReal = host.filteredEntries.find(entry => entry.path !== '..');
      if (firstReal) {
        host.selectionCursorPath = firstReal.path;
        host.selectionAnchorPath = firstReal.path;
      }
    }

    host.persistDirTabState();
    setTimeout(() => host.focusTreeNode(event.path), 0);
  }

  private handleDirStreamError(host: FilePaneDirectoryHost, event: DirStreamErrorEvent): void {
    host.error = event.error || 'Failed to load directory';
  }
}