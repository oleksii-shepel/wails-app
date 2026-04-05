import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';

export type RemoteProtocol = 'ftp' | 'sftp' | 'smb' | 'webdav';

export interface RemoteConnectionRequest {
  protocol: RemoteProtocol;
  host: string;
  port: number;
  username?: string;
  password?: string;
  share?: string;
  schema?: 'http' | 'https';
  basePath?: string;
  pathSuffix?: string;
}

const DEFAULT_PORTS: Record<RemoteProtocol, number> = {
  ftp: 21,
  sftp: 22,
  smb: 445,
  webdav: 443,
};

export function parseRemoteUrl(raw: string): RemoteConnectionRequest | null {
  const value = (raw || '').trim().replace(/\\/g, '/');
  if (!value) return null;

  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    return null;
  }

  const protocol = parsed.protocol.replace(/:$/, '').toLowerCase() as RemoteProtocol;
  if (!(protocol in DEFAULT_PORTS) || !parsed.hostname) {
    return null;
  }

  const pathSegments = parsed.pathname
    .replace(/^\/+/, '')
    .replace(/\/+$/, '')
    .split('/')
    .filter(Boolean);

  const request: RemoteConnectionRequest = {
    protocol,
    host: parsed.hostname,
    port: parsed.port ? parseInt(parsed.port, 10) : DEFAULT_PORTS[protocol],
    username: parsed.username ? decodeURIComponent(parsed.username) : undefined,
    password: parsed.password ? decodeURIComponent(parsed.password) : undefined,
  };

  switch (protocol) {
    case 'smb': {
      request.share = pathSegments[0] || undefined;
      request.pathSuffix = pathSegments.slice(1).join('/') || undefined;
      break;
    }
    case 'webdav': {
      const normalizedBasePath = parsed.pathname
        .replace(/\/+$/, '')
        .replace(/^\/+/, '/');
      request.schema = request.port === 80 ? 'http' : 'https';
      request.basePath = normalizedBasePath && normalizedBasePath !== '/' ? normalizedBasePath : undefined;
      break;
    }
    case 'sftp':
    case 'ftp': {
      request.pathSuffix = pathSegments.join('/') || undefined;
      break;
    }
  }

  return request;
}

export function resolveMountedRemotePath(mountPath: string, request: RemoteConnectionRequest): string {
  const base = (mountPath || '').replace(/\/+$/, '');
  if (!base) {
    return mountPath;
  }

  const suffix = (request.pathSuffix || '').replace(/^\/+/, '').replace(/\/+$/, '');
  if (!suffix) {
    return base;
  }

  return `${base}/${suffix}`;
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
