import { CommonModule } from '@angular/common';
import { Component, ElementRef, NgZone, ViewChild } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { createSubject, takeUntil, type Stream } from '@epikodelabs/streamix';
import { BrowserOpenURL } from 'wailsjs/runtime';
import { contracts } from '../../../wailsjs/go/models';
import { InputHistoryDirective } from '../directives/input-history.directive';
import { moveFocusWithin } from '../utils/focus-navigation.utils';
import { ApiService } from '../services/api.service';
import { InputContextService } from '../services/input-context.service';
import { InputEngineService } from '../services/input-engine.service';
import { RemoteConnectDialogService } from '../services/remote-connect-dialog.service';

interface RemoteConnectDialogRequest {
  resolve: (value: contracts.RemoteMount | null) => void;
  initialName?: string;
  initialProtocol?: string;
}

type RemoteProtocolId = 's3' | 'gdrive' | 'dropbox' | 'onedrive';
type CloudCredentialMode = 'oauth' | 'token';
type RemoteOAuthStatusId = 'idle' | 'pending' | 'complete' | 'failed';
type S3ProviderPresetId = 'aws' | 'minio' | 'cloudflare-r2' | 'wasabi' | 'backblaze-b2' | 'custom';

interface RemoteProtocolPreset {
  id: RemoteProtocolId;
  label: string;
  description: string;
  aliasLabel: string;
  credentialLabel: string;
  credentialPlaceholder: string;
  oauthClientLabel?: string;
}

interface S3ProviderPreset {
  id: S3ProviderPresetId;
  label: string;
  description: string;
  endpoint: string;
  endpointPlaceholder: string;
  region: string;
  usePathStyle: boolean;
  endpointRequired: boolean;
}

interface RemoteConnectFormModel {
  protocol: RemoteProtocolId;
  provider: S3ProviderPresetId;
  authMode: CloudCredentialMode;
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
  accessToken: string;
  clientId: string;
  tenant: string;
}

const REMOTE_PROTOCOL_PRESETS: readonly RemoteProtocolPreset[] = [
  {
    id: 's3',
    label: 'S3-Compatible',
    description: 'AWS S3, MinIO, R2, Wasabi, Backblaze B2, and other S3-compatible endpoints.',
    aliasLabel: 's3://alias',
    credentialLabel: 'Credentials',
    credentialPlaceholder: 'Access key / secret key or anonymous access',
  },
  {
    id: 'gdrive',
    label: 'Google Drive',
    description: 'Mounts Google Drive through a browser-based OAuth sign-in or a pre-issued bearer token.',
    aliasLabel: 'gdrive://alias',
    credentialLabel: 'Access Token',
    credentialPlaceholder: 'Google OAuth access token',
    oauthClientLabel: 'Google desktop client ID',
  },
  {
    id: 'dropbox',
    label: 'Dropbox',
    description: 'Mounts Dropbox through a browser-based OAuth sign-in or a pre-issued bearer token.',
    aliasLabel: 'dropbox://alias',
    credentialLabel: 'Access Token',
    credentialPlaceholder: 'Dropbox access token',
    oauthClientLabel: 'Dropbox app key',
  },
  {
    id: 'onedrive',
    label: 'OneDrive',
    description: 'Mounts OneDrive through Microsoft browser sign-in or a pre-issued Graph access token.',
    aliasLabel: 'onedrive://alias',
    credentialLabel: 'Access Token',
    credentialPlaceholder: 'Microsoft Graph access token',
    oauthClientLabel: 'Microsoft application client ID',
  },
];

