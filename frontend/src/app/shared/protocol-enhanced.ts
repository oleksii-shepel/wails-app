/**
 * Enhanced protocol definitions with Phase 1 features:
 * - Tabs
 * - Workspaces
 * - Address Bar
 * - Item Filter
 * - Batch Operations
 * - Progress Tracking
 */

// Re-export everything from the base protocol so consumers only need one import.
export * from './protocol';

// ============================================================================
// Quick Access / Special Folders
// ============================================================================

// ============================================================================
// Tab Management
// ============================================================================

export interface TabInfo {
  id: string;
  title: string;
  path: string;
  /** "dir" (default) or "viewer". Old persisted workspaces omit this field. */
  kind?: 'dir' | 'viewer';
  /** Viewer tab payload; only meaningful when kind === 'viewer'. */
  viewer?: {
    /** Absolute filesystem path to the viewed file (initially FS-only). */
    targetPath: string;
    /** Optional explicit viewer selection override. */
    viewerId?: string;
    /** When true, this tab's target auto-follows selection in dir tabs. */
    autoFollow?: boolean;
    /** Origin of this preview (used to restore cursor when switching previews). */
    sourcePane?: 'left' | 'right';
    sourceTabId?: string;
    sourceDir?: string;
    /**
     * Present when the viewed file lives inside an archive.
     * The viewer uses this to call ArchiveReadFile (in-memory extraction)
     * instead of ReadFile.  Cleared when autoFollow changes the target to a
     * regular filesystem file.
     */
    archiveContext?: {
      /** Outermost archive on disk, e.g. "C:/foo/archive.zip" */
      fsPath: string;
      /** Nested archive chain, e.g. ["inner.tar.gz"] — empty for flat archives */
      nestedChain: string[];
      /** Entry path inside the innermost archive, e.g. "src/main.go" */
      entryPath: string;
    };
  };
  /** If false, the UI hides the close button. Defaults to true when omitted. */
  isClosable?: boolean;
  isActive: boolean;
  isPinned: boolean;
  createdAt: number;
}

export interface PaneState {
  /** e.g. 'left' | 'right' */
  id: string;
  tabs: TabInfo[];
  activeTabId: string;
}

// ============================================================================
// Workspace Management
// ============================================================================

export interface WorkspaceConfig {
  id: string;
  name: string;
  icon?: string;
  color?: string;
  leftPane: PaneState;
  rightPane: PaneState;
  showHidden: boolean;
  createdAt: number;
  lastAccessedAt: number;
}

export interface WorkspaceList {
  workspaces: WorkspaceConfig[];
  activeWorkspaceId: string;
}

// ============================================================================
// Filter System
// ============================================================================

export enum FilterType {
  NAME = 'name',
  EXTENSION = 'ext',
  TYPE = 'type',
  SIZE = 'size',
  MODIFIED = 'modified',
  GLOB = 'glob',
}

export interface FilterCriteria {
  type: FilterType;
  value: string;
  operator?:
    | 'equals'
    | 'contains'
    | 'startsWith'
    | 'endsWith'
    | 'gt'
    | 'lt'
    | 'gte'
    | 'lte';
}

export interface FilterQuery {
  /** Simple text search across file names */
  text?: string;
  /** Advanced multi-criteria filters */
  criteria?: FilterCriteria[];
  caseSensitive?: boolean;
}

// ============================================================================
// Operation Types (from backend)
// ============================================================================

export type OperationKind = 'copy' | 'move' | 'delete' | 'archive' | 'extract' | 'search';
export type OperationStatus = 'pending' | 'running' | 'paused' | 'completed' | 'error' | 'cancelled';

export type ConflictResolution = 'skip' | 'overwrite' | 'rename' | 'cancel';

export interface ConflictEvent {
  operationId: string;
  conflictId: string;
  sourcePath: string;
  destPath: string;
  sourceSize: number;
  destSize: number;
  sourceMtime: number;
  destMtime: number;
  isDirectory: boolean;
}

export interface ConflictResolutionRequest {
  resolution: ConflictResolution;
  applyToAll: boolean;
}

// ============================================================================
// Batch Operations
// ============================================================================

export interface BatchItem {
  source: string;
  destination: string;
}

export interface BatchRequest {
  items: BatchItem[];
  recursive: boolean;
}

export interface BatchResult {
  operationId: string;
  total: number;
  succeeded: number;
  skipped: number;
  failed: number;
  errors?: BatchItemError[];
}

export interface BatchItemError {
  source: string;
  destination?: string;
  error: string;
}

export interface BatchDeleteRequest {
  paths: string[];
  recursive: boolean;
  /** Move to OS recycle bin / trash instead of permanently deleting. Defaults to true. */
  useTrash?: boolean;
}

// ============================================================================
// Archive Extraction
// ============================================================================

export interface BatchExtractRequest {
  archives: string[];
  destinationDir: string;
  createSubdirectory?: boolean;
  expandNestedArchives?: boolean;
  maxNestDepth?: number;
  flattenSingleRoot?: boolean;
}

