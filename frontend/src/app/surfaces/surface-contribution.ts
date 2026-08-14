import type { Type } from '@angular/core';
import type { TabInfo } from '../shared/protocol-enhanced';
import type { SurfaceContext } from './surface-context';

export interface SurfaceRequest {
  tab: TabInfo;
  /** Available when resolution happens from a mounted pane surface. */
  context?: SurfaceContext;
}

export interface SurfaceContribution {
  /** Stable surface id. Tabs persist this value as `surfaceId`. */
  id: string;
  /** Higher wins when no exact persisted surface id is registered. */
  score(req: SurfaceRequest): number;
  load(): Promise<Type<unknown>> | Type<unknown>;
}
