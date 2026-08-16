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

export type LocationKind = 'location' | 'query' | 'surface';

export interface LocationReference {
  location: string;
  scheme: string;
  kind: LocationKind;
}

export interface TabInfo {
  id: string;
  title: string;
  path: string;
  /** Canonical durable navigation target. Legacy workspaces may omit this and fall back to path. */
  location?: LocationReference;
  /** Stable rendered surface id. Legacy workspaces may omit this and fall back to kind. */
  surfaceId?: string;
  /** "dir" (default), "viewer", or "size-manager". Old persisted workspaces omit this field. */
  kind?: 'dir' | 'viewer' | 'size-manager';
  /** Viewer tab payload; only meaningful when kind === 'viewer'. */
  viewer?: {
    /** Absolute filesystem path to the viewed file (initially FS-only). */
    targetPath: string;
    /** Sibling filesystem files from the source pane, used for viewer next/previous navigation. */
    fileList?: ViewerFileListItem[];
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
      * The viewer uses this to call ArchiveRead through the shared VFS-backed
      * archive path instead of ReadFile. Cleared when autoFollow changes the
      * target to a regular filesystem file.
     */
    archiveContext?: {
      /** Outermost archive on disk, e.g. "filesystem://c/foo/archive.zip" */
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
  /**
   * JSON-encoded per-tab view state (sort, view mode, filter, selection, cursor).
   * Opaque to the backend — produced and consumed by `DirTabStateController`.
   */
  viewStateJson?: string;
}

export interface ViewerFileListItem {
  path: string;
  name: string;
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
  /** Which pane held focus when the workspace was last saved. */
  activePaneId?: 'left' | 'right';
  /** Left-pane width as a percentage of the split container (0-100). */
  splitRatio?: number;
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

export type OperationKind = 'copy' | 'move' | 'delete' | 'archive' | 'extract' | 'search' | 'calculate';
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
  exactDestination?: boolean;
}

export interface BatchRequest {
  items: BatchItem[];
  recursive: boolean;
}

export interface BatchAffectedPath {
  source: string;
  destination: string;
}

export interface BatchResult {
  operationId: string;
  total: number;
  succeeded: number;
  skipped: number;
  failed: number;
  errors?: BatchItemError[];
  affectedPaths?: BatchAffectedPath[];
}

export type OperationErrorCode =
  | 'permission_denied'
  | 'not_found'
  | 'disk_full'
  | 'read_only'
  | 'already_exists'
  | 'invalid_path'
  | 'cancelled'
  | 'unknown';

const ERROR_CODE_MESSAGES: Record<OperationErrorCode, string> = {
  permission_denied: 'Permission denied',
  not_found:         'File not found',
  disk_full:         'Disk is full',
  read_only:         'Destination is read-only',
  already_exists:    'File already exists',
  invalid_path:      'Invalid path',
  cancelled:         'Cancelled',
  unknown:           'Unexpected error',
};

/** Returns a short, user-friendly label for the given error code. */
export function operationErrorMessage(code: OperationErrorCode): string {
  return ERROR_CODE_MESSAGES[code] ?? 'Unexpected error';
}

export interface BatchItemError {
  source: string;
  destination?: string;
  error: string;
  code: OperationErrorCode;
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
// Disk Usage Analyzer
// ============================================================================

export interface DiskUsageTypeStat {
  count: number;
  size: number;
}

export interface DiskUsageEntry {
  name: string;
  path: string;
  size: number;
  isDir: boolean;
  files: number;
  dirs: number;
  types?: Record<string, DiskUsageTypeStat>;
  error?: string;
}

export interface DiskUsageResult {
  path: string;
  totalSize: number;
  totalFiles: number;
  totalDirs: number;
  types?: Record<string, DiskUsageTypeStat>;
  entries: DiskUsageEntry[];
  elapsedMs: number;
}

export interface DiskUsageProgress {
  totalSize: number;
  totalFiles: number;
  totalDirs: number;
  elapsedMs: number;
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

export interface WindowState {
  width: number;
  height: number;
  x: number;
  y: number;
  maximised: boolean;
  /** Distinguishes "x/y captured" from default zero. Older records may omit it. */
  hasPosition?: boolean;
}

export interface UserPreferences {
  theme: 'dark' | 'light' | 'system' | 'auto';
  accentColor: string;
  uiZoom: number;
  compactMode: boolean;
  animationsEnabled: boolean;
  windowState?: WindowState;
  defaultShowHidden: boolean;
  defaultSortBy: 'name' | 'size' | 'modified' | 'type';
  defaultSortOrder: 'asc' | 'desc';
  enableKeyboardShortcuts: boolean;
  doubleClickAction: 'open' | 'preview';
  confirmDelete: boolean;
  confirmOnClose: boolean;
  reopenLastWorkspace: boolean;
  memoryRoot?: string;
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