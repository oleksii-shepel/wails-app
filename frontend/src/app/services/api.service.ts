// src/types/wails.d.ts
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

declare global {
  interface Window {
    go: {
      main: {
        App: {
          // System
          Ping(): Promise<number>;
          GetOSInfo_(): Promise<any>;
          GetDrives(): Promise<any>;
          
          // File System
          ListDir(path: string, showHidden: boolean): Promise<DirectoryListing>;
          ListDirFiltered(path: string, showHidden: boolean, filter: any): Promise<DirectoryListing>;
          FileInfo(path: string): Promise<FileInfo>;
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
          ArchiveRead(fsPath: string, nestedChain: string[], entryPath: string): Promise<FileContent>;
          
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

export {};