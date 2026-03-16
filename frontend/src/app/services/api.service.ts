import { Injectable } from '@angular/core';
import { 
  BatchItem, 
  BatchResult, 
  BatchDeleteRequest,
  BatchExtractRequest,
  BatchExtractResult,
  BatchSelectiveExtractRequest,
  BatchSelectiveExtractResult,
  ConflictResolution,
  DirectoryListing,
  DrivesList,
  FileContent,
  FileInfo,
  OperationResult,
  SearchResult,
  WorkspaceConfig,
  UserPreferences,
  WorkspaceList,
  ArchiveListing,
  DiskUsageEntry,
  DiskUsageProgress,
  DiskUsageResult,
  OperationProgress,
  FileType,
  DriveInfo,
} from '../shared/protocol-enhanced';
import { SpecialFolder } from '../shared/protocol';
import { EventsEmit } from 'wailsjs/runtime/runtime';

/** Mirror of Go's ElevatedSessionStatus struct. */
export interface ElevatedSessionStatus {
  /** True when the elevated helper process is connected and healthy. */
  active: boolean;
  /** OS PID of the elevated helper process, or 0 when inactive. */
  pid: number;
  /** Unix timestamp (seconds) when the session was started, or 0. */
  startedAt: number;
  /** TCP port used for the IPC channel. */
  port: number;
  /** Number of operations dispatched through this session. */
  opCount: number;
  /** Non-empty when the last operation produced an error. */
  error: string;
}

// Extend the Window interface to include Wails runtime objects
declare global {
  interface Window {
    go: {
      main: {
        App: {
          // System
          Ping(): Promise<number>;
          GetOSInfo(): Promise<any>;
          GetDrives(): Promise<DrivesList>;
          GetSpecialFolders(): Promise<{ folders: SpecialFolder[] }>;
          
          // File System
          ListDir(path: string, showHidden: boolean): Promise<DirectoryListing>;
          ListDirFiltered(path: string, showHidden: boolean, filter: any): Promise<DirectoryListing>;
          StartListDirStream(paneId: string, path: string, showHidden: boolean): Promise<string>;
          CancelListDirStream(streamId: string): Promise<void>;
          FileInfo(path: string): Promise<FileInfo>;
          OpenFile(path: string): Promise<void>;
          ResolveLink(path: string): Promise<{ target: string; isDir: boolean }>;
          ReadFileContent(path: string, encoding: string): Promise<FileContent>;
          WriteFileContent(path: string, content: string, encoding: string): Promise<OperationResult>;
          CreateDir(path: string, recursive: boolean): Promise<OperationResult>;
          Search(path: string, pattern: string, recursive: boolean): Promise<SearchResult>;
          
          // Batch Operations
          BatchCopy(request: { items: BatchItem[]; recursive: boolean }): Promise<BatchResult>;
          BatchMove(request: { items: BatchItem[]; recursive: boolean }): Promise<BatchResult>;
          BatchDelete(request: BatchDeleteRequest): Promise<BatchResult>;
          BatchExtract(request: BatchExtractRequest): Promise<BatchExtractResult>;
          BatchSelectiveExtract(request: BatchSelectiveExtractRequest): Promise<BatchSelectiveExtractResult>;
          
          // Archive Operations
          ArchiveBrowse(fsPath: string, nestedChain: string[], innerPath: string): Promise<ArchiveListing>;
          ArchiveRead(fsPath: string, nestedChain: string[], entryPath: string, encoding: string): Promise<FileContent>;
          
          // Navigation
          NavNavigate(paneId: string, path: string): Promise<DirectoryListing>;
          NavNavigateShowHidden(paneId: string, path: string, showHidden: boolean): Promise<DirectoryListing>;
          NavBack(paneId: string): Promise<DirectoryListing>;
          NavForward(paneId: string): Promise<DirectoryListing>;
          NavGoUp(paneId: string): Promise<DirectoryListing>;
          
          // Workspace
          GetWorkspaces(): Promise<WorkspaceList>;
          SaveWorkspace(workspace: WorkspaceConfig): Promise<void>;
          LoadWorkspace(id: string): Promise<WorkspaceConfig>;
          DeleteWorkspace(id: string): Promise<void>;
          GetPreferences(): Promise<UserPreferences>;
          SavePreferences(prefs: UserPreferences): Promise<void>;
          
          // Operation Progress
          GetActiveOperations(): Promise<OperationProgress[]>;
          CancelOperation(id: string): Promise<boolean>;
          DismissOperation(id: string): Promise<void>;
          ResolveConflict(conflictId: string, resolution: ConflictResolution, applyToAll: boolean): Promise<boolean>;
          
          // Path Utilities
          SplitPath(path: string): Promise<Record<string, string>>;
          JoinPaths(parts: string[]): Promise<string>;
          GetParentPath(path: string): Promise<string>;
          ConfirmElevationRequest(requestID: string, approve: boolean): Promise<void>;
          DirSizeRecursive(path: string): Promise<number>;
          AnalyzeDiskUsage(path: string): Promise<any>;
          StartDiskUsageStream(path: string): Promise<string>;
          CancelDiskUsageStream(scanId: string): Promise<void>;

          // Elevation – persistent JWT-secured helper service
          IsProcessElevated(): Promise<boolean>;
          StartElevatedSession(): Promise<ElevatedSessionStatus>;
          StopElevatedSession(): Promise<void>;
          GetElevatedSessionStatus(): Promise<ElevatedSessionStatus>;
        };
      };
    };
    runtime: {
      EventsOn(eventName: string, callback: (...args: any[]) => void): void;
      EventsOff(eventName: string, ...additionalEventNames: string[]): void;
      EventsEmit(eventName: string, ...data: any[]): void;
    };
  }
}

