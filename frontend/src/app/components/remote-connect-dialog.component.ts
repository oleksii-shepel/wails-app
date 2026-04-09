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
  usePathStyle: boolean;
  insecure: boolean;
}

@Component({
  selector: 'app-remote-connect-dialog',
  standalone: true,
  imports: [CommonModule, FormsModule, DialogFocusTrapDirective],
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
            <button type="button" class="icon-btn" aria-label="Close" (click)="onCancel()">Close</button>
          </header>

          <div class="remote-connect-body">
            <section class="remote-connect-form-section">
              <div class="section-heading">
                <h3>Connection</h3>
                <span class="section-hint">Blank credentials use the default AWS credential chain.</span>
              </div>

              <div class="remote-grid">
                <label class="field field--full">
                  <span>Alias</span>
                  <input
                    data-dialog-initial-focus
                    type="text"
                    [(ngModel)]="form.name"
                    (ngModelChange)="clearError()"
                    placeholder="media-archive"
                  />
                  <small>Shown as s3://alias</small>
                </label>

                <label class="field field--full">
                  <span>Endpoint</span>
                  <input
                    type="text"
                    [(ngModel)]="form.endpoint"
                    (ngModelChange)="clearError()"
                    placeholder="https://play.min.io"
                  />
                  <small>Optional for AWS S3. Set this for MinIO or any S3-compatible service.</small>
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

                <label class="field field--full">
                  <span>Prefix</span>
                  <input
                    type="text"
                    [(ngModel)]="form.prefix"
                    (ngModelChange)="clearError()"
                    placeholder="photos/2025"
                  />
                  <small>Optional subfolder inside the bucket.</small>
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

                <label class="field field--full">
                  <span>Session Token</span>
                  <input
                    type="password"
                    [(ngModel)]="form.sessionToken"
                    (ngModelChange)="clearError()"
                    placeholder="Optional temporary credential token"
                  />
                </label>
              </div>

              <div class="toggle-row">
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
                <button type="button" class="btn btn-secondary" (click)="onCancel()">Cancel</button>
                <button type="button" class="btn btn-primary" [disabled]="isConnecting" (click)="onSubmit()">
                  {{ isConnecting ? 'Connecting...' : 'Connect Mount' }}
                </button>
              </div>
            </section>

            <aside class="remote-connect-side">
              <div class="section-heading">
                <h3>Active S3 Mounts</h3>
                <button type="button" class="link-btn" [disabled]="isRefreshing" (click)="refreshConnections()">
                  {{ isRefreshing ? 'Refreshing...' : 'Refresh' }}
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
                      class="btn btn-secondary btn-small"
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
  styles: [`
    .remote-connect-overlay {
      position: fixed;
      inset: 0;
      z-index: var(--vsc-z-modal);
      display: flex;
      align-items: center;
      justify-content: center;
      padding: 24px;
      background: rgba(0, 0, 0, 0.48);
    }

    .remote-connect-panel {
      width: min(1040px, calc(100vw - 48px));
      max-height: min(760px, calc(100vh - 48px));
      display: flex;
      flex-direction: column;
      background: var(--vsc-panel-background);
      color: var(--vsc-foreground);
      border: 1px solid var(--vsc-border-bright);
      border-radius: 12px;
      box-shadow: 0 18px 48px rgba(0, 0, 0, 0.42);
      overflow: hidden;
    }

    .remote-connect-header {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 16px;
      padding: 18px 20px;
      border-bottom: 1px solid var(--vsc-border-subtle);
      background:
        linear-gradient(135deg, color-mix(in srgb, var(--vsc-button-background) 18%, transparent), transparent 48%),
        color-mix(in srgb, var(--vsc-panel-background) 92%, var(--vsc-editor-background));
    }

    .remote-connect-header h2 {
      margin: 0;
      font-size: 20px;
      line-height: 1.15;
    }

    .remote-connect-header p {
      margin: 6px 0 0;
      color: var(--vsc-descriptionForeground, var(--vsc-foreground-dim));
      font-size: 12px;
    }

    .remote-connect-body {
      display: grid;
      grid-template-columns: minmax(0, 1.25fr) minmax(280px, 0.75fr);
      min-height: 0;
      overflow: hidden;
    }

    .remote-connect-form-section,
    .remote-connect-side {
      min-height: 0;
      overflow: auto;
      padding: 20px;
    }

    .remote-connect-side {
      border-left: 1px solid var(--vsc-border-subtle);
      background: color-mix(in srgb, var(--vsc-sideBar-background, var(--vsc-editor-background)) 52%, transparent);
    }

    .section-heading {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 12px;
      margin-bottom: 14px;
    }

    .section-heading h3 {
      margin: 0;
      font-size: 13px;
      text-transform: uppercase;
      letter-spacing: 0.08em;
      color: var(--vsc-foreground-dim);
    }

    .section-hint {
      font-size: 11px;
      color: var(--vsc-descriptionForeground, var(--vsc-foreground-dim));
      text-align: right;
    }

    .remote-grid {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 14px;
    }

    .field {
      display: flex;
      flex-direction: column;
      gap: 6px;
    }

    .field--full {
      grid-column: 1 / -1;
    }

    .field span {
      font-size: 12px;
      font-weight: 600;
      color: var(--vsc-foreground-bright, var(--vsc-foreground));
    }

    .field input {
      width: 100%;
      height: 34px;
      padding: 0 10px;
      box-sizing: border-box;
      border-radius: 8px;
      border: 1px solid var(--vsc-input-border);
      background: var(--vsc-input-background);
      color: var(--vsc-input-foreground);
      font: inherit;
      outline: none;
    }

    .field input:focus {
      border-color: var(--vsc-input-focus-border);
      box-shadow: 0 0 0 1px color-mix(in srgb, var(--vsc-input-focus-border) 35%, transparent);
    }

    .field small {
      color: var(--vsc-descriptionForeground, var(--vsc-foreground-dim));
      font-size: 11px;
      line-height: 1.4;
    }

    .toggle-row {
      display: flex;
      flex-wrap: wrap;
      gap: 14px 18px;
      margin-top: 16px;
    }

    .toggle-field {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      font-size: 12px;
      color: var(--vsc-foreground);
    }

    .error-banner {
      margin: 16px 0 0;
      padding: 10px 12px;
      border-radius: 8px;
      border: 1px solid color-mix(in srgb, var(--vsc-errorForeground, #f48771) 55%, transparent);
      background: color-mix(in srgb, var(--vsc-errorForeground, #f48771) 12%, transparent);
      color: var(--vsc-errorForeground, #f48771);
      white-space: pre-line;
    }

    .actions {
      display: flex;
      justify-content: flex-end;
      gap: 10px;
      margin-top: 18px;
    }

    .btn,
    .icon-btn,
    .link-btn {
      font: inherit;
    }

    .btn {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      height: 32px;
      padding: 0 14px;
      border-radius: 8px;
      border: 1px solid transparent;
      cursor: pointer;
      transition: background 0.12s ease, border-color 0.12s ease;
      white-space: nowrap;
    }

    .btn:disabled,
    .icon-btn:disabled,
    .link-btn:disabled {
      opacity: 0.65;
      cursor: default;
    }

    .btn-primary {
      background: var(--vsc-button-background);
      color: var(--vsc-button-foreground, #fff);
    }

    .btn-primary:hover:not(:disabled) {
      background: var(--vsc-button-hover, var(--vsc-button-background));
    }

    .btn-secondary,
    .icon-btn {
      background: var(--vsc-button-secondary);
      color: var(--vsc-foreground);
      border-color: color-mix(in srgb, var(--vsc-border) 70%, transparent);
    }

    .btn-secondary:hover:not(:disabled),
    .icon-btn:hover:not(:disabled) {
      background: var(--vsc-button-secondary-hover);
    }

    .btn-small {
      height: 28px;
      padding: 0 10px;
      font-size: 11px;
    }

    .icon-btn,
    .link-btn {
      padding: 0 10px;
      height: 30px;
      border-radius: 8px;
      border: 1px solid transparent;
      cursor: pointer;
    }

    .link-btn {
      background: transparent;
      color: var(--vsc-textLink-foreground, var(--vsc-button-background));
    }

    .link-btn:hover:not(:disabled) {
      background: color-mix(in srgb, var(--vsc-button-background) 10%, transparent);
    }

    .empty-state {
      padding: 18px 14px;
      border: 1px dashed var(--vsc-border);
      border-radius: 10px;
      color: var(--vsc-descriptionForeground, var(--vsc-foreground-dim));
      font-size: 12px;
      line-height: 1.5;
    }

    .connection-list {
      display: flex;
      flex-direction: column;
      gap: 10px;
    }

    .connection-card {
      display: flex;
      align-items: flex-start;
      justify-content: space-between;
      gap: 12px;
      width: 100%;
      padding: 12px;
      border: 1px solid var(--vsc-border);
      border-radius: 10px;
      background: color-mix(in srgb, var(--vsc-panel-background) 84%, var(--vsc-editor-background));
      color: inherit;
      text-align: left;
      cursor: pointer;
    }

    .connection-card:hover {
      border-color: var(--vsc-input-focus-border);
      background: color-mix(in srgb, var(--vsc-list-hover-background, var(--vsc-button-secondary)) 60%, transparent);
    }

    .connection-card-main {
      display: flex;
      flex-direction: column;
      gap: 4px;
      min-width: 0;
    }

    .connection-card-main strong,
    .connection-card-main span,
    .connection-card-main small {
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .connection-card-main span {
      font-size: 12px;
      color: var(--vsc-foreground-bright, var(--vsc-foreground));
    }

    .connection-card-main small {
      font-size: 11px;
      color: var(--vsc-descriptionForeground, var(--vsc-foreground-dim));
    }

    .connection-card-actions {
      display: flex;
      flex-direction: column;
      align-items: flex-end;
      gap: 8px;
      flex-shrink: 0;
    }

    .read-only-badge {
      padding: 2px 8px;
      border-radius: 999px;
      background: color-mix(in srgb, var(--vsc-button-background) 16%, transparent);
      color: var(--vsc-button-background);
      font-size: 10px;
      font-weight: 700;
      letter-spacing: 0.06em;
      text-transform: uppercase;
    }

    @media (max-width: 900px) {
      .remote-connect-panel {
        width: min(100vw - 24px, 760px);
      }

      .remote-connect-body {
        grid-template-columns: 1fr;
      }

      .remote-connect-side {
        border-left: 0;
        border-top: 1px solid var(--vsc-border-subtle);
      }
    }

    @media (max-width: 640px) {
      .remote-connect-overlay {
        padding: 12px;
      }

      .remote-connect-panel {
        width: calc(100vw - 24px);
        max-height: calc(100vh - 24px);
      }

      .remote-grid {
        grid-template-columns: 1fr;
      }

      .connection-card {
        flex-direction: column;
      }

      .connection-card-actions {
        width: 100%;
        flex-direction: row;
        justify-content: space-between;
        align-items: center;
      }

      .remote-connect-header {
        padding: 16px;
      }

      .remote-connect-form-section,
      .remote-connect-side {
        padding: 16px;
      }
    }
  `],
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