const S3_PROVIDER_PRESETS: readonly S3ProviderPreset[] = [
  {
    id: 'aws',
    label: 'AWS S3',
    description: 'Uses AWS-managed endpoints. Leave endpoint blank unless you need a custom gateway.',
    endpoint: '',
    endpointPlaceholder: 'Optional for AWS S3',
    region: 'us-east-1',
    usePathStyle: false,
    endpointRequired: false,
  },
  {
    id: 'minio',
    label: 'MinIO',
    description: 'Uses an explicit MinIO server endpoint and path-style URLs by default.',
    endpoint: 'https://play.min.io',
    endpointPlaceholder: 'https://play.min.io',
    region: 'us-east-1',
    usePathStyle: true,
    endpointRequired: true,
  },
  {
    id: 'cloudflare-r2',
    label: 'Cloudflare R2',
    description: 'Requires your account-specific endpoint and usually uses the auto region.',
    endpoint: '',
    endpointPlaceholder: 'https://<accountid>.r2.cloudflarestorage.com',
    region: 'auto',
    usePathStyle: false,
    endpointRequired: true,
  },
  {
    id: 'wasabi',
    label: 'Wasabi',
    description: 'Uses a Wasabi S3 endpoint. Adjust region and endpoint if your bucket lives elsewhere.',
    endpoint: 'https://s3.us-east-1.wasabisys.com',
    endpointPlaceholder: 'https://s3.us-east-1.wasabisys.com',
    region: 'us-east-1',
    usePathStyle: false,
    endpointRequired: true,
  },
  {
    id: 'backblaze-b2',
    label: 'Backblaze B2 S3',
    description: 'Uses the Backblaze S3-compatible endpoint for a specific region.',
    endpoint: 'https://s3.us-west-004.backblazeb2.com',
    endpointPlaceholder: 'https://s3.us-west-004.backblazeb2.com',
    region: 'us-west-004',
    usePathStyle: false,
    endpointRequired: true,
  },
  {
    id: 'custom',
    label: 'Custom S3-Compatible',
    description: 'For Ceph, local gateways, and other S3-compatible APIs that need a manual endpoint.',
    endpoint: '',
    endpointPlaceholder: 'https://storage.example.com',
    region: 'us-east-1',
    usePathStyle: false,
    endpointRequired: true,
  },
];

const REMOTE_PROTOCOL_PRESET_MAP = new Map<RemoteProtocolId, RemoteProtocolPreset>(
  REMOTE_PROTOCOL_PRESETS.map((preset) => [preset.id, preset]),
);

const S3_PROVIDER_PRESET_MAP = new Map<S3ProviderPresetId, S3ProviderPreset>(
  S3_PROVIDER_PRESETS.map((preset) => [preset.id, preset]),
);