@Injectable({
  providedIn: 'root'
})
export class ApiService {
    /**
     * Compatibility: Returns the list of drives (Windows only).
     * Use getDrives() internally.
     */
    async listDrives(): Promise<DriveInfo[]> {
      const response = await this.getDrives();
      return response.drives || [];
    }

    /**
     * Progress-aware file copy (single file).
     * Uses batchCopy under the hood.
     */
    async copyFileWithProgress(source: string, destination: string, recursive: boolean = true): Promise<any> {
      const items = [{ source, destination }];
      return await this.batchCopy(items, recursive);
    }

    /**
     * Progress-aware file move (single file).
     * Uses batchMove under the hood.
     */
    async moveFileWithProgress(source: string, destination: string, recursive: boolean = true): Promise<any> {
      const items = [{ source, destination }];
      return await this.batchMove(items, recursive);
    }

    /**
     * Progress-aware file delete (single file).
     * Uses batchDelete under the hood.
     */
    async deleteFile(path: string, recursive: boolean = true, useTrash: boolean = true): Promise<any> {
      return await this.batchDelete([path], recursive, useTrash);
    }
  private ws: WebSocket | null = null;

  // ============================================================================
  // System
  // ============================================================================

  async checkHealth(): Promise<boolean> {
    try {
      const result = await window.go.main.App.Ping();
      return result > 0;
    } catch (error) {
      console.error('Health check failed:', error);
      return false;
    }
  }

  async getOSInfo(): Promise<any> {
    return await window.go.main.App.GetOSInfo();
  }

  async getDrives(): Promise<DrivesList> {
    return await window.go.main.App.GetDrives();
  }

  async getSpecialFolders(): Promise<SpecialFolder[]> {
    const response = await window.go.main.App.GetSpecialFolders();
    return response.folders || [];
  }

  // ============================================================================
  // File System Operations
  // ============================================================================

  async listDirectory(path: string, showHidden: boolean = false): Promise<DirectoryListing> {
    return await window.go.main.App.ListDir(path, showHidden);
  }

  async startListDirectoryStream(paneId: string, path: string, showHidden: boolean = false): Promise<string> {
    return await window.go.main.App.StartListDirStream(paneId, path, showHidden);
  }

  async cancelListDirectoryStream(streamId: string): Promise<void> {
    if (!streamId) return;
    await window.go.main.App.CancelListDirStream(streamId);
  }

  async listDirectoryFiltered(path: string, showHidden: boolean, filter: any): Promise<DirectoryListing> {
    return await window.go.main.App.ListDirFiltered(path, showHidden, filter);
  }

  async getFileInfo(path: string): Promise<FileInfo> {
    return await window.go.main.App.FileInfo(path);
  }

  async openFile(path: string): Promise<void> {
    return await window.go.main.App.OpenFile(path);
  }

  async resolveLink(path: string): Promise<{ target: string; isDir: boolean }> {
    return await window.go.main.App.ResolveLink(path);
  }

  async readFile(path: string, encoding: string = 'utf8'): Promise<FileContent> {
    return await window.go.main.App.ReadFileContent(path, encoding);
  }

  async writeFile(path: string, content: string, encoding: string = 'utf8'): Promise<OperationResult> {
    return await window.go.main.App.WriteFileContent(path, content, encoding);
  }

  async createDirectory(path: string, recursive: boolean = false): Promise<OperationResult> {
    return await window.go.main.App.CreateDir(path, recursive);
  }

  async search(path: string, pattern: string, recursive: boolean = true): Promise<SearchResult> {
    return await window.go.main.App.Search(path, pattern, recursive);
  }

  // ============================================================================
  // Batch Operations
  // ============================================================================

  async batchCopy(items: BatchItem[], recursive: boolean = true): Promise<BatchResult> {
    return await window.go.main.App.BatchCopy({
      items,
      recursive
    });
  }

