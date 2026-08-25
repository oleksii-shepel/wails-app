import { Injectable } from '@angular/core';
import { contracts } from '../../../wailsjs/go/models';
import { QueuedDialogGate } from './dialog-gate';

interface RemoteConnectDialogRequest {
  resolve(value: contracts.RemoteMount | null): void;
  initialName?: string;
  initialProtocol?: string;
}

@Injectable({ providedIn: 'root' })
export class RemoteConnectDialogService {
  private readonly gate = new QueuedDialogGate<RemoteConnectDialogRequest, contracts.RemoteMount | null>();
  public readonly activeRequest$ = this.gate.activeRequest$;
  public readonly mountedRequest$ = this.gate.mountedRequest$;
  public readonly closingRequest$ = this.gate.closingRequest$;

  openDialog(options?: { name?: string; protocol?: string }): Promise<contracts.RemoteMount | null> {
    return new Promise<contracts.RemoteMount | null>((resolve) => {
      this.gate.enqueue({
        resolve,
        initialName: options?.name?.trim() || undefined,
        initialProtocol: options?.protocol?.trim() || undefined,
      });
    });
  }

  openS3Dialog(options?: { name?: string }): Promise<contracts.RemoteMount | null> {
    return this.openDialog({ ...options, protocol: 's3' });
  }

  resolveActive(result: contracts.RemoteMount | null): void {
    this.gate.beginClose(result);
  }

  finishClose(): void {
    this.gate.finishClose();
  }
}