@Component({
  selector: 'app-remote-connect-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule, InputHistoryDirective],
  styleUrls: ['./tools-dialog-remote-theme.scss', './remote-connect-dialog.component.scss'],
  template: `
    <ng-container *ngIf="request$ | async">
      <div
        class="remote-connect-overlay palette-dialog-backdrop dialog-group"
        [class.remote-connect-overlay--closing]="isClosing"
        [class.dialog-group--closing]="isClosing"
        (click)="onCancel()"
        (animationend)="onAnimationEnd($event)"
      >
        <section
          #dialogPanel
          class="remote-connect-panel palette-dialog-surface dialog-surface"
          role="dialog"
          aria-modal="true"
          aria-label="Connect remote storage"
          tabindex="-1"
          (click)="$event.stopPropagation()"
         (keydown.escape)="onCancel(); $event.preventDefault(); $event.stopPropagation()">
          <header class="remote-connect-header palette-dialog-header">
            <div class="palette-dialog-title">
              <strong>Connect Remote</strong>
              <span class="palette-dialog-subtitle">{{ selectedProtocol.label }}</span>
            </div>
            <div class="remote-connect-header-actions">
                            <span class="dialog-close" aria-label="Close tab" (click)="onCancel()" title="Close" role="button" tabindex="0" (keydown.enter)="onCancel()" (keydown.space)="onCancel(); $event.preventDefault()"> × </span>
            </div>
          </header>
          <p class="remote-connect-intro palette-dialog-footnote">
            Create a session-only mount and open it in the active pane.
          </p>

          <div class="remote-connect-body palette-dialog-body">
            <section class="remote-connect-form-section">
              <div class="section-heading">
                <h3>Connection</h3>
                <span class="section-hint">{{ selectedProtocol.description }}</span>
              </div>

              <div class="remote-grid">
                <label class="field field--full">
                  <span>Storage Type</span>
                  <select [(ngModel)]="form.protocol" (ngModelChange)="onProtocolChange($event)">
                    <option *ngFor="let preset of protocolPresets" [ngValue]="preset.id">{{ preset.label }}</option>
                  </select>
                </label>

                <label *ngIf="isS3Protocol" class="field field--full">
                  <span>Provider</span>
                  <select [(ngModel)]="form.provider" (ngModelChange)="onProviderPresetChange($event)">
                    <option *ngFor="let preset of providerPresets" [ngValue]="preset.id">{{ preset.label }}</option>
                  </select>
                  <small>{{ selectedProvider.description }}</small>
                </label>

                <label class="field">
                  <span>Alias <small>(Shown as {{ selectedProtocol.aliasLabel }})</small></span>
                  <input
                    data-dialog-initial-focus
                    type="text"
                    name="remote-name"
                    [(ngModel)]="form.name"
                    (ngModelChange)="clearError()"
                    placeholder="media-archive"
                    [appInputHistory]="'remote.name'"
                  />
                </label>

                <ng-container *ngIf="isS3Protocol; else cloudFields">
                  <label class="field">
                    <span>Bucket</span>
                    <input
                      type="text"
                      name="remote-bucket"
                      [(ngModel)]="form.bucket"
                      (ngModelChange)="clearError()"
                      placeholder="bucket-name"
                      [appInputHistory]="'remote.bucket'"
                    />
                  </label>

                  <label class="field">
                    <span>Endpoint <small>{{ selectedProvider.endpointRequired ? 'Required for this preset' : 'Optional' }}</small></span>
                    <input
                      type="text"
                      name="remote-endpoint"
                      [(ngModel)]="form.endpoint"
                      (ngModelChange)="clearError()"
                      [placeholder]="selectedProvider.endpointPlaceholder"
                      [appInputHistory]="'remote.endpoint'"
                    />
                  </label>

                  <label class="field">
                    <span>Region</span>
                    <input
                      type="text"
                      name="remote-region"
                      [(ngModel)]="form.region"
                      (ngModelChange)="clearError()"
                      [placeholder]="selectedProvider.region || 'us-east-1'"
                      [appInputHistory]="'remote.region'"
                    />
                  </label>

                  <label class="field">
                    <span>Prefix <small>(Optional subfolder)</small></span>
                    <input
                      type="text"
                      name="remote-prefix"
                      [(ngModel)]="form.prefix"
                      (ngModelChange)="clearError()"
                      placeholder="photos/2025"
                      [appInputHistory]="'remote.prefix'"
                    />
                  </label>

                  <label class="field">
                    <span>Access Key ID</span>
                    <input
                      type="text"
                      name="remote-access-key-id"
                      [(ngModel)]="form.accessKeyId"
                      (ngModelChange)="clearError()"
                      placeholder="minioadmin"
                      [appInputHistory]="'remote.accessKeyId'"
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
                </ng-container>

                <ng-template #cloudFields>
                  <label class="field">
                    <span>Sign-In Method</span>
                    <select [(ngModel)]="form.authMode" (ngModelChange)="onAuthModeChange($event)">
                      <option [ngValue]="'oauth'">Browser Sign-In</option>
                      <option [ngValue]="'token'">Existing Access Token</option>
                    </select>
                  </label>

                  <ng-container *ngIf="form.authMode === 'oauth'; else cloudTokenFields">
                    <label class="field field--full">
                      <span>{{ selectedProtocol.oauthClientLabel || 'Client ID' }}</span>
                      <input
                        type="text"
                        name="remote-client-id"
                        [(ngModel)]="form.clientId"
                        (ngModelChange)="onOAuthInputChange()"
                        placeholder="Paste the provider client ID"
                        [appInputHistory]="'remote.clientId'"
                      />
                      <small>Register a native or desktop app with the provider, then paste its public client ID here.</small>
                    </label>

                    <label *ngIf="form.protocol === 'onedrive'" class="field field--full">
                      <span>Tenant <small>(Optional)</small></span>
                      <input
                        type="text"
                        name="remote-tenant"
                        [(ngModel)]="form.tenant"
                        (ngModelChange)="onOAuthInputChange()"
                        placeholder="common"
                        [appInputHistory]="'remote.tenant'"
                      />
                      <small>Use <code>common</code> for personal and work accounts, or a tenant ID for single-tenant registration.</small>
                    </label>

                    <div class="field field--full">
                      <span>Browser Authorization</span>
                      <div class="oauth-panel">
                        <div class="oauth-panel-copy">
                          <strong>{{ oauthActionTitle }}</strong>
                          <small>Opens your default browser and waits for the provider to redirect back to a temporary localhost callback.</small>
                        </div>
                        <button
                          type="button"
                          class="palette-dialog-button"
                          [disabled]="isAuthorizing"
                          (click)="authorizeInBrowser()"
                        >
                          {{ isAuthorizing ? 'Waiting...' : oauthActionButton }}
                        </button>
                      </div>
                    </div>

                    <p *ngIf="oauthStatus === 'pending'" class="info-banner">
                      Waiting for browser sign-in to complete. The app is listening on {{ oauthRedirectUri || 'localhost' }}.
                    </p>

                    <p *ngIf="oauthStatus === 'complete'" class="success-banner">
                      Authorized{{ oauthAccountLabel ? ' as ' + oauthAccountLabel : '' }}. Select Connect to mount it in this session.
                    </p>
                  </ng-container>

                  <ng-template #cloudTokenFields>
                    <label class="field field--full">
                      <span>{{ selectedProtocol.credentialLabel }}</span>
                      <input
                        type="password"
                        [(ngModel)]="form.accessToken"
                        (ngModelChange)="clearError()"
                        [placeholder]="selectedProtocol.credentialPlaceholder"
                      />
                      <small>Stored in memory only for this app session.</small>
                    </label>
                  </ng-template>
                </ng-template>
              </div>

              <div *ngIf="isS3Protocol" class="toggle-row">
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

              <div class="actions palette-dialog-actions">
                <button type="button" class="palette-dialog-button" (click)="onCancel()">Cancel</button>
                <button
                  type="button"
                  class="palette-dialog-button palette-dialog-button--primary"
                  [disabled]="isConnecting || isAuthorizing"
                  (click)="onSubmit()"
                >
                  {{ isConnecting ? 'Connecting...' : 'Connect' }}
                </button>
              </div>
            </section>

            <aside class="remote-connect-side">
              <div class="section-heading">
                <h3>Active Remote Mounts</h3>
                <button type="button" class="palette-dialog-button remote-connect-utility-button" [disabled]="isRefreshing" (click)="refreshConnections()">
                  {{ isRefreshing ? '...' : 'Refresh' }}
                </button>
              </div>

              <div *ngIf="connections.length === 0" class="empty-state">
                No remote mounts are active in this session.
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
                    <small>{{ formatProtocolLabel(mount.protocol) }}{{ mount.host ? ' - ' + mount.host : '' }}</small>
                  </div>
                  <div class="connection-card-actions">
                    <span *ngIf="mount.readOnly" class="read-only-badge">Read only</span>
                    <button
                      type="button"
                      class="palette-dialog-button remote-connect-utility-button"
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
        </section>
      </div>
    </ng-container>
  `,
})
export class RemoteConnectDialogComponent {
  @ViewChild('dialogPanel') private dialogPanel?: ElementRef<HTMLElement>;

