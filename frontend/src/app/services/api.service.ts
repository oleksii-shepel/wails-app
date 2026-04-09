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
  FileProbe,
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
import { normalizePath } from '../components/address-bar.utils';
import { EventsEmit } from 'wailsjs/runtime';
import { app, contracts, elevation } from '../../../wailsjs/go/models';

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

function castProtocol<T>(value: unknown): T {
  return value as T;
}

interface StreamTextResult {
  content: string;
  size: number;
}

export interface VideoInfo {
  size: number;
  format: string;
  playbackMode: string;
  manifestUrl?: string;
  streamUrl?: string;
  durationSeconds?: number;
  videoCodec?: string;
  audioCodec?: string;
  seekable?: boolean;
  useNativePlayer?: boolean;
}

function normalizeElevatedSessionStatus(status: elevation.SessionStatus): ElevatedSessionStatus {
  return {
    active: status.active,
    pid: status.pid ?? 0,
    startedAt: status.startedAt ?? 0,
    port: status.port ?? 0,
    opCount: status.opCount,
    error: status.error ?? '',
  };
}

@Injectable({
  providedIn: 'root'
})
export class ApiService {
    private normalizeCanonicalPath(path: string): string {
      return normalizePath(path);
    }

    private normalizeTextEncoding(encoding: string): string {
      const normalized = (encoding ?? '').trim().toLowerCase().replace(/_/g, '-');
      switch (normalized) {
        case '':
        case 'utf8':
        case 'utf-8':
          return 'utf-8';
        case 'utf16':
        case 'utf-16':
        case 'utf16le':
        case 'utf-16le':
          return 'utf-16le';
        case 'utf16be':
        case 'utf-16be':
          return 'utf-16be';
        case 'windows1252':
        case 'cp1252':
        case 'windows-1252':
          return 'windows-1252';
        case 'latin1':
        case 'latin-1':
        case 'iso8859-1':
        case 'iso-8859-1':
          return 'iso-8859-1';
        default:
          return normalized;
      }
    }

    private decodeTextBytes(bytes: ArrayBuffer, encoding: string): string {
      const decoder = new TextDecoder(this.normalizeTextEncoding(encoding));
      const decoded = decoder.decode(bytes);
      return decoded.charCodeAt(0) === 0xfeff ? decoded.slice(1) : decoded;
    }

    private async fetchStreamResponse(url: string): Promise<Response> {
      const response = await fetch(url);
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
      return response;
    }

    private normalizeCanonicalPathList(paths: string[]): string[] {
      return paths.map((path) => this.normalizeCanonicalPath(path));
    }

    private normalizeOptionalCanonicalPath(path: string | undefined): string | undefined {
      return path ? this.normalizeCanonicalPath(path) : path;
    }

    private normalizeBatchItems(items: BatchItem[]): BatchItem[] {
      return items.map((item) => ({
        ...item,
        source: this.normalizeCanonicalPath(item.source),
        destination: this.normalizeCanonicalPath(item.destination),
        exactDestination: item.exactDestination,
      }));
    }

    /**
     * Compatibility: Returns the list of drives (Windows only).
     * Use getDrives() internally.
     */
    async listDrives(): Promise<DriveInfo[]> {
      const response = await this.getDrives();
      return response.schemes || [];
    }

    /**
     * Progress-aware file copy (single file).
     * Uses batchCopy under the hood.
     */
    async copyFileWithProgress(source: string, destination: string, recursive: boolean = true): Promise<any> {
      const items = [{
        source: this.normalizeCanonicalPath(source),
        destination: this.normalizeCanonicalPath(destination),
      }];
      return await this.batchCopy(items, recursive);
    }

    /**
     * Progress-aware file move (single file).
     * Uses batchMove under the hood.
     */
    async moveFileWithProgress(source: string, destination: string, recursive: boolean = true): Promise<any> {
      const items = [{
        source: this.normalizeCanonicalPath(source),
        destination: this.normalizeCanonicalPath(destination),
      }];
      return await this.batchMove(items, recursive);
    }

