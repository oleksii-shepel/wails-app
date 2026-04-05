import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';

export type RemoteProtocol = 'ftp' | 'sftp' | 'smb' | 'webdav';

export interface RemoteConnectionRequest {
  protocol: RemoteProtocol;
  host: string;
  port: number;
}

const DEFAULT_PORTS: Record<RemoteProtocol, number> = {
  ftp: 21,
  sftp: 22,
  smb: 445,
  webdav: 443,
};

export function parseRemoteUrl(raw: string): RemoteConnectionRequest | null {
  const match = /^(ftp|sftp|smb|webdav):\/\/([^/:]+)(?::(\d+))?\/?.*$/i.exec(raw);
  if (!match) return null;
  const protocol = match[1].toLowerCase() as RemoteProtocol;
  return {
    protocol,
    host: match[2],
    port: match[3] ? parseInt(match[3], 10) : DEFAULT_PORTS[protocol],
  };
}

interface PendingRequest {
  prefill: RemoteConnectionRequest;
  resolve: (mountPoint: string | null) => void;
}

@Injectable({ providedIn: 'root' })
export class RemoteConnectionService {
  readonly request$ = createBehaviorSubject<PendingRequest | null>(null);

  /** Opens the dialog pre-filled from a URL. Resolves with mount-point on success, null on cancel. */
  open(prefill: RemoteConnectionRequest): Promise<string | null> {
    return new Promise<string | null>(resolve => {
      this.request$.next({ prefill, resolve });
    });
  }

  resolve(mountPoint: string | null): void {
    const pending = this.request$.value;
    if (!pending) return;
    pending.resolve(mountPoint);
    this.request$.next(null);
  }
}