  readonly request$: Stream<RemoteConnectDialogRequest | null> = this.remoteConnectDialog.mountedRequest$;
  readonly protocolPresets = REMOTE_PROTOCOL_PRESETS;
  readonly providerPresets = S3_PROVIDER_PRESETS;

  form: RemoteConnectFormModel = this.createDefaultForm();
  connections: contracts.RemoteMount[] = [];
  errorMessage = '';
  isConnecting = false;
  isRefreshing = false;
  readonly disconnectingMounts = new Set<string>();

  oauthStatus: RemoteOAuthStatusId = 'idle';
  oauthSessionId: string | null = null;
  oauthRedirectUri = '';
  oauthAccountLabel = '';

  private readonly destroy$ = createSubject<void>();
  private isOpen = false;
  private releaseDialogContext: (() => void) | null = null;
  private oauthPollTimer: number | null = null;
  private oauthSessionConnected = false;
  private connectAttemptGeneration = 0;
  private readonly captureEscapeListener = (event: KeyboardEvent) => {
    if (!this.isOpen || event.key !== 'Escape') {
      return;
    }

    event.preventDefault();
    this.ngZone.run(() => this.onCancel());
  };

  constructor(
    private readonly apiService: ApiService,
    private readonly remoteConnectDialog: RemoteConnectDialogService,
    private readonly inputContexts: InputContextService,
    private readonly inputEngine: InputEngineService,
    private readonly ngZone: NgZone,
  ) {}