    async moveFile(source: string, destination: string): Promise<OperationResult> {
      return await window.go.app.App.MoveFile(
        this.normalizeCanonicalPath(source),
        this.normalizeCanonicalPath(destination)
      );
    }

    /**
     * Progress-aware file delete (single file).
     * Uses batchDelete under the hood.
     */
    async deleteFile(path: string, recursive: boolean = true, useTrash: boolean = true): Promise<any> {
      return await this.batchDelete([this.normalizeCanonicalPath(path)], recursive, useTrash);
    }
  private ws: WebSocket | null = null;

  // ============================================================================
  // System
  // ============================================================================

  async checkHealth(): Promise<boolean> {
    try {
	      const result = await window.go.app.App.Ping();
      return result > 0;
    } catch (error) {
      console.error('Health check failed:', error);
      return false;
    }
  }

  async getOSInfo(): Promise<any> {
	    return await window.go.app.App.GetOSInfo();
  }

  async getDrives(): Promise<DrivesList> {
	    return castProtocol<DrivesList>(await window.go.app.App.GetDrives());
  }

  async getSpecialFolders(): Promise<SpecialFolder[]> {
	    const response = await window.go.app.App.GetSpecialFolders();
    return response.folders || [];
  }

  async remoteConnect(config: contracts.RemoteConfig): Promise<contracts.RemoteMount> {
	    return castProtocol<contracts.RemoteMount>(await window.go.app.App.RemoteConnect(config));
  }

  async remoteConnectOAuth(request: contracts.RemoteOAuthConnectRequest): Promise<contracts.RemoteMount> {
	    return castProtocol<contracts.RemoteMount>(await window.go.app.App.RemoteConnectOAuth(request));
  }

  async remoteDisconnect(mountPoint: string): Promise<void> {
	    await window.go.app.App.RemoteDisconnect(mountPoint);
  }

  async remoteListConnections(): Promise<contracts.RemoteMount[]> {
	    return castProtocol<contracts.RemoteMount[]>(await window.go.app.App.RemoteListConnections());
  }

  async remoteOAuthBegin(request: contracts.RemoteOAuthBeginRequest): Promise<contracts.RemoteOAuthBeginResponse> {
	    return castProtocol<contracts.RemoteOAuthBeginResponse>(await window.go.app.App.RemoteOAuthBegin(request));
  }

  async remoteOAuthStatus(sessionId: string): Promise<contracts.RemoteOAuthStatus> {
	    return castProtocol<contracts.RemoteOAuthStatus>(await window.go.app.App.RemoteOAuthStatus(sessionId));
  }

  async remoteOAuthCancel(sessionId: string): Promise<void> {
	    await window.go.app.App.RemoteOAuthCancel(sessionId);
  }

  // ============================================================================
  // File System Operations
  // ============================================================================

  async listDirectory(path: string, showHidden: boolean = false): Promise<DirectoryListing> {
	    return castProtocol<DirectoryListing>(await window.go.app.App.ListDir(this.normalizeCanonicalPath(path), showHidden));
  }

  async startListDirectoryStream(paneId: string, path: string, showHidden: boolean = false): Promise<string> {
	    return await window.go.app.App.StartListDirStream(paneId, this.normalizeCanonicalPath(path), showHidden);
  }

  async cancelListDirectoryStream(streamId: string): Promise<void> {
    if (!streamId) return;
	    await window.go.app.App.CancelListDirStream(streamId);
  }

  async listDirectoryFiltered(path: string, showHidden: boolean, filter: any): Promise<DirectoryListing> {
	    return castProtocol<DirectoryListing>(await window.go.app.App.ListDirFiltered(this.normalizeCanonicalPath(path), showHidden, filter));
  }

  async getFileInfo(path: string): Promise<FileInfo> {
	    return castProtocol<FileInfo>(await window.go.app.App.FileInfo(this.normalizeCanonicalPath(path)));
  }

  async probeFile(path: string): Promise<FileProbe> {
	    return castProtocol<FileProbe>(await window.go.app.App.ProbeFile(this.normalizeCanonicalPath(path)));
  }

