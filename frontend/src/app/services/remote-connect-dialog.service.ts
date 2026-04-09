import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import { contracts } from '../../../wailsjs/go/models';

interface RemoteConnectDialogRequest {
  resolve: (value: contracts.RemoteMount | null) => void;
}

@Injectable({ providedIn: 'root' })
export class RemoteConnectDialogService {
  public readonly activeRequest$ = createBehaviorSubject<RemoteConnectDialogRequest | null>(null);

  private readonly queue: RemoteConnectDialogRequest[] = [];

  openS3Dialog(): Promise<contracts.RemoteMount | null> {
    return new Promise<contracts.RemoteMount | null>((resolve) => {
      this.queue.push({ resolve });
      this.tryOpenNext();
    });
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