  ngOnInit(): void {
    window.addEventListener('keydown', this.captureEscapeListener, { capture: true });
    this.inputEngine.registerCommand('dialog.remoteConnect.submit', () => {
      void this.onSubmit();
    });
    this.inputEngine.registerCommand('dialog.remoteConnect.close', () => this.onCancel());
    this.inputEngine.registerCommand('dialog.remoteConnect.focusNext', () => this.moveDialogFocus(1));
    this.inputEngine.registerCommand('dialog.remoteConnect.focusPrev', () => this.moveDialogFocus(-1));

    this.remoteConnectDialog.mountedRequest$
      .pipe(takeUntil(this.destroy$))
      .subscribe((request) => {
        if (request) {
          void this.cancelUnusedOAuthSession();
          this.form = this.createDefaultForm();
          if (request.initialProtocol && this.isRemoteProtocol(request.initialProtocol)) {
            this.form.protocol = request.initialProtocol;
            this.onProtocolChange(request.initialProtocol);
          }
          if (request.initialName) {
            this.form.name = request.initialName;
          }
          this.connections = [];
          this.errorMessage = '';
          this.isConnecting = false;
          this.isOpen = true;
          this.disconnectingMounts.clear();
          this.releaseDialogContext ??= this.inputContexts.acquire('dialog.remoteConnect');
          void this.refreshConnections();
          return;
        }

        if (!this.oauthSessionConnected) {
          void this.cancelUnusedOAuthSession();
        } else {
          this.stopOAuthPolling();
          this.resetOAuthState();
        }
        this.oauthSessionConnected = false;
        this.errorMessage = '';
        this.isConnecting = false;
        this.isOpen = false;
        this.connections = [];
        this.disconnectingMounts.clear();
        this.releaseDialogContext?.();
        this.releaseDialogContext = null;
      });
  }

  ngOnDestroy(): void {
    window.removeEventListener('keydown', this.captureEscapeListener, { capture: true });
    this.releaseDialogContext?.();
    this.releaseDialogContext = null;
    this.inputEngine.unregisterCommand('dialog.remoteConnect.submit');
    this.inputEngine.unregisterCommand('dialog.remoteConnect.close');
    this.inputEngine.unregisterCommand('dialog.remoteConnect.focusNext');
    this.inputEngine.unregisterCommand('dialog.remoteConnect.focusPrev');
    void this.cancelUnusedOAuthSession();
    this.destroy$.next();
    this.destroy$.complete();
  }