  async openFile(path: string): Promise<void> {
    return await window.go.app.App.OpenFile(this.normalizeCanonicalPath(path));
  }

  async resolveLink(path: string): Promise<{ target: string; isDir: boolean }> {
    return await window.go.app.App.ResolveLink(this.normalizeCanonicalPath(path));
  }

  async readFile(path: string, encoding: string = 'utf8'): Promise<FileContent> {
    return castProtocol<FileContent>(await window.go.app.App.ReadFileContent(this.normalizeCanonicalPath(path), encoding));
  }

  async readFileRange(path: string, offset: number, length: number): Promise<FileContent> {
    return castProtocol<FileContent>(await window.go.app.App.ReadFileRange(this.normalizeCanonicalPath(path), offset, length));
  }

  async writeFile(path: string, content: string, encoding: string = 'utf8'): Promise<OperationResult> {
    return await window.go.app.App.WriteFileContent(this.normalizeCanonicalPath(path), content, encoding);
  }

  async createDirectory(path: string, recursive: boolean = false): Promise<OperationResult> {
    return await window.go.app.App.CreateDir(this.normalizeCanonicalPath(path), recursive);
  }

  async search(path: string, pattern: string, recursive: boolean = true): Promise<SearchResult> {
    return castProtocol<SearchResult>(await window.go.app.App.Search(this.normalizeCanonicalPath(path), pattern, recursive));
  }

  async buildSearchAlias(path: string, pattern: string, recursive: boolean = true): Promise<string> {
    return await window.go.app.App.BuildSearchAlias(this.normalizeCanonicalPath(path), pattern, recursive);
  }

  async dirSizeRecursive(path: string): Promise<number> {
    return await window.go.app.App.DirSizeRecursive(this.normalizeCanonicalPath(path));
  }

  // ============================================================================
  // Batch Operations
  // ============================================================================

  async batchCopy(items: BatchItem[], recursive: boolean = true): Promise<BatchResult> {
    return castProtocol<BatchResult>(await window.go.app.App.BatchCopy(contracts.BatchRequest.createFrom({
      items: this.normalizeBatchItems(items),
      recursive
    })));
  }

  async batchMove(items: BatchItem[], recursive: boolean = true): Promise<BatchResult> {
    return castProtocol<BatchResult>(await window.go.app.App.BatchMove(contracts.BatchRequest.createFrom({
      items: this.normalizeBatchItems(items),
      recursive
    })));
  }

  async batchDelete(paths: string[], recursive: boolean = true, useTrash: boolean = true): Promise<BatchResult> {
    const request: BatchDeleteRequest = {
      paths: this.normalizeCanonicalPathList(paths),
      recursive,
      useTrash
    };
    return castProtocol<BatchResult>(await window.go.app.App.BatchDelete(request));
  }

  async batchExtract(request: BatchExtractRequest): Promise<BatchExtractResult> {
    return castProtocol<BatchExtractResult>(await window.go.app.App.BatchExtract(contracts.BatchExtractRequest.createFrom({
      ...request,
      archives: this.normalizeCanonicalPathList(request.archives),
      destinationDir: this.normalizeCanonicalPath(request.destinationDir),
      createSubdirectory: request.createSubdirectory ?? false,
      expandNestedArchives: request.expandNestedArchives ?? false,
      maxNestDepth: request.maxNestDepth ?? 0,
      flattenSingleRoot: request.flattenSingleRoot ?? false,
    })));
  }

  async batchSelectiveExtract(request: BatchSelectiveExtractRequest): Promise<BatchSelectiveExtractResult> {
    return castProtocol<BatchSelectiveExtractResult>(await window.go.app.App.BatchSelectiveExtract(contracts.BatchSelectiveExtractRequest.createFrom({
      destinationDir: this.normalizeCanonicalPath(request.destinationDir),
      items: request.items.map(item => ({
        archivePath: this.normalizeCanonicalPath(item.archivePath),
        nestedChain: item.nestedChain ?? [],
        entries: item.entries ?? [],
        destinationDir: this.normalizeOptionalCanonicalPath(item.destinationDir),
        flattenPaths: item.flattenPaths ?? false,
      })),
    })));
  }

