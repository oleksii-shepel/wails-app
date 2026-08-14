import type { Type } from '@angular/core';
import type { TabInfo } from '../shared/protocol-enhanced';
import type { SurfaceContext } from './surface-context';

export type SurfaceCapability =
  | 'navigate'
  | 'selection'
  | 'fileOperations'
  | 'preview'
  | 'viewerControls';

export interface SurfaceRequest {
  tab: TabInfo;
  context?: SurfaceContext;
}

export interface SurfaceContribution {
  id: string;
  /** Commands/features this surface can participate in. */
  capabilities?: readonly SurfaceCapability[];
  /** Higher wins; 0 = cannot handle. Used as a compatibility fallback. */
  score(req: SurfaceRequest): number;
  load(): Promise<Type<unknown>> | Type<unknown>;
}
