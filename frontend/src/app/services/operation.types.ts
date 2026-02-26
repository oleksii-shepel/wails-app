export type OperationStatus = 'pending' | 'running' | 'paused' | 'completed' | 'error' | 'cancelled';
export type OperationKind = 'copy' | 'move' | 'delete' | 'archive' | 'extract' | 'search' | 'upload' | 'download';

// VS Code theme color tokens
export type ThemeColor = 
  | '--vsc-accent-blue'
  | '--vsc-accent-green' 
  | '--vsc-accent-yellow'
  | '--vsc-accent-orange'
  | '--vsc-accent-red'
  | '--vsc-accent-purple'
  | '--vsc-accent-teal'
  | '--vsc-accent-indigo'
  | '--vsc-info'
  | '--vsc-success'
  | '--vsc-warning'
  | '--vsc-error'
  | '--vsc-foreground-dim';

export interface OperationConfig {
  kind: OperationKind;
  title: string;
  icon: string;
  themeColor: ThemeColor;
  cancellable: boolean;
  pausable: boolean;
}

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

export interface OperationState {
  id: string;
  config: OperationConfig;
  status: OperationStatus;
  metrics: OperationMetrics;
  percentComplete: number;
  timing: OperationTiming;
  isMinimized: boolean;
  isCollapsed: boolean;
  stackIndex: number;
  error?: string;
}

// Backend events
export interface OperationStartEvent {
  id: string;
  kind: OperationKind;
  title: string;
  sources: string[];
  destination?: string;
  totalBytes: number;
  totalItems: number;
}

export interface OperationProgressEvent {
  id: string;
  processedBytes: number;
  processedItems: number;
  currentFile?: string;
  currentFileSize?: number;
  speedBps: number;
  elapsedMs: number;
  remainingMs: number;
}

export interface OperationCompleteEvent {
  id: string;
  status: Exclude<OperationStatus, 'pending' | 'running' | 'paused'>;
  error?: string;
  finalMetrics: OperationMetrics;
  timing: OperationTiming;
}