  // ============================================================================
  // Archive Operations
  // ============================================================================

  async archiveBrowse(fsPath: string, nestedChain: string[] = [], innerPath: string = ''): Promise<ArchiveListing> {
    return castProtocol<ArchiveListing>(await window.go.app.App.ArchiveBrowse(this.normalizeCanonicalPath(fsPath), nestedChain, innerPath));
  }

  async archiveRead(fsPath: string, nestedChain: string[] = [], entryPath: string, encoding: string = 'utf8'): Promise<FileContent> {
    return castProtocol<FileContent>(await window.go.app.App.ArchiveRead(this.normalizeCanonicalPath(fsPath), nestedChain, entryPath, encoding));
  }

  async probeArchiveFile(fsPath: string, nestedChain: string[] = [], entryPath: string): Promise<FileProbe> {
	    return castProtocol<FileProbe>(await window.go.app.App.ProbeArchiveFile(this.normalizeCanonicalPath(fsPath), nestedChain, entryPath));
  }

  async getStreamURL(path: string): Promise<string> {
	return await window.go.app.App.GetStreamURL(this.normalizeCanonicalPath(path));
  }

  async getArchiveStreamURL(fsPath: string, nestedChain: string[] = [], entryPath: string): Promise<string> {
	return await window.go.app.App.GetArchiveStreamURL(this.normalizeCanonicalPath(fsPath), nestedChain, entryPath);
  }

  async getVideoPreviewURL(path: string): Promise<string> {
    return await (window as any).go.app.App.GetVideoPreviewURL(this.normalizeCanonicalPath(path));
  }

  async getArchiveVideoPreviewURL(fsPath: string, nestedChain: string[] = [], entryPath: string): Promise<string> {
    return await (window as any).go.app.App.GetArchiveVideoPreviewURL(this.normalizeCanonicalPath(fsPath), nestedChain, entryPath);
  }

  async getVideoPlaybackPlan(path: string): Promise<app.VideoPlaybackPlan> {
    return app.VideoPlaybackPlan.createFrom(
      await (window as any).go.app.App.GetVideoPlaybackPlan(this.normalizeCanonicalPath(path))
    );
  }

  async getArchiveVideoPlaybackPlan(fsPath: string, nestedChain: string[] = [], entryPath: string): Promise<app.VideoPlaybackPlan> {
    return app.VideoPlaybackPlan.createFrom(
      await (window as any).go.app.App.GetArchiveVideoPlaybackPlan(this.normalizeCanonicalPath(fsPath), nestedChain, entryPath)
    );
  }

  async getVideoInfo(path: string): Promise<VideoInfo> {
    const [info, plan] = await Promise.all([
      this.getFileInfo(path),
      this.getVideoPlaybackPlan(path),
    ]);

    return {
      size: info.size ?? 0,
      format: plan.format || path.split('.').pop()?.toLowerCase() || '',
      playbackMode: plan.mode ?? '',
      manifestUrl: plan.mode === 'direct' ? plan.url : undefined,
      streamUrl: plan.mode !== 'direct' ? plan.url : undefined,
      durationSeconds: plan.durationSeconds,
      videoCodec: plan.videoCodec,
      audioCodec: plan.audioCodec,
      seekable: plan.seekable,
      useNativePlayer: plan.useNativePlayer,
    };
  }

  async getConvertedImageURL(path: string, targetFormat: string = 'jpeg'): Promise<string> {
    return await (window as any).go.app.App.GetConvertedImageURL(this.normalizeCanonicalPath(path), targetFormat);
  }

  async getImageThumbnailURL(path: string, maxWidth: number = 256, maxHeight: number = 256): Promise<string> {
    return await (window as any).go.app.App.GetImageThumbnailURL(this.normalizeCanonicalPath(path), maxWidth, maxHeight);
  }

