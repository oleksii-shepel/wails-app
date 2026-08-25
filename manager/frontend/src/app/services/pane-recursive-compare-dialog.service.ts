import { Injectable } from '@angular/core';
import { QueuedDialogGate } from './dialog-gate';
import type {
  PaneComparisonPolicy,
  PaneComparisonResult,
} from './pane-comparison.service';

export interface PaneRecursiveCompareDialogViewModel {
  getLeftPath(): string;
  getRightPath(): string;
  getResult(): PaneComparisonResult;
  getPolicy(): PaneComparisonPolicy;
  getScanning(): boolean;
  getError(): string;
  onPolicyChange(policy: PaneComparisonPolicy): void | Promise<void>;
  onRefresh(): void | Promise<void>;
  onSelectLeft(): void;
  onSelectRight(): void;
  onCopyMissingLeftToRight(): void | Promise<void>;
  onCopyMissingRightToLeft(): void | Promise<void>;
  onOpenSyncReview(): void | Promise<void>;
}

export interface PaneRecursiveCompareDialogRequest {
  viewModel: PaneRecursiveCompareDialogViewModel;
  resolve(): void;
}

@Injectable({ providedIn: 'root' })
export class PaneRecursiveCompareDialogService {
  private readonly gate = new QueuedDialogGate<PaneRecursiveCompareDialogRequest, void>();
  readonly activeRequest$ = this.gate.activeRequest$;
  readonly mountedRequest$ = this.gate.mountedRequest$;
  readonly closingRequest$ = this.gate.closingRequest$;

  openDialog(viewModel: PaneRecursiveCompareDialogViewModel): Promise<void> {
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