export interface BatchExtractResult {
  operationId: string;
  total: number;
  succeeded: number;
  skipped: number;
  failed: number;
  extractedFiles: number;
  extractedBytes: number;
  errors?: BatchItemError[];
}

export interface SelectiveExtractItem {
  archivePath: string;
  nestedChain?: string[];
  entries?: string[];
  destinationDir?: string;
  flattenPaths?: boolean;
}

export interface BatchSelectiveExtractRequest {
  items: SelectiveExtractItem[];
  destinationDir: string;
}

export interface BatchSelectiveExtractResult {
  operationId: string;
  total: number;
  succeeded: number;
  skipped: number;
  failed: number;
  extractedFiles: number;
  extractedBytes: number;
  errors?: BatchItemError[];
}

// ============================================================================
// Progress Tracking (from backend events)
// ============================================================================

export interface OperationMetrics {
  totalBytes: number;
  processedBytes: number;
  totalItems: number;
  processedItems: number;
  currentFile?: string;
  currentFileSize?: number;
}

export interface OperationTiming {
  startedAt: number;
  elapsedMs: number;
  estimatedRemainingMs: number;
  speedBytesPerSecond: number;
}

export interface OperationProgress {
  id: string;
  type: OperationKind;
  status: OperationStatus;
  title: string;
  sourcePath: string;
  destinationPath?: string;
  startTime: number;
  endTime?: number;
  totalBytes: number;
  processedBytes: number;
  totalItems: number;
  processedItems: number;
  currentFile?: string;
  currentFileSize?: number;
  percentComplete: number;
  avgSpeedBytesPerSec: number;
  speedBytesPerSec: number;
  elapsedSeconds: number;
  etaSeconds: number;
  error?: string;
}

// ============================================================================
// Event Payloads (from backend)
// ============================================================================

export interface OpStartEvent {
  id: string;
  kind: string;
  title: string;
  sources: string[];
  destination?: string;
  totalBytes: number;
  totalItems: number;
}

export interface OpProgressEvent {
  id: string;
  processedBytes: number;
  processedItems: number;
  currentFile?: string;
  currentFileSize?: number;
  speedBps: number;
  elapsedMs: number;
  remainingMs: number;
}

export interface OpCompleteEvent {
  id: string;
  status: string;
  error?: string;
  finalMetrics: {
    totalBytes: number;
    processedBytes: number;
    totalItems: number;
    processedItems: number;
  };
  timing: {
    startedAt: number;
    elapsedMs: number;
    estimatedRemainingMs: number;
    speedBytesPerSecond: number;
  };
}

// ============================================================================
// Enhanced Commands
// ============================================================================

export enum EnhancedCommandType {
  // Workspace commands
  SAVE_WORKSPACE = 'SAVE_WORKSPACE',
  LOAD_WORKSPACE = 'LOAD_WORKSPACE',
  DELETE_WORKSPACE = 'DELETE_WORKSPACE',
  LIST_WORKSPACES = 'LIST_WORKSPACES',

  // Tab commands
  SAVE_TAB_STATE = 'SAVE_TAB_STATE',
  LOAD_TAB_STATE = 'LOAD_TAB_STATE',

  // Filter commands (client-side mostly, but may need server for complex patterns)
  APPLY_FILTER = 'APPLY_FILTER',
}

export interface SaveWorkspaceCommand {
  type: EnhancedCommandType.SAVE_WORKSPACE;
  workspace: WorkspaceConfig;
}

export interface LoadWorkspaceCommand {
  type: EnhancedCommandType.LOAD_WORKSPACE;
  workspaceId: string;
}

export interface DeleteWorkspaceCommand {
  type: EnhancedCommandType.DELETE_WORKSPACE;
  workspaceId: string;
}

export interface ListWorkspacesCommand {
  type: EnhancedCommandType.LIST_WORKSPACES;
}

// ============================================================================
// Local Storage Keys
// ============================================================================

export const STORAGE_KEYS = {
  WORKSPACES: 'filemanager:workspaces',
  ACTIVE_WORKSPACE: 'filemanager:active-workspace',
  TAB_STATE_PREFIX: 'filemanager:tabs:',
  PREFERENCES: 'filemanager:preferences',
  HISTORY: 'filemanager:history',
} as const;

// ============================================================================
// User Preferences
// ============================================================================

export interface UserPreferences {
  theme: 'dark' | 'light' | 'auto';
  defaultShowHidden: boolean;
  defaultSortBy: 'name' | 'size' | 'modified' | 'type';
  defaultSortOrder: 'asc' | 'desc';
  enableKeyboardShortcuts: boolean;
  doubleClickAction: 'open' | 'preview';
  confirmDelete: boolean;
}

// ============================================================================
// Navigation History
// ============================================================================

export interface HistoryEntry {
  path: string;
  timestamp: number;
  paneId: string;
}

export interface NavigationHistory {
  entries: HistoryEntry[];
  maxEntries: number;
}