  async batchMove(items: BatchItem[], recursive: boolean = true): Promise<BatchResult> {
    return await window.go.main.App.BatchMove({
      items,
      recursive
    });
  }

  async batchDelete(paths: string[], recursive: boolean = true, useTrash: boolean = true): Promise<BatchResult> {
    const request: BatchDeleteRequest = {
      paths,
      recursive,
      useTrash
    };
    return await window.go.main.App.BatchDelete(request);
  }

  async batchExtract(request: BatchExtractRequest): Promise<BatchExtractResult> {
    return await window.go.main.App.BatchExtract(request);
  }

  async batchSelectiveExtract(request: BatchSelectiveExtractRequest): Promise<BatchSelectiveExtractResult> {
    return await window.go.main.App.BatchSelectiveExtract(request);
  }

  // ============================================================================
  // Archive Operations
  // ============================================================================

  async archiveBrowse(fsPath: string, nestedChain: string[] = [], innerPath: string = ''): Promise<ArchiveListing> {
    return await window.go.main.App.ArchiveBrowse(fsPath, nestedChain, innerPath);
  }

  async archiveRead(fsPath: string, nestedChain: string[] = [], entryPath: string, encoding: string = 'utf8'): Promise<FileContent> {
    return await window.go.main.App.ArchiveRead(fsPath, nestedChain, entryPath, encoding);
  }

  // ============================================================================
  // Navigation
  // ============================================================================

  async navigate(paneId: string, path: string, showHidden: boolean = false): Promise<DirectoryListing> {
    if (showHidden) {
      return await window.go.main.App.NavNavigateShowHidden(paneId, path, showHidden);
    }
    return await window.go.main.App.NavNavigate(paneId, path);
  }

  async navigateBack(paneId: string): Promise<DirectoryListing | null> {
    try {
      return await window.go.main.App.NavBack(paneId);
    } catch (error) {
      console.warn('Cannot navigate back:', error);
      return null;
    }
  }

  async navigateForward(paneId: string): Promise<DirectoryListing | null> {
    try {
      return await window.go.main.App.NavForward(paneId);
    } catch (error) {
      console.warn('Cannot navigate forward:', error);
      return null;
    }
  }

  async navigateUp(paneId: string): Promise<DirectoryListing | null> {
    try {
      return await window.go.main.App.NavGoUp(paneId);
    } catch (error) {
      console.warn('Cannot navigate up:', error);
      return null;
    }
  }

  // ============================================================================
  // Workspace & Preferences
  // ============================================================================

  async getWorkspaces(): Promise<WorkspaceList> {
    return await window.go.main.App.GetWorkspaces();
  }

  async saveWorkspace(workspace: WorkspaceConfig): Promise<void> {
    return await window.go.main.App.SaveWorkspace(workspace);
  }

  async loadWorkspace(id: string): Promise<WorkspaceConfig> {
    return await window.go.main.App.LoadWorkspace(id);
  }

  async deleteWorkspace(id: string): Promise<void> {
    return await window.go.main.App.DeleteWorkspace(id);
  }

  async getPreferences(): Promise<UserPreferences> {
    return await window.go.main.App.GetPreferences();
  }

  async savePreferences(prefs: UserPreferences): Promise<void> {
    return await window.go.main.App.SavePreferences(prefs);
  }

  // ============================================================================
  // Operation Progress Tracking
  // ============================================================================

  async getActiveOperations(): Promise<OperationProgress[]> {
    return await window.go.main.App.GetActiveOperations();
  }

  async cancelOperation(id: string): Promise<boolean> {
    return await window.go.main.App.CancelOperation(id);
  }

  async dismissOperation(id: string): Promise<void> {
    return await window.go.main.App.DismissOperation(id);
  }

  async resolveConflict(conflictId: string, resolution: ConflictResolution, applyToAll: boolean): Promise<boolean> {
    return await window.go.main.App.ResolveConflict(conflictId, resolution, applyToAll);
  }

  // ============================================================================
  // WebSocket Connection (Optional)
  // ============================================================================

  connectWebSocket(): void {
    if (this.ws?.readyState === WebSocket.OPEN) return;
    
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    this.ws = new WebSocket(wsUrl);
    
    this.ws.onopen = () => {
      console.log('WebSocket connected');
    };
    
    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    
    this.ws.onclose = () => {
      console.log('WebSocket disconnected');
    };
  }

