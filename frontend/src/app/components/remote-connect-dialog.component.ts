import { CommonModule } from '@angular/common';
import { Component, ElementRef, ViewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil, type Stream } from '@epikodelabs/streamix';
import { contracts } from '../../../wailsjs/go/models';
import { DialogFocusTrapDirective } from '../directives/dialog-focus-trap.directive';
import { moveFocusWithin } from '../directives/focus-trap.utils';
import { ApiService } from '../services/api.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { NativeOverlayFreezeService } from '../services/native-overlay-freeze.service';
import { RemoteConnectDialogService } from '../services/remote-connect-dialog.service';

interface RemoteConnectDialogRequest {
  resolve: (value: contracts.RemoteMount | null) => void;
  initialName?: string;
}

interface RemoteConnectFormModel {
  name: string;
  endpoint: string;
  region: string;
  bucket: string;
  prefix: string;
  accessKeyId: string;
  secretAccessKey: string;
  sessionToken: string;
  anonymous: boolean;
  usePathStyle: boolean;
  insecure: boolean;
}

@Component({
  selector: 'app-remote-connect-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogFocusTrapDirective],
  styleUrls: ['./remote-connect-dialog.component.scss'],
  template: `
    <ng-container *ngIf="request$ | async">
      <div class="remote-connect-overlay" (click)="onCancel()">
        <div
          #dialogPanel
          class="remote-connect-panel"
          [appTrapFocus]="true"
          role="dialog"
          aria-modal="true"
          aria-label="Connect S3-compatible storage"
          tabindex="-1"
          (click)="$event.stopPropagation()"
        >
          <header class="remote-connect-header">
            <div>
              <h2>S3 / MinIO Mount</h2>
              <p>Create a session-only mount and open it in the active pane.</p>
            </div>
            <button type="button" class="dialog-close-btn" aria-label="Close" (click)="onCancel()" title="Close">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path d="M8 7.293l3.536-3.536.707.707L8.707 8l3.536 3.536-.707.707L8 8.707l-3.536 3.536-.707-.707L7.293 8 3.757 4.464l.707-.707L8 7.293z"/></svg>
            </button>
          </header>

          <div class="remote-connect-body">
            <section class="remote-connect-form-section">
              <div class="section-heading">
                <h3>Connection</h3>
                <span class="section-hint">Use anonymous mode for public buckets, or leave credentials blank to use the default AWS credential chain.</span>
              </div>

              <div class="remote-grid">
                <label class="field field--full">
                  <span>Alias <small>(Shown as s3://alias)</small></span>
                  <input
                    data-dialog-initial-focus
                    type="text"
                    [(ngModel)]="form.name"
                    (ngModelChange)="clearError()"
                    placeholder="media-archive"
                  />
                </label>

                <label class="field">
                  <span>Endpoint <small>(Optional for AWS S3)</small></span>
                  <input
                    type="text"
                    [(ngModel)]="form.endpoint"
                    (ngModelChange)="clearError()"
                    placeholder="https://play.min.io"
                  />
                </label>

                <label class="field">
                  <span>Region</span>
                  <input
                    type="text"
                    [(ngModel)]="form.region"
                    (ngModelChange)="clearError()"
                    placeholder="us-east-1"
                  />
                </label>

                <label class="field">
                  <span>Bucket</span>
                  <input
                    type="text"
                    [(ngModel)]="form.bucket"
                    (ngModelChange)="clearError()"
                    placeholder="bucket-name"
                  />
                </label>

                <label class="field">
                  <span>Prefix <small>(Optional subfolder)</small></span>
                  <input
                    type="text"
                    [(ngModel)]="form.prefix"
                    (ngModelChange)="clearError()"
                    placeholder="photos/2025"
                  />
                </label>

                <label class="field">
                  <span>Access Key ID</span>
                  <input
                    type="text"
                    [(ngModel)]="form.accessKeyId"
                    (ngModelChange)="clearError()"
                    placeholder="minioadmin"
                  />
                </label>

                <label class="field">
                  <span>Secret Access Key</span>
                  <input
                    type="password"
                    [(ngModel)]="form.secretAccessKey"
                    (ngModelChange)="clearError()"
                    placeholder="Secret key"
                  />
                </label>

                <label class="field">
                  <span>Session Token</span>
                  <input
                    type="password"
                    [(ngModel)]="form.sessionToken"
                    (ngModelChange)="clearError()"
                    placeholder="Optional token"
                  />
                </label>
              </div>

              <div class="toggle-row">
                <label class="toggle-field">
                  <input type="checkbox" [(ngModel)]="form.anonymous" (ngModelChange)="clearError()" />
                  <span>Anonymous public bucket</span>
                </label>
                <label class="toggle-field">
                  <input type="checkbox" [(ngModel)]="form.usePathStyle" (ngModelChange)="clearError()" />
                  <span>Force path-style URLs</span>
                </label>
                <label class="toggle-field">
                  <input type="checkbox" [(ngModel)]="form.insecure" (ngModelChange)="clearError()" />
                  <span>Allow insecure TLS</span>
                </label>
              </div>

              <p *ngIf="errorMessage" class="error-banner">{{ errorMessage }}</p>

              <div class="actions">
                <button type="button" class="btn secondary" (click)="onCancel()">Cancel</button>
                <button type="button" class="btn" [disabled]="isConnecting" (click)="onSubmit()">
                  {{ isConnecting ? 'Connecting...' : 'Connect' }}
                </button>
              </div>
            </section>

            <aside class="remote-connect-side">
              <div class="section-heading">
                <h3>Active S3 Mounts</h3>
                <button type="button" class="btn secondary" style="min-height: 24px; padding: 2px 8px; font-size: 11px;" [disabled]="isRefreshing" (click)="refreshConnections()">
                  {{ isRefreshing ? '...' : 'Refresh' }}
                </button>
              </div>

              <div *ngIf="connections.length === 0" class="empty-state">
                No S3-compatible mounts are active in this session.
              </div>

              <div *ngIf="connections.length > 0" class="connection-list">
                <div
                  *ngFor="let mount of connections"
                  class="connection-card"
                  role="button"
                  tabindex="0"
                  (click)="openMount(mount)"
                  (keydown.enter)="openMount(mount)"
                  (keydown.space)="openMount(mount); $event.preventDefault()"
                >
                  <div class="connection-card-main">
                    <strong>{{ mount.name }}</strong>
                    <span>{{ mount.publicPath }}</span>
                    <small>{{ mount.host || 'custom endpoint' }}</small>
                  </div>
                  <div class="connection-card-actions">
                    <span *ngIf="mount.readOnly" class="read-only-badge">Read only</span>
                    <button
                      type="button"
                      class="btn secondary"
                      style="min-height: 24px; padding: 2px 8px; font-size: 11px;"
                      [disabled]="disconnectingMounts.has(mount.mountPoint)"
                      (click)="disconnectMount(mount, $event)"
                    >
                      {{ disconnectingMounts.has(mount.mountPoint) ? 'Disconnecting...' : 'Disconnect' }}
                    </button>
                  </div>
                </div>
              </div>
            </aside>
          </div>
        </div>
      </div>
    </ng-container>
  `,
})
export class RemoteConnectDialogComponent {
  @ViewChild('dialogPanel') private dialogPanel?: ElementRef<HTMLElement>;

