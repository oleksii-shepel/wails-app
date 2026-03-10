import type { Type } from '@angular/core';
import type { TabInfo } from '../shared/protocol-enhanced';

export interface SurfaceRequest {
  tab: TabInfo;
}

export interface SurfaceContribution {
  id: string;
  /** Higher wins; 0 = cannot handle. */
  score(req: SurfaceRequest): number;
  load(): Promise<Type<unknown>> | Type<unknown>;
}

