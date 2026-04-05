import { Component, OnDestroy } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { DialogFocusTrapDirective } from '../directives/dialog-focus-trap.directive';
import { RemoteConnectionService, RemoteProtocol } from '../services/remote-connection.service';
import { RemoteConnect } from 'wailsjs/go/app/App';
import { contracts } from 'wailsjs/go/models';
import type { Stream } from '@epikodelabs/streamix';
import { createSubject, takeUntil } from '@epikodelabs/streamix';

const PROTOCOL_LABELS: Record<RemoteProtocol, string> = {
  ftp: 'FTP',
  sftp: 'SFTP',
  smb: 'SMB',
  webdav: 'WebDAV',
};

const DEFAULT_PORTS: Record<RemoteProtocol, number> = {
  ftp: 21,
  sftp: 22,
  smb: 445,
  webdav: 443,
};

@Component({
  selector: 'app-remote-connection-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogFocusTrapDirective],
  template: `
    <ng-container *ngIf="visible">
      <div class="rc-overlay" (click)="onCancel()">
        <div
          class="rc-panel"
          [appTrapFocus]="true"
          role="dialog"
          aria-modal="true"
          aria-label="Connect to remote server"
          tabindex="-1"
          (click)="$event.stopPropagation()"
        >
          <div class="rc-header">
            <h2>Connect to {{ protocolLabel }} Server</h2>
          </div>

          <div class="rc-body">
            <!-- Protocol -->
            <div class="rc-field">
              <label class="rc-label">Protocol</label>
              <select class="rc-select" [(ngModel)]="protocol" (ngModelChange)="onProtocolChange()">
                <option value="ftp">FTP</option>
                <option value="sftp">SFTP</option>
                <option value="smb">SMB</option>
                <option value="webdav">WebDAV</option>
              </select>
            </div>

            <!-- Host + Port -->
            <div class="rc-row">
              <div class="rc-field rc-field--grow">
                <label class="rc-label">Host</label>
                <input class="rc-input" type="text" [(ngModel)]="host" placeholder="hostname or IP" data-dialog-initial-focus />
              </div>
              <div class="rc-field rc-field--port">
                <label class="rc-label">Port</label>
                <input class="rc-input" type="number" [(ngModel)]="port" min="1" max="65535" />
              </div>
            </div>

            <!-- Username + Password -->
            <div class="rc-row">
              <div class="rc-field rc-field--grow">
                <label class="rc-label">Username</label>
                <input class="rc-input" type="text" [(ngModel)]="username" placeholder="anonymous" />
              </div>
              <div class="rc-field rc-field--grow">
                <label class="rc-label">Password</label>
                <input class="rc-input" type="password" [(ngModel)]="password" />
              </div>
            </div>

            <!-- SMB-specific -->
            <ng-container *ngIf="protocol === 'smb'">
              <div class="rc-row">
                <div class="rc-field rc-field--grow">
                  <label class="rc-label">Domain</label>
                  <input class="rc-input" type="text" [(ngModel)]="domain" placeholder="WORKGROUP" />
                </div>
                <div class="rc-field rc-field--grow">
                  <label class="rc-label">Share</label>
                  <input class="rc-input" type="text" [(ngModel)]="share" placeholder="Documents" />
                </div>
              </div>
            </ng-container>

            <!-- WebDAV-specific -->
            <ng-container *ngIf="protocol === 'webdav'">
              <div class="rc-row">
                <div class="rc-field rc-field--grow">
                  <label class="rc-label">Schema</label>
                  <select class="rc-select" [(ngModel)]="schema">
                    <option value="https">HTTPS</option>
                    <option value="http">HTTP</option>
                  </select>
                </div>
                <div class="rc-field rc-field--grow">
                  <label class="rc-label">Base Path</label>
                  <input class="rc-input" type="text" [(ngModel)]="basePath" placeholder="/" />
                </div>
              </div>
            </ng-container>

            <!-- Error message -->
            <div class="rc-error" *ngIf="error">
              <svg class="rc-error-icon" viewBox="0 0 16 16" fill="currentColor">
                <path d="M7.56 1h.88l6.54 12.26-.44.74H1.44L1 13.26Zm.44 2L2.28 13h11.44ZM8 11a.75.75 0 1 1 0 1.5A.75.75 0 0 1 8 11zm.75-5v4.5h-1.5V6h1.5z"/>
              </svg>
              {{ error }}
            </div>
          </div>

          <div class="rc-actions">
            <button class="rc-btn rc-btn--cancel" type="button" (click)="onCancel()" [disabled]="connecting">
              Cancel
            </button>
            <button class="rc-btn rc-btn--connect" type="button" (click)="onConnect()" [disabled]="connecting || !host">
              <span class="rc-spinner" *ngIf="connecting"></span>
              {{ connecting ? 'Connecting…' : 'Connect' }}
            </button>
          </div>
        </div>
      </div>
    </ng-container>
  `,
  styles: [`
    .rc-overlay {
      position: fixed;
      inset: 0;
      z-index: var(--vsc-z-modal);
      display: flex;
      align-items: center;
      justify-content: center;
      background: rgba(0, 0, 0, 0.4);
      padding: var(--vsc-padding-xl);
      animation: rcFadeIn 0.15s ease;
    }

    @keyframes rcFadeIn {
      from { opacity: 0; }
      to { opacity: 1; }
    }

    @keyframes rcSlideIn {
      from { transform: translateY(-8px); opacity: 0; }
      to { transform: translateY(0); opacity: 1; }
    }

    .rc-panel {
      width: min(460px, calc(100vw - 2 * var(--vsc-padding-xl)));
      background: var(--vsc-panel-background);
      color: var(--vsc-foreground);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45);
      overflow: hidden;
      animation: rcSlideIn 0.2s ease;
    }

    .rc-header {
      padding: 14px 16px 10px;
      border-bottom: 1px solid var(--vsc-border-subtle);

      h2 {
        margin: 0;
        font-size: 13px;
        font-weight: 600;
        color: var(--vsc-foreground);
      }
    }

    .rc-body {
      padding: 12px 16px;
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    .rc-row {
      display: flex;
      gap: 10px;
    }

    .rc-field {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    .rc-field--grow { flex: 1; }
    .rc-field--port { width: 80px; flex-shrink: 0; }

    .rc-label {
      font-size: 11px;
      font-weight: 600;
      color: var(--vsc-foreground-dim);
      text-transform: uppercase;
      letter-spacing: 0.3px;
    }

    .rc-input, .rc-select {
      height: 28px;
      padding: 0 8px;
      background: var(--vsc-input-background);
      color: var(--vsc-input-foreground);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm);
      font-size: var(--vsc-font-size);
      font-family: var(--vsc-font-family);
      outline: none;

      &:focus {
        border-color: var(--vsc-accent-blue);
      }
    }

    .rc-select {
      cursor: pointer;
    }

    .rc-error {
      display: flex;
      align-items: flex-start;
      gap: 8px;
      padding: 8px 10px;
      background: rgba(255, 85, 85, 0.08);
      border: 1px solid rgba(255, 85, 85, 0.3);
      border-radius: var(--vsc-border-radius-sm);
      color: #ff5555;
      font-size: 12px;
      line-height: 1.4;
    }

    .rc-error-icon {
      width: 14px;
      height: 14px;
      flex-shrink: 0;
      margin-top: 1px;
    }

    .rc-actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
      padding: 10px 16px 14px;
    }

    .rc-btn {
      height: 28px;
      padding: 0 14px;
      border: none;
      border-radius: var(--vsc-border-radius-sm);
      font-size: var(--vsc-font-size);
      font-family: var(--vsc-font-family);
      cursor: pointer;
      display: flex;
      align-items: center;
      gap: 6px;

      &:disabled {
        opacity: 0.5;
        cursor: not-allowed;
      }
    }

    .rc-btn--cancel {
      background: transparent;
      color: var(--vsc-foreground);
      border: 1px solid var(--vsc-border);

      &:hover:not(:disabled) {
        background: var(--vsc-input-background);
      }
    }

    .rc-btn--connect {
      background: var(--vsc-accent-blue);
      color: #fff;

      &:hover:not(:disabled) {
        filter: brightness(1.1);
      }
    }

    .rc-spinner {
      display: inline-block;
      width: 12px;
      height: 12px;
      border: 2px solid rgba(255, 255, 255, 0.3);
      border-top-color: #fff;
      border-radius: 50%;
      animation: rcSpin 0.7s linear infinite;
    }

    @keyframes rcSpin { to { transform: rotate(360deg); } }
  `],
})
export class RemoteConnectionDialogComponent implements OnDestroy {
  visible = false;

