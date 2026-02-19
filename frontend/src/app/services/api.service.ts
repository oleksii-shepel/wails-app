import { Injectable } from '@angular/core';
import {
  OSInfoResponse,
  DrivesList,
  DriveInfo,
  DirectoryListing,
  FileContent,
  FileInfo,
  OperationResult,
  SearchResult,
} from '../shared/protocol';
import {
  FilterQuery,
} from '../shared/protocol-enhanced';

/**
 * Thin wrapper around the Wails-generated Go bindings.
 *
 * Every method delegates directly to `window['go']['main']['App']`, which is
 * injected by the Wails runtime.  The old HTTP/WebSocket transport has been
 * removed entirely – there is no server to connect to in a Wails desktop app.
 *
 * Method signatures are kept identical to the previous ApiService so that all
 * callers (components, other services) compile without changes.
 */
@Injectable({
  providedIn: 'root',
})
export class ApiService {

  // ---------------------------------------------------------------------------
  // Internal helper – single point of entry into the Wails bridge
  // ---------------------------------------------------------------------------

  /** Call a method on the bound Go App struct via the Wails bridge. */
  private call<T>(method: string, ...args: unknown[]): Promise<T> {
    const bridge = (window as any)?.go?.main?.App;
    if (!bridge) {
      return Promise.reject(
        new Error(
          `Wails bridge not available. Make sure the app is running inside the Wails runtime (window.go.main.App is undefined).`
        )
      );
    }
    if (typeof bridge[method] !== 'function') {
      return Promise.reject(new Error(`Wails bridge method '${method}' not found.`));
    }
    return bridge[method](...args) as Promise<T>;
  }

  // ---------------------------------------------------------------------------
  // OS / System
  // ---------------------------------------------------------------------------

  /** Retrieve host OS details (OS name, version, arch, hostname, system drive). */
  async getOSInfo(): Promise<OSInfoResponse> {
    return this.call<OSInfoResponse>('GetOSInfo');
  }

  // ---------------------------------------------------------------------------
  // Drives
  // ---------------------------------------------------------------------------

  /** List available drives. Returns all mounts on Linux/macOS, lettered drives on Windows. */
  async listDrives(): Promise<DriveInfo[]> {
    const result = await this.call<DrivesList>('GetDrives');
    return result.drives;
  }

  // ---------------------------------------------------------------------------
  // Directory / File listing
  // ---------------------------------------------------------------------------

  /** List the contents of a directory. */
  async listDirectory(path: string, showHidden = false): Promise<DirectoryListing> {
    return this.call<DirectoryListing>('ListDir', path, showHidden);
  }

  /**
   * List directory contents with a filter query applied server-side.
   * Falls back to client-side filtering via FilterService if not needed.
   */
  async listDirectoryFiltered(
    path: string,
    showHidden = false,
    filter?: FilterQuery,
  ): Promise<DirectoryListing> {
    if (!filter) {
      return this.listDirectory(path, showHidden);
    }
    return this.call<DirectoryListing>('ListDirFiltered', path, showHidden, filter);
  }

  // ---------------------------------------------------------------------------
  // File I/O
  // ---------------------------------------------------------------------------

  /** Read a file's contents, optionally as base64. */
  async readFile(path: string, encoding: 'utf8' | 'base64' = 'utf8'): Promise<FileContent> {
    return this.call<FileContent>('ReadFile', path, encoding);
  }

  /** Write content to a file. */
  async writeFile(
    path: string,
    content: string,
    encoding: 'utf8' | 'base64' = 'utf8',
  ): Promise<OperationResult> {
    return this.call<OperationResult>('WriteFile', path, content, encoding);
  }

  // ---------------------------------------------------------------------------
  // File operations
  // ---------------------------------------------------------------------------

  /** Delete a file or directory. Pass `recursive: true` for non-empty directories. */
  async deleteFile(path: string, recursive = false): Promise<OperationResult> {
    return this.call<OperationResult>('Delete', path, recursive);
  }

  /** Create a directory, optionally creating all parent directories. */
  async createDirectory(path: string, recursive = true): Promise<OperationResult> {
    return this.call<OperationResult>('CreateDir', path, recursive);
  }

  /** Move (rename) a file or directory. */
  async moveFile(source: string, destination: string): Promise<OperationResult> {
    return this.call<OperationResult>('Move', source, destination);
  }

  /** Copy a file or directory. Pass `recursive: true` for directories. */
  async copyFile(
    source: string,
    destination: string,
    recursive = false,
  ): Promise<OperationResult> {
    return this.call<OperationResult>('Copy', source, destination, recursive);
  }

  // ---------------------------------------------------------------------------
  // File metadata
  // ---------------------------------------------------------------------------