  get isS3Protocol(): boolean {
    return this.form.protocol === 's3';
  }

  get isAuthorizing(): boolean {
    return this.oauthStatus === 'pending';
  }

  get selectedProtocol(): RemoteProtocolPreset {
    return REMOTE_PROTOCOL_PRESET_MAP.get(this.form.protocol) ?? REMOTE_PROTOCOL_PRESETS[0];
  }

  get selectedProvider(): S3ProviderPreset {
    return S3_PROVIDER_PRESET_MAP.get(this.form.provider) ?? S3_PROVIDER_PRESETS[0];
  }

  get oauthActionTitle(): string {
    if (this.oauthStatus === 'complete') {
      return 'Authorization ready';
    }
    return 'Browser-based OAuth sign-in';
  }

  get oauthActionButton(): string {
    return this.oauthStatus === 'complete' ? 'Re-authorize' : 'Authorize';
  }

  clearError(): void {
    this.errorMessage = '';
  }

  onProtocolChange(protocol: RemoteProtocolId): void {
    const previousProtocol = this.form.protocol;
    this.form.protocol = protocol;
    if (protocol === 's3') {
      this.onProviderPresetChange(this.form.provider);
    } else if (previousProtocol !== protocol) {
      this.form.authMode = 'oauth';
      this.form.tenant = protocol === 'onedrive' ? 'common' : '';
      void this.cancelUnusedOAuthSession();
    }
    this.clearError();
  }

  onAuthModeChange(mode: CloudCredentialMode): void {
    this.form.authMode = mode;
    if (mode !== 'oauth') {
      void this.cancelUnusedOAuthSession();
    }
    this.clearError();
  }

  onOAuthInputChange(): void {
    if (this.oauthStatus === 'complete' || this.oauthStatus === 'failed') {
      void this.cancelUnusedOAuthSession();
    } else {
      this.clearError();
    }
  }

  onProviderPresetChange(provider: S3ProviderPresetId): void {
    const preset = S3_PROVIDER_PRESET_MAP.get(provider);
    if (!preset) {
      return;
    }

    this.form.provider = provider;
    this.form.endpoint = preset.endpoint;
    this.form.region = preset.region;
    this.form.usePathStyle = preset.usePathStyle;
    this.clearError();
  }

  onCancel(): void {
    if (!this.isOpen || this.isClosing) {
      return;
    }

    // Closing must never be blocked by a slow or hung remote-connect request.
    // Invalidate the current attempt so a late completion cannot update or
    // re-resolve a dialog the user has already dismissed.
    this.connectAttemptGeneration += 1;
    this.remoteConnectDialog.resolveActive(null);
  }

  async authorizeInBrowser(): Promise<void> {
    if (this.isS3Protocol || this.form.authMode !== 'oauth') {
      return;
    }

    const clientId = this.form.clientId.trim();
    if (!clientId) {
      this.errorMessage = `${this.selectedProtocol.oauthClientLabel || 'Client ID'} is required before authorizing in the browser.`;
      return;
    }

    await this.cancelUnusedOAuthSession();
    this.clearError();
    this.oauthStatus = 'pending';

    try {
      const response = await this.apiService.remoteOAuthBegin(new contracts.RemoteOAuthBeginRequest({
        protocol: this.form.protocol,
        clientId,
        tenant: this.form.protocol === 'onedrive' ? this.normalizeOptionalField(this.form.tenant) : undefined,
      }));
      this.oauthSessionId = response.sessionId;
      this.oauthRedirectUri = response.redirectUri;
      this.oauthAccountLabel = '';
      BrowserOpenURL(response.authUrl);
      this.startOAuthPolling(response.sessionId);
    } catch (error) {
      this.oauthStatus = 'failed';
      this.errorMessage = error instanceof Error ? error.message : String(error);
    }
  }