  async getArchiveImageThumbnailURL(fsPath: string, nestedChain: string[] = [], entryPath: string, maxWidth: number = 256, maxHeight: number = 256): Promise<string> {
    return await (window as any).go.app.App.GetArchiveImageThumbnailURL(this.normalizeCanonicalPath(fsPath), nestedChain, entryPath, maxWidth, maxHeight);
  }

  async fetchTextFromURL(url: string, encoding: string = 'utf8'): Promise<StreamTextResult> {
    const response = await this.fetchStreamResponse(url);
    const bytes = await response.arrayBuffer();
    return {
      content: this.decodeTextBytes(bytes, encoding),
      size: bytes.byteLength,
    };
  }

  async fetchBytesFromURL(url: string): Promise<Uint8Array> {
    const response = await this.fetchStreamResponse(url);
    return new Uint8Array(await response.arrayBuffer());
  }

  async canConvertImage(path: string): Promise<boolean> {
    const ext = path.split('.').pop()?.toLowerCase() || '';
    if (!ext) return false;
    return await (window as any).go.app.App.CanConvertImage(ext);
  }

  async ConvertOfficeDocument(path: string): Promise<any> {
    return await (window as any).go.app.App.ConvertOfficeDocument(this.normalizeCanonicalPath(path));
  }

  async IsOfficeDocumentAvailable(): Promise<boolean> {
    return await (window as any).go.app.App.IsOfficeDocumentAvailable();
  }

  async readFileStream(path: string, chunkSize: number = 64 * 1024): Promise<string> {
    return await window.go.app.App.ReadFileStream(this.normalizeCanonicalPath(path), chunkSize);
  }

  async archiveReadStream(fsPath: string, nestedChain: string[] = [], entryPath: string, chunkSize: number = 64 * 1024): Promise<string> {
    return await window.go.app.App.ArchiveReadStream(this.normalizeCanonicalPath(fsPath), nestedChain, entryPath, chunkSize);
  }

  cancelReadFileStream(streamID: string): void {
    window.go.app.App.CancelReadFileStream(streamID).catch(() => {}); // Ignore errors
  }

  cancelArchiveReadStream(streamID: string): void {
    window.go.app.App.CancelArchiveReadStream(streamID).catch(() => {}); // Ignore errors
  }

  // ============================================================================
  // Navigation
  // ============================================================================

  async navigate(paneId: string, path: string, showHidden: boolean = false): Promise<DirectoryListing> {
    if (showHidden) {
	      return castProtocol<DirectoryListing>(await window.go.app.App.NavNavigateShowHidden(paneId, this.normalizeCanonicalPath(path), showHidden));
    }
    return castProtocol<DirectoryListing>(await window.go.app.App.NavNavigate(paneId, this.normalizeCanonicalPath(path)));
  }

  async navigateBack(paneId: string): Promise<DirectoryListing | null> {
    try {
      return castProtocol<DirectoryListing>(await window.go.app.App.NavBack(paneId));
    } catch (error) {
      console.warn('Cannot navigate back:', error);
      return null;
    }
  }

  async navigateForward(paneId: string): Promise<DirectoryListing | null> {
    try {
      return castProtocol<DirectoryListing>(await window.go.app.App.NavForward(paneId));
    } catch (error) {
      console.warn('Cannot navigate forward:', error);
      return null;
    }
  }

  async navigateUp(paneId: string): Promise<DirectoryListing | null> {
    try {
      return castProtocol<DirectoryListing>(await window.go.app.App.NavGoUp(paneId));
    } catch (error) {
      console.warn('Cannot navigate up:', error);
      return null;
    }
  }

  // ============================================================================
  // Workspace & Preferences
  // ============================================================================

  async getWorkspaces(): Promise<WorkspaceList> {
    return castProtocol<WorkspaceList>(await window.go.app.App.GetWorkspaces());
  }

  async saveWorkspace(workspace: WorkspaceConfig): Promise<void> {
    return await window.go.app.App.SaveWorkspace(contracts.WorkspaceConfig.createFrom(workspace));
  }

  async loadWorkspace(id: string): Promise<WorkspaceConfig> {
    return castProtocol<WorkspaceConfig>(await window.go.app.App.LoadWorkspace(id));
  }