  protocol: RemoteProtocol = 'sftp';
  host = '';
  port = 22;
  username = '';
  password = '';

  // SMB
  domain = '';
  share = '';

  // WebDAV
  schema = 'https';
  basePath = '';

  connecting = false;
  error = '';

  private readonly destroy$ = createSubject<void>();

  get protocolLabel(): string {
    return PROTOCOL_LABELS[this.protocol] || this.protocol.toUpperCase();
  }

  constructor(private remoteService: RemoteConnectionService) {
    this.remoteService.request$.pipe(takeUntil(this.destroy$)).subscribe(req => {
      if (req) {
        this.protocol = req.prefill.protocol;
        this.host = req.prefill.host;
        this.port = req.prefill.port;
        this.username = '';
        this.password = '';
        this.domain = '';
        this.share = '';
        this.schema = 'https';
        this.basePath = '';
        this.error = '';
        this.connecting = false;
        this.visible = true;
      } else {
        this.visible = false;
      }
    });
  }

  ngOnDestroy(): void {
    this.destroy$.next();
  }

  onProtocolChange(): void {
    this.port = DEFAULT_PORTS[this.protocol];
    this.error = '';
  }

  async onConnect(): Promise<void> {
    if (!this.host || this.connecting) return;
    this.connecting = true;
    this.error = '';

    const cfg = new contracts.RemoteConfig({
      protocol: this.protocol,
      host: this.host,
      port: this.port,
      username: this.username || undefined,
      password: this.password || undefined,
      ignoreHostKey: this.protocol === 'sftp' ? true : undefined,
      domain: this.protocol === 'smb' ? this.domain || undefined : undefined,
      share: this.protocol === 'smb' ? this.share || undefined : undefined,
      schema: this.protocol === 'webdav' ? this.schema : undefined,
      basePath: this.protocol === 'webdav' ? this.basePath || undefined : undefined,
    });

    try {
      const mount = await RemoteConnect(cfg);
      this.remoteService.resolve(mount.mountPoint);
    } catch (err: any) {
      this.error = typeof err === 'string' ? err : err?.message || 'Connection failed';
      this.connecting = false;
    }
  }

  onCancel(): void {
    if (this.connecting) return;
    this.remoteService.resolve(null);
  }
}