  readonly request$: Stream<RemoteConnectDialogRequest | null> = this.remoteConnectDialog.activeRequest$;

  form: RemoteConnectFormModel = this.createDefaultForm();
  connections: contracts.RemoteMount[] = [];
  errorMessage = '';
  isConnecting = false;
  isRefreshing = false;
  readonly disconnectingMounts = new Set<string>();

  private readonly destroy$ = createSubject<void>();
  private releaseDialogContext: (() => void) | null = null;

  constructor(
    private readonly apiService: ApiService,
    private readonly remoteConnectDialog: RemoteConnectDialogService,
    private readonly inputContexts: InputContextService,
    private readonly inputEngine: InputEngineService,
    private readonly overlayFreeze: NativeOverlayFreezeService,
  ) {}

  ngOnInit(): void {
    this.inputEngine.registerCommand('dialog.remoteConnect.submit', () => {
      void this.onSubmit();
    });
    this.inputEngine.registerCommand('dialog.remoteConnect.close', () => this.onCancel());
    this.inputEngine.registerCommand('dialog.remoteConnect.focusNext', () => this.moveDialogFocus(1));
    this.inputEngine.registerCommand('dialog.remoteConnect.focusPrev', () => this.moveDialogFocus(-1));

    this.remoteConnectDialog.activeRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe((request) => {
        if (request) {
          this.form = this.createDefaultForm();
          if (request.initialName) {
            this.form.name = request.initialName;
          }
          this.connections = [];
          this.errorMessage = '';
          this.isConnecting = false;
          this.disconnectingMounts.clear();
          this.releaseDialogContext ??= this.inputContexts.acquire('dialog.remoteConnect');
          this.overlayFreeze.acquire('dialog.remoteConnect');
          void this.refreshConnections();
          return;
        }

        this.errorMessage = '';
        this.isConnecting = false;
        this.connections = [];
        this.disconnectingMounts.clear();
        this.releaseDialogContext?.();
        this.releaseDialogContext = null;
        this.overlayFreeze.release('dialog.remoteConnect');
      });
  }