  disconnectWebSocket(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  // ============================================================================
  // Path Utilities (via Go)
  // ============================================================================

  async splitPath(path: string): Promise<{ dir: string; base: string }> {
    const result = await window.go.main.App.SplitPath(path);
    return {
      dir: result['dir'] || '',
      base: result['base'] || ''
    };
  }

  async joinPaths(parts: string[]): Promise<string> {
    return await window.go.main.App.JoinPaths(parts);
  }

  async getParentPath(path: string): Promise<string> {
    return await window.go.main.App.GetParentPath(path);
  }

  async confirmElevationRequest(requestID: string, approve: boolean): Promise<void> {
    return await window.go.main.App.ConfirmElevationRequest(requestID, approve);
  }

  // ── Persistent elevated service session (JWT-secured helper process) ───────

  async isProcessElevated(): Promise<boolean> {
    return window.go.main.App.IsProcessElevated();
  }

  async startElevatedSession(): Promise<ElevatedSessionStatus> {
    return window.go.main.App.StartElevatedSession();
  }

  async stopElevatedSession(): Promise<void> {
    return window.go.main.App.StopElevatedSession();
  }

  async getElevatedSessionStatus(): Promise<ElevatedSessionStatus> {
    return window.go.main.App.GetElevatedSessionStatus();
  }

  /**
   * Set up event listeners for operation progress
   */
  setupOperationListeners(): void {
    // These events will be picked up by OperationService
    // No need to do anything here - just ensure wailsjs/runtime is imported
  }

  /**
   * Emit an event (for testing or custom events)
   */
  emitEvent(eventName: string, data: any): void {
    EventsEmit(eventName, data);
  }

  // ============================================================================
  // Disk Usage Analyzer
  // ============================================================================

  /**
   * Legacy blocking call — waits for the full scan before resolving.
   * Prefer streamDiskUsage for interactive use.
   */
  async analyzeDiskUsage(path: string): Promise<import('../shared/protocol-enhanced').DiskUsageResult> {
    return await window.go.main.App.AnalyzeDiskUsage(path);
  }

  /**
   * Streaming disk-usage scan.
   *
   * Returns an AsyncGenerator that yields DiskUsageEntry objects as each
   * direct child finishes scanning, then yields the final DiskUsageResult
   * summary as the last item.
   *
   * The generator handles its own cleanup: it cancels the Go-side scan if
   * the caller breaks out of the for-await loop early (e.g. on navigation).
   *
   * Usage in the component:
   *
   *   for await (const item of this.api.streamDiskUsage(path)) {
   *     if (isDiskUsageResult(item)) { ... } else { ... }
   *   }
   */
  async *streamDiskUsage(
    path: string,
  ): AsyncGenerator<
    DiskUsageEntry |
    DiskUsageProgress |
    DiskUsageResult
  > {
    const scanId = await window.go.main.App.StartDiskUsageStream(path);

    type Pending =
      | { kind: 'entry'; value: DiskUsageEntry }
      | { kind: 'progress'; value: DiskUsageProgress }
      | { kind: 'done'; value: DiskUsageResult }
      | { kind: 'error';  error: Error };

    const queue: Pending[] = [];
    let wake: (() => void) | null = null;
    let finished = false;

    const notify = () => { const r = wake; wake = null; r?.(); };

    const onEntry = (payload: { scanId: string; entry: any }) => {
      if (payload.scanId !== scanId) return;
      queue.push({ kind: 'entry', value: payload.entry });
      notify();
    };

    const onDone = (payload: { scanId: string; result: any }) => {
      if (payload.scanId !== scanId) return;
      queue.push({ kind: 'done', value: payload.result });
      finished = true;
      notify();
    };

    const onProgress = (payload: { scanId: string; progress: any }) => {
      if (payload.scanId !== scanId) return;
      queue.push({ kind: 'progress', value: payload.progress });
      notify();
    };

    const onError = (payload: { scanId: string; error: string }) => {
      if (payload.scanId !== scanId) return;
      queue.push({ kind: 'error', error: new Error(payload.error) });
      finished = true;
      notify();
    };

    window.runtime.EventsOn('disk-usage:entry', onEntry);
    window.runtime.EventsOn('disk-usage:progress', onProgress);
    window.runtime.EventsOn('disk-usage:done',  onDone);
    window.runtime.EventsOn('disk-usage:error', onError);

    const cleanup = () => {
      window.runtime.EventsOff('disk-usage:entry', 'disk-usage:progress', 'disk-usage:done', 'disk-usage:error');
    };

    try {
      while (true) {
        while (queue.length > 0) {
          const item = queue.shift()!;
          if (item.kind === 'error') throw item.error;
          yield item.value;
          if (item.kind === 'done') return;
        }

        if (finished) return;

        // Park until the next event wakes us.
        await new Promise<void>(res => { wake = res; });
      }
    } finally {
      // Runs on break, return, or throw — cancel any in-progress Go scan.
      cleanup();
      window.go.main.App.CancelDiskUsageStream(scanId).catch(() => {/* best-effort */});
    }
  }

  async isDirectory(path: string): Promise<boolean> {
    try {
      const info = await this.getFileInfo(path);
      return info.type === FileType.DIRECTORY;
    } catch {
      return false;
    }
  }
}