  async onSubmit(): Promise<void> {
    const validationError = this.validateForm();
    if (validationError) {
      this.errorMessage = validationError;
      return;
    }

    const attemptGeneration = ++this.connectAttemptGeneration;
    this.isConnecting = true;
    this.errorMessage = '';

    try {
      let mount: contracts.RemoteMount;
      if (!this.isS3Protocol && this.form.authMode === 'oauth') {
        mount = await this.apiService.remoteConnectOAuth(new contracts.RemoteOAuthConnectRequest({
          sessionId: this.oauthSessionId!,
          name: this.form.name.trim(),
        }));
        this.oauthSessionConnected = true;
      } else {
        mount = await this.apiService.remoteConnect(new contracts.RemoteConfig({
          protocol: this.form.protocol,
          host: '',
          name: this.form.name.trim(),
          endpoint: this.isS3Protocol ? this.normalizeOptionalField(this.form.endpoint) : undefined,
          region: this.isS3Protocol ? (this.normalizeOptionalField(this.form.region) ?? 'us-east-1') : undefined,
          bucket: this.isS3Protocol ? this.form.bucket.trim() : undefined,
          prefix: this.isS3Protocol ? this.normalizeOptionalField(this.form.prefix) : undefined,
          accessKeyId: this.isS3Protocol ? this.normalizeOptionalField(this.form.accessKeyId) : undefined,
          secretAccessKey: this.isS3Protocol ? this.normalizeOptionalField(this.form.secretAccessKey) : undefined,
          sessionToken: this.isS3Protocol ? this.normalizeOptionalField(this.form.sessionToken) : undefined,
          anonymous: this.isS3Protocol ? this.form.anonymous : undefined,
          usePathStyle: this.isS3Protocol ? this.form.usePathStyle : undefined,
          insecure: this.isS3Protocol ? this.form.insecure : undefined,
          accessToken: !this.isS3Protocol ? this.normalizeOptionalField(this.form.accessToken) : undefined,
        }));
      }

      if (attemptGeneration !== this.connectAttemptGeneration || !this.isOpen) {
        // The user dismissed the dialog while the backend was still working.
        // Do not leak a late-created mount into the session.
        try {
          await this.apiService.remoteDisconnect(mount.mountPoint);
        } catch {
          // Best-effort rollback; the dialog must remain dismissed either way.
        }
        return;
      }

      this.remoteConnectDialog.resolveActive(mount);
    } catch (error) {
      if (attemptGeneration === this.connectAttemptGeneration && this.isOpen) {
        this.errorMessage = error instanceof Error ? error.message : String(error);
      }
    } finally {
      if (attemptGeneration === this.connectAttemptGeneration) {
        this.isConnecting = false;
      }
    }
  }