  async deleteWorkspace(id: string): Promise<void> {
    return await window.go.app.App.DeleteWorkspace(id);
  }

  async getPreferences(): Promise<UserPreferences> {
    return castProtocol<UserPreferences>(await window.go.app.App.GetPreferences());
  }

  async savePreferences(prefs: UserPreferences): Promise<void> {
    return await window.go.app.App.SavePreferences(contracts.UserPreferences.createFrom(prefs));
  }

  // ============================================================================
  // Operation Progress Tracking
  // ============================================================================

  async getActiveOperations(): Promise<OperationProgress[]> {
    return castProtocol<OperationProgress[]>(await window.go.app.App.GetActiveOperations());
  }

  async cancelOperation(id: string): Promise<boolean> {
    return await window.go.app.App.CancelOperation(id);
  }

  async dismissOperation(id: string): Promise<void> {
    return await window.go.app.App.DismissOperation(id);
  }

  async resolveConflict(conflictId: string, resolution: ConflictResolution, applyToAll: boolean): Promise<boolean> {
    return await window.go.app.App.ResolveConflict(conflictId, resolution, applyToAll);
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
    const result = await window.go.app.App.SplitPath(this.normalizeCanonicalPath(path));
    return {
      dir: result['dir'] || '',
      base: result['base'] || ''
    };
  }

  async joinPaths(parts: string[]): Promise<string> {
    return await window.go.app.App.JoinPaths(this.normalizeCanonicalPathList(parts));
  }

  async getParentPath(path: string): Promise<string> {
    return await window.go.app.App.GetParentPath(this.normalizeCanonicalPath(path));
  }

  async confirmElevationRequest(requestID: string, approve: boolean): Promise<void> {
    return await window.go.app.App.ConfirmElevationRequest(requestID, approve);
  }

  // ── Persistent elevated service session (JWT-secured helper process) ───────

  async isProcessElevated(): Promise<boolean> {
    return window.go.app.App.IsProcessElevated();
  }

  async startElevatedSession(): Promise<ElevatedSessionStatus> {
    return normalizeElevatedSessionStatus(await window.go.app.App.StartElevatedSession());
  }

  async stopElevatedSession(): Promise<void> {
    return window.go.app.App.StopElevatedSession();
  }

  async getElevatedSessionStatus(): Promise<ElevatedSessionStatus> {
    return normalizeElevatedSessionStatus(await window.go.app.App.GetElevatedSessionStatus());
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
    return castProtocol<DiskUsageResult>(await window.go.app.App.AnalyzeDiskUsage(this.normalizeCanonicalPath(path)));
  }

  /**
   * Streaming disk-usage scan.
   *
    * Returns an AsyncGenerator that yields DiskUsageEntry objects as direct
    * child snapshots update during the scan, then yields the final
    * DiskUsageResult summary as the last item.
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
    options?: { signal?: AbortSignal },
  ): AsyncGenerator<
    DiskUsageEntry |
    DiskUsageProgress |
    DiskUsageResult
  > {
    const signal = options?.signal;
    const normalizedPath = this.normalizeCanonicalPath(path);
    const scanId = await window.go.app.App.StartDiskUsageStream(normalizedPath);

    type Pending =
      | { kind: 'entry'; value: DiskUsageEntry }
      | { kind: 'progress'; value: DiskUsageProgress }
      | { kind: 'done'; value: DiskUsageResult }
      | { kind: 'error';  error: Error };

    const queue: Pending[] = [];
    let wake: (() => void) | null = null;
    let finished = false;
    let aborted = false;

    const notify = () => { const r = wake; wake = null; r?.(); };

    const onAbort = () => {
      aborted = true;
      finished = true;
      notify();
    };

    if (signal?.aborted) {
      onAbort();
    }

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

    signal?.addEventListener('abort', onAbort, { once: true });

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
      signal?.removeEventListener('abort', onAbort);
      cleanup();
      window.go.app.App.CancelDiskUsageStream(scanId).catch(() => {/* best-effort */});
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
