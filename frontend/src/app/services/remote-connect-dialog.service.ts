import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import { contracts } from '../../../wailsjs/go/models';

interface RemoteConnectDialogRequest {
  resolve: (value: contracts.RemoteMount | null) => void;
  initialName?: string;
  initialProtocol?: string;
}

@Injectable({ providedIn: 'root' })
export class RemoteConnectDialogService {
  public readonly activeRequest$ = createBehaviorSubject<RemoteConnectDialogRequest | null>(null);

  private readonly queue: RemoteConnectDialogRequest[] = [];

  openDialog(options?: { name?: string; protocol?: string }): Promise<contracts.RemoteMount | null> {
    return new Promise<contracts.RemoteMount | null>((resolve) => {
      this.queue.push({
        resolve,
        initialName: options?.name?.trim() || undefined,
        initialProtocol: options?.protocol?.trim() || undefined,
      });
      this.tryOpenNext();
    });
  }

  openS3Dialog(options?: { name?: string }): Promise<contracts.RemoteMount | null> {
    return this.openDialog({ ...options, protocol: 's3' });
  }

  resolveActive(result: contracts.RemoteMount | null): void {
    const active = this.activeRequest$.value;
    if (!active) {
      return;
    }

    active.resolve(result);
    this.activeRequest$.next(null);
    this.tryOpenNext();
  }

  private tryOpenNext(): void {
    if (this.activeRequest$.value) {
      return;
    }
    this.activeRequest$.next(this.queue.shift() ?? null);
  }
}