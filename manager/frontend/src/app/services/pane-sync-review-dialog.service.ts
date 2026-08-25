import { Injectable } from '@angular/core';
import { QueuedDialogGate } from './dialog-gate';
import type {
  PaneComparisonResult,
  PaneComparisonStatus,
} from './pane-comparison.service';
import type { SyncHistoryEntry } from './operation-history.service';

export type PaneSyncDirection = 'skip' | 'left-to-right' | 'right-to-left';
export type PaneSyncPreset = 'safe' | 'left-to-right' | 'right-to-left' | 'bidirectional';

export interface PaneSyncPlanItem {
  key: string;
  name: string;
  status: PaneComparisonStatus;
  direction: PaneSyncDirection;
  leftPath: string | null;
  rightPath: string | null;
}

export interface PaneSyncPlan {
  items: PaneSyncPlanItem[];
  leftToRight: string[];
  rightToLeft: string[];
  preset: PaneSyncPreset | 'custom';
  conflicts: number;
}

export interface PaneSyncReviewDialogViewModel {
  getResult(): PaneComparisonResult;
  getHistory(): readonly SyncHistoryEntry[];
  onExecute(plan: PaneSyncPlan): void | Promise<void>;
}

export interface PaneSyncReviewDialogRequest {
  viewModel: PaneSyncReviewDialogViewModel;
  resolve(): void;
}

@Injectable({ providedIn: 'root' })
export class PaneSyncReviewDialogService {
  private readonly gate = new QueuedDialogGate<PaneSyncReviewDialogRequest, void>();
  readonly activeRequest$ = this.gate.activeRequest$;
  readonly mountedRequest$ = this.gate.mountedRequest$;
  readonly closingRequest$ = this.gate.closingRequest$;

  openDialog(viewModel: PaneSyncReviewDialogViewModel): Promise<void> {
    return new Promise<void>((resolve) => {
      this.gate.enqueue({ viewModel, resolve });
    });
  }

  close(): void {
    this.gate.beginClose(undefined);
  }

  finishClose(): void {
    this.gate.finishClose();
  }

  notifyActiveChanged(): void {
    const active = this.gate.activeRequest;
    if (!active) {
      return;
    }
    this.activeRequest$.next({ ...active });
  }
}