  /** Retrieve metadata for a single file or directory. */
  async getFileInfo(path: string): Promise<FileInfo> {
    return this.call<FileInfo>('GetFileInfo', path);
  }

  // ---------------------------------------------------------------------------
  // Search
  // ---------------------------------------------------------------------------

  /** Search for files matching a pattern within a directory. */
  async searchFiles(path: string, pattern: string, recursive = true): Promise<SearchResult> {
    return this.call<SearchResult>('Search', path, pattern, recursive);
  }

  // ---------------------------------------------------------------------------
  // Path utilities
  // ---------------------------------------------------------------------------

  /** Check whether a path exists on the filesystem. */
  async pathExists(path: string): Promise<boolean> {
    return this.call<boolean>('PathExists', path);
  }

  /** Check whether a path points to a directory. */
  async isDirectory(path: string): Promise<boolean> {
    return this.call<boolean>('IsDirectory', path);
  }

  /** Join path segments using the OS separator. */
  async joinPaths(parts: string[]): Promise<string> {
    return this.call<string>('JoinPaths', parts);
  }

  /** Get the parent path of a given path. */
  async getParentPath(path: string): Promise<string> {
    return this.call<string>('GetParentPath', path);
  }

  /** Split a path into { dir, base, ext } parts. */
  async splitPath(path: string): Promise<Record<string, string>> {
    return this.call<Record<string, string>>('SplitPath', path);
  }

  /** Return breadcrumb segments for a path. */
  async breadcrumbs(path: string): Promise<Array<Record<string, string>>> {
    return this.call<Array<Record<string, string>>>('Breadcrumbs', path);
  }

  // ---------------------------------------------------------------------------
  // Navigation history (server-side, per-pane)
  // ---------------------------------------------------------------------------

  /** Navigate to a path inside a pane, recording history. */
  async navNavigate(paneId: string, path: string): Promise<DirectoryListing> {
    return this.call<DirectoryListing>('NavNavigate', paneId, path);
  }

  /** Navigate to a path with show-hidden toggle. */
  async navNavigateShowHidden(
    paneId: string,
    path: string,
    showHidden: boolean,
  ): Promise<DirectoryListing> {
    return this.call<DirectoryListing>('NavNavigateShowHidden', paneId, path, showHidden);
  }

  /** Go back in navigation history for a pane. */
  async navBack(paneId: string): Promise<DirectoryListing> {
    return this.call<DirectoryListing>('NavBack', paneId);
  }

  /** Go forward in navigation history for a pane. */
  async navForward(paneId: string): Promise<DirectoryListing> {
    return this.call<DirectoryListing>('NavForward', paneId);
  }

  /** Navigate to the parent directory. */
  async navGoUp(paneId: string): Promise<DirectoryListing> {
    return this.call<DirectoryListing>('NavGoUp', paneId);
  }

  /** Check whether back navigation is possible. */
  async navCanGoBack(paneId: string): Promise<boolean> {
    return this.call<boolean>('NavCanGoBack', paneId);
  }

  /** Check whether forward navigation is possible. */
  async navCanGoForward(paneId: string): Promise<boolean> {
    return this.call<boolean>('NavCanGoForward', paneId);
  }

  /** Get the current path recorded for a pane. */
  async navCurrentPath(paneId: string): Promise<string> {
    return this.call<string>('NavCurrentPath', paneId);
  }

  // ---------------------------------------------------------------------------
  // Preferences
  // ---------------------------------------------------------------------------

  /** Load user preferences from the backend. */
  async getPreferences(): Promise<any> {
    return this.call<any>('GetPreferences');
  }

  /** Persist user preferences. */
  async savePreferences(prefs: any): Promise<void> {
    return this.call<void>('SavePreferences', prefs);
  }

  // ---------------------------------------------------------------------------
  // Health / diagnostics
  // ---------------------------------------------------------------------------

  /**
   * Ping the Go backend.
   * Returns a Unix-ms timestamp; resolves to true if the call succeeds.
   */
  async checkHealth(): Promise<boolean> {
    try {
      await this.call<number>('Ping');
      return true;
    } catch {
      return false;
    }
  }

  // ---------------------------------------------------------------------------
  // Legacy no-ops kept for interface compatibility
  // ---------------------------------------------------------------------------

  /**
   * No-op in the Wails version – there is no WebSocket to connect to.
   * Kept so existing call sites don't break at compile time.
   */
  connectWebSocket(): void {
    // Wails uses IPC; no WebSocket needed.
  }

  /** No-op in the Wails version. */
  disconnectWebSocket(): void {
    // Wails uses IPC; no WebSocket needed.
  }
}