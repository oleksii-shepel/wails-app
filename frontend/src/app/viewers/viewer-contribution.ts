import type { Type } from '@angular/core';
import type { FileProbe } from '../shared/protocol';

export interface ViewerRequest {
  targetPath: string;
  fileList?: { path: string; name: string }[];
  probe?: FileProbe | null;
  /** Optional explicit viewer id (from tab metadata). */
  viewerId?: string | null;
}

export interface ViewerContribution {
  /** Stable id, used in tab metadata and diagnostics. */
  id: string;
  /** Human label (optional). */
  title?: string;
  /**
   * Return a score: 0 = cannot handle; higher wins.
   * Keep it cheap; do not read file bytes here.
   */
  score(req: ViewerRequest): number;
  /** Lazy-load the viewer surface component (standalone Angular component). */
  load(): Promise<Type<unknown>> | Type<unknown>;
}