  ngOnDestroy(): void {
    this.overlayFreeze.release('dialog.remoteConnect');
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.inputEngine.unregisterCommand('dialog.remoteConnect.submit');
    this.inputEngine.unregisterCommand('dialog.remoteConnect.close');
    this.inputEngine.unregisterCommand('dialog.remoteConnect.focusNext');
    this.inputEngine.unregisterCommand('dialog.remoteConnect.focusPrev');
    this.destroy$.next();
    this.destroy$.complete();
  }

  clearError(): void {
    this.errorMessage = '';
  }

  onCancel(): void {
    if (this.isConnecting) {
      return;
    }
    this.remoteConnectDialog.resolveActive(null);
  }

  async onSubmit(): Promise<void> {
    const validationError = this.validateForm();
    if (validationError) {
      this.errorMessage = validationError;
      return;
    }

    this.isConnecting = true;
    this.errorMessage = '';

    try {
      const mount = await this.apiService.remoteConnect(new contracts.RemoteConfig({
        protocol: 's3',
        host: '',
        name: this.form.name.trim(),
        endpoint: this.normalizeOptionalField(this.form.endpoint),
        region: this.normalizeOptionalField(this.form.region) ?? 'us-east-1',
        bucket: this.form.bucket.trim(),
        prefix: this.normalizeOptionalField(this.form.prefix),
        accessKeyId: this.normalizeOptionalField(this.form.accessKeyId),
        secretAccessKey: this.normalizeOptionalField(this.form.secretAccessKey),
        sessionToken: this.normalizeOptionalField(this.form.sessionToken),
        anonymous: this.form.anonymous,
        usePathStyle: this.form.usePathStyle,
        insecure: this.form.insecure,
      }));
      this.remoteConnectDialog.resolveActive(mount);
    } catch (error) {
      this.errorMessage = error instanceof Error ? error.message : String(error);
    } finally {
      this.isConnecting = false;
    }
  }

  async refreshConnections(): Promise<void> {
    this.isRefreshing = true;
    this.errorMessage = '';

    try {
      const connections = await this.apiService.remoteListConnections();
      this.connections = connections
        .filter((mount) => mount.protocol === 's3')
        .sort((left, right) => left.name.localeCompare(right.name));
    } catch (error) {
      this.errorMessage = error instanceof Error ? error.message : String(error);
    } finally {
      this.isRefreshing = false;
    }
  }

  openMount(mount: contracts.RemoteMount): void {
    this.remoteConnectDialog.resolveActive(mount);
  }

  async disconnectMount(mount: contracts.RemoteMount, event: Event): Promise<void> {
    event.stopPropagation();
    this.disconnectingMounts.add(mount.mountPoint);
    this.errorMessage = '';

    try {
      await this.apiService.remoteDisconnect(mount.mountPoint);
      await this.refreshConnections();
    } catch (error) {
      this.errorMessage = error instanceof Error ? error.message : String(error);
    } finally {
      this.disconnectingMounts.delete(mount.mountPoint);
    }
  }

  private validateForm(): string | null {
    if (!/[A-Za-z0-9]/.test(this.form.name)) {
      return 'Alias is required and must contain at least one letter or number.';
    }
    if (!this.form.bucket.trim()) {
      return 'Bucket is required.';
    }

    const hasAccessKey = this.form.accessKeyId.trim().length > 0;
    const hasSecretKey = this.form.secretAccessKey.trim().length > 0;
    if (hasAccessKey !== hasSecretKey) {
      return 'Access key ID and secret access key must be provided together.';
    }
    if (this.form.anonymous && (hasAccessKey || hasSecretKey || this.form.sessionToken.trim().length > 0)) {
      return 'Anonymous mode cannot be combined with access keys or a session token.';
    }

    return null;
  }

  private normalizeOptionalField(value: string): string | undefined {
    const trimmed = value.trim();
    return trimmed.length > 0 ? trimmed : undefined;
  }

  private createDefaultForm(): RemoteConnectFormModel {
    return {
      name: '',
      endpoint: '',
      region: 'us-east-1',
      bucket: '',
      prefix: '',
      accessKeyId: '',
      secretAccessKey: '',
      sessionToken: '',
      anonymous: false,
      usePathStyle: false,
      insecure: false,
    };
  }

  private moveDialogFocus(delta: number): void {
    if (!this.dialogPanel?.nativeElement) {
      return;
    }
    moveFocusWithin(this.dialogPanel.nativeElement, delta);
  }
}