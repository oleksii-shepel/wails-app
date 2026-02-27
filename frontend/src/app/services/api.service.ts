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
  FileContent,
  FileInfo,
  OperationResult,
  SearchResult,
  WorkspaceConfig,
  UserPreferences,
  WorkspaceList,
  ArchiveListing,
  OperationProgress
} from '../shared/protocol-enhanced';

@Injectable({
  providedIn: 'root'
})
export class ApiService {
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
    return await window.go.main.App.GetOSInfo_();
  }

  async getDrives(): Promise<any> {
    return await window.go.main.App.GetDrives();
  }

  // ============================================================================
  // File System Operations
  // ============================================================================

  async listDirectory(path: string, showHidden: boolean = false): Promise<DirectoryListing> {
    return await window.go.main.App.ListDir(path, showHidden);
  }

  async listDirectoryFiltered(path: string, showHidden: boolean, filter: any): Promise<DirectoryListing> {
    return await window.go.main.App.ListDirFiltered(path, showHidden, filter);
  }

  async getFileInfo(path: string): Promise<FileInfo> {
    return await window.go.main.App.FileInfo(path);
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
  // Batch Operations (New)
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

  async batchDelete(paths: string[], recursive: boolean = true): Promise<BatchResult> {
    const request: BatchDeleteRequest = {
      paths,
      recursive
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

  async archiveReadFile(fsPath: string, nestedChain: string[] = [], entryPath: string): Promise<FileContent> {
    return await window.go.main.App.ArchiveRead(fsPath, nestedChain, entryPath);
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
  // Operation Progress Tracking (New)
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
    return await window.go.main.App.SplitPath(path);
  }

  async joinPaths(parts: string[]): Promise<string> {
    return await window.go.main.App.JoinPaths(parts);
  }

  async getParentPath(path: string): Promise<string> {
    return await window.go.main.App.GetParentPath(path);
  }
}