  async refreshConnections(): Promise<void> {
    this.isRefreshing = true;
    this.errorMessage = '';

    try {
      const connections = await this.apiService.remoteListConnections();
      this.connections = connections.sort((left, right) => {
        const byProtocol = left.protocol.localeCompare(right.protocol);
        return byProtocol !== 0 ? byProtocol : left.name.localeCompare(right.name);
      });
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

  formatProtocolLabel(protocol: string): string {
    return REMOTE_PROTOCOL_PRESET_MAP.get(protocol as RemoteProtocolId)?.label ?? protocol.toUpperCase();
  }

  private validateForm(): string | null {
    if (!/[A-Za-z0-9]/.test(this.form.name)) {
      return 'Alias is required and must contain at least one letter or number.';
    }

    if (this.isS3Protocol) {
      if (!this.form.bucket.trim()) {
        return 'Bucket is required.';
      }
      if (this.selectedProvider.endpointRequired && !this.form.endpoint.trim()) {
        return 'Endpoint is required for the selected provider preset.';
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

    if (this.form.authMode === 'oauth') {
      if (!this.form.clientId.trim()) {
        return `${this.selectedProtocol.oauthClientLabel || 'Client ID'} is required.`;
      }
      if (!this.oauthSessionId || this.oauthStatus !== 'complete') {
        return 'Authorize this provider in the browser before connecting.';
      }
      return null;
    }

    if (!this.form.accessToken.trim()) {
      return `${this.selectedProtocol.credentialLabel} is required.`;
    }
    return null;
  }

  private normalizeOptionalField(value: string): string | undefined {
    const trimmed = value.trim();
    return trimmed.length > 0 ? trimmed : undefined;
  }

  private createDefaultForm(): RemoteConnectFormModel {
    return {
      protocol: 's3',
      provider: 'aws',
      authMode: 'oauth',
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
      accessToken: '',
      clientId: '',
      tenant: 'common',
    };
  }

  private moveDialogFocus(delta: number): void {
    if (!this.dialogPanel?.nativeElement) {
      return;
    }
    moveFocusWithin(this.dialogPanel.nativeElement, delta);
  }

  private isRemoteProtocol(value: string): value is RemoteProtocolId {
    return value === 's3' || value === 'gdrive' || value === 'dropbox' || value === 'onedrive';
  }

  private startOAuthPolling(sessionId: string): void {
    this.stopOAuthPolling();
    const tick = async (): Promise<void> => {
      try {
        const status = await this.apiService.remoteOAuthStatus(sessionId);
        this.applyOAuthStatus(status);
        if (status.status === 'pending') {
          this.oauthPollTimer = window.setTimeout(() => void tick(), 1000);
        }
      } catch (error) {
        this.oauthStatus = 'failed';
        this.errorMessage = error instanceof Error ? error.message : String(error);
      }
    };
    void tick();
  }

  private stopOAuthPolling(): void {
    if (this.oauthPollTimer !== null) {
      window.clearTimeout(this.oauthPollTimer);
      this.oauthPollTimer = null;
    }
  }

  private applyOAuthStatus(status: contracts.RemoteOAuthStatus): void {
    this.oauthSessionId = status.sessionId;
    this.oauthStatus = this.normalizeOAuthStatus(status.status);
    if (this.oauthStatus === 'complete') {
      this.oauthAccountLabel = this.formatOAuthAccountLabel(status);
      this.errorMessage = '';
      this.stopOAuthPolling();
      return;
    }
    if (this.oauthStatus === 'failed') {
      this.errorMessage = status.error || 'OAuth authorization failed.';
      this.stopOAuthPolling();
    }
  }

  private formatOAuthAccountLabel(status: contracts.RemoteOAuthStatus): string {
    const parts = [status.accountName?.trim(), status.accountEmail?.trim()].filter((value): value is string => !!value);
    if (parts.length === 0) {
      return '';
    }
    if (parts.length === 1 || parts[0] === parts[1]) {
      return parts[0];
    }
    return `${parts[0]} (${parts[1]})`;
  }

  private normalizeOAuthStatus(value: string): RemoteOAuthStatusId {
    return value === 'pending' || value === 'complete' || value === 'failed' ? value : 'failed';
  }

  private async cancelUnusedOAuthSession(): Promise<void> {
    const sessionId = this.oauthSessionId;
    this.stopOAuthPolling();
    if (sessionId && !this.oauthSessionConnected) {
      try {
        await this.apiService.remoteOAuthCancel(sessionId);
      } catch {
        // Best-effort cleanup only.
      }
    }
    if (!this.oauthSessionConnected) {
      this.resetOAuthState();
    }
  }

  private resetOAuthState(): void {
    this.oauthStatus = 'idle';
    this.oauthSessionId = null;
    this.oauthRedirectUri = '';
    this.oauthAccountLabel = '';
  }

  get isClosing(): boolean {
    return !!this.remoteConnectDialog.closingRequest$.value;
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) {
      return;
    }
    this.remoteConnectDialog.finishClose();
  }
}