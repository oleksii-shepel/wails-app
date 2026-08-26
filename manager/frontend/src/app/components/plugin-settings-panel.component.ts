import { CommonModule } from '@angular/common';
import { ChangeDetectorRef, Component, EventEmitter, OnInit, Output } from '@angular/core';
import { ApiService } from '../services/api.service';
import { PluginDiscoveryService } from '../services/plugin-discovery.service';
import {
  PluginLifecycleCoordinatorService,
  type ManagedPluginRuntimeSnapshot,
} from '../services/plugin-lifecycle-coordinator.service';
import type { PluginBackendPrivilege, PluginPermission } from '../plugin/plugin-manifest';

interface PluginPolicyDraft {
  trusted: boolean;
  grantedPermissions: PluginPermission[];
}

@Component({
  selector: 'app-plugin-settings-panel',
  standalone: true,
  imports: [CommonModule],
  template: `
    <section class="plugins-settings" aria-label="Plugins">
      <div class="plugins-header">
        <div class="plugins-heading">
          <h3 class="section-title">Plugins</h3>
          <p class="section-desc">Manage discovered plugin packages, trust, availability, permissions and runtime health.</p>
        </div>
        <div class="plugins-header-actions">
          <button type="button" class="btn btn-secondary btn-small" [disabled]="refreshing" (click)="refresh()">
            {{ refreshing ? 'Refreshing…' : 'Refresh' }}
          </button>
          <button type="button" class="btn btn-secondary btn-small" [disabled]="openingDirectory" (click)="openPluginDirectory()">
            Open Plugin Folder
          </button>
        </div>
      </div>

      <div *ngIf="error" class="plugins-banner error" role="alert">{{ error }}</div>
      <div *ngIf="pluginDirectory" class="plugins-directory" title="Plugin directory">{{ pluginDirectory }}</div>

      <div *ngIf="!refreshing && snapshots.length === 0" class="plugins-empty">
        <strong>No plugin packages discovered.</strong>
        <span>Place manifest-based packages in the plugin folder, then refresh.</span>
      </div>

      <article *ngFor="let snapshot of snapshots; trackBy: trackPlugin" class="plugin-card" [class.invalid]="snapshot.status === 'invalid'" [class.quarantined]="snapshot.status === 'quarantined'">
        <header class="plugin-card-header">
          <div class="plugin-identity">
            <div class="plugin-title-row">
              <strong>{{ snapshot.plugin.discovered.manifest.name || snapshot.plugin.discovered.manifest.id }}</strong>
              <span class="plugin-version">v{{ snapshot.plugin.discovered.manifest.version || '?' }}</span>
              <span class="plugin-status" [attr.data-status]="snapshot.status">{{ formatStatus(snapshot.status) }}</span>
            </div>
            <div class="plugin-id">{{ snapshot.plugin.discovered.manifest.id }}</div>
            <p *ngIf="snapshot.plugin.discovered.manifest.description" class="plugin-description">
              {{ snapshot.plugin.discovered.manifest.description }}
            </p>
          </div>

          <button
            *ngIf="isTrustedForCurrentVersion(snapshot)"
            type="button"
            class="btn btn-small plugin-availability-action"
            [class.btn-primary]="!availabilityEnabled(snapshot)"
            [class.btn-secondary]="availabilityEnabled(snapshot)"
            [disabled]="isBusy(snapshot)"
            (click)="setEnabled(snapshot, !availabilityEnabled(snapshot))"
          >
            {{ availabilityEnabled(snapshot) ? 'Disable' : 'Enable' }}
          </button>
        </header>

        <div *ngIf="snapshot.reason" class="plugin-reason">{{ snapshot.reason }}</div>

        <div *ngIf="snapshot.plugin.discovered.errors.length" class="plugin-messages error">
          <strong>Manifest errors</strong>
          <ul><li *ngFor="let message of snapshot.plugin.discovered.errors">{{ message }}</li></ul>
        </div>
        <div *ngIf="snapshot.plugin.discovered.warnings.length" class="plugin-messages warning">
          <strong>Warnings</strong>
          <ul><li *ngFor="let message of snapshot.plugin.discovered.warnings">{{ message }}</li></ul>
        </div>

        <div *ngIf="snapshot.plugin.discovered.valid" class="plugin-grid">
          <section class="plugin-block">
            <div class="plugin-block-heading">
              <strong>Privilege</strong>
              <span
                [class.state-good]="pluginPrivilege(snapshot) === 'normal' || processElevated"
                [class.state-warn]="pluginPrivilege(snapshot) === 'elevated' && !processElevated"
              >
                {{ pluginPrivilege(snapshot) }}
              </span>
            </div>
            <p>{{ pluginPrivilegeMode(snapshot) }}</p>
          </section>

          <section class="plugin-block">
            <div class="plugin-block-heading">
              <strong>Trust</strong>
              <span *ngIf="isTrustedForCurrentVersion(snapshot)" class="state-good">Trusted for this version</span>
              <span *ngIf="trustedValue(snapshot) && !isTrustedForCurrentVersion(snapshot)" class="state-warn">Review required after upgrade</span>
              <span *ngIf="!trustedValue(snapshot)" class="state-muted">Not trusted</span>
            </div>
            <p>Trust grants all permissions requested by this exact plugin version.</p>
            <button
              type="button"
              class="btn btn-secondary btn-small"
              [disabled]="isBusy(snapshot)"
              (click)="toggleTrusted(snapshot)"
            >
              {{ isTrustedForCurrentVersion(snapshot) ? 'Revoke Trust' : 'Trust This Version' }}
            </button>
          </section>

          <section class="plugin-block">
            <div class="plugin-block-heading"><strong>Permissions</strong></div>
            <p *ngIf="snapshot.plugin.requestedPermissions.length === 0" class="state-muted">This plugin requests no permissions.</p>
            <label *ngFor="let permission of snapshot.plugin.requestedPermissions" class="permission-row">
              <input
                type="checkbox"
                [checked]="permissionGranted(snapshot, permission)"
                [disabled]="isBusy(snapshot) || isTrustedForCurrentVersion(snapshot)"
                (change)="togglePermission(snapshot, permission, $any($event.target).checked)"
              />
              <code>{{ permission }}</code>
              <span *ngIf="isTrustedForCurrentVersion(snapshot)" class="permission-via-trust">via trust</span>
            </label>
          </section>

          <section class="plugin-block">
            <div class="plugin-block-heading"><strong>Runtime health</strong></div>
            <dl class="health-grid">
              <div><dt>Last activation</dt><dd>{{ formatActivation(snapshot) }}</dd></div>
              <div><dt>Failures</dt><dd>{{ snapshot.plugin.state.consecutiveActivationFailures }}</dd></div>
              <div *ngIf="snapshot.plugin.state.lastActivationError"><dt>Last error</dt><dd class="error-text">{{ snapshot.plugin.state.lastActivationError }}</dd></div>
            </dl>
            <div *ngIf="snapshot.plugin.state.quarantined" class="quarantine-box">
              <strong>Quarantined</strong>
              <span>{{ snapshot.plugin.state.quarantineReason || 'Repeated activation failures.' }}</span>
              <button type="button" class="btn btn-secondary btn-small" [disabled]="isBusy(snapshot)" (click)="clearQuarantine(snapshot)">Clear Quarantine</button>
            </div>
          </section>
        </div>
      </article>
    </section>
  `,
  styles: [`
    :host { display: block; }
    .plugins-settings { display: flex; flex-direction: column; gap: 14px; }
    .plugins-header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
    .plugins-heading { display: grid; gap: 4px; max-width: 62ch; }
    .plugins-header .section-title { margin: 0; }
    .plugins-header .section-desc { margin: 0; opacity: .72; }
    .plugins-header-actions { display: flex; gap: 8px; flex-wrap: wrap; justify-content: flex-end; }
    .plugins-directory { padding: 8px 10px; border: 1px solid color-mix(in srgb, var(--vsc-border-bright) 50%, transparent); border-radius: 8px; background: color-mix(in srgb, var(--vsc-foreground) 3%, transparent); font-family: var(--font-mono, monospace); font-size: 11px; opacity: .78; overflow-wrap: anywhere; }
    .plugins-banner, .plugin-reason, .plugin-messages { border-radius: 5px; padding: 8px 10px; font-size: 12px; }
    .plugins-banner.error, .plugin-messages.error { background: color-mix(in srgb, var(--vsc-error) 12%, transparent); border: 1px solid color-mix(in srgb, var(--vsc-error) 35%, transparent); }
    .plugin-messages.warning { background: color-mix(in srgb, var(--vsc-warning) 10%, transparent); border: 1px solid color-mix(in srgb, var(--vsc-warning) 30%, transparent); }
    .plugin-messages ul { margin: 5px 0 0 18px; padding: 0; }
    .plugins-empty { min-height: 160px; border: 1px dashed color-mix(in srgb, var(--vsc-border-bright) 60%, transparent); border-radius: 10px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 5px; opacity: .78; text-align: center; background: color-mix(in srgb, var(--vsc-foreground) 2%, transparent); }
    .plugin-card { border: 1px solid color-mix(in srgb, var(--vsc-border-bright) 64%, transparent); border-radius: 10px; padding: 16px; background: linear-gradient(180deg, color-mix(in srgb, var(--vsc-foreground) 4%, var(--vsc-editor-background)) 0%, color-mix(in srgb, var(--vsc-foreground) 2%, var(--vsc-editor-background)) 100%); display: flex; flex-direction: column; gap: 12px; box-shadow: 0 12px 28px color-mix(in srgb, var(--vsc-scrim) 12%, transparent); transition: transform var(--vsc-transition-fast), border-color var(--vsc-transition-fast), box-shadow var(--vsc-transition-fast); }
    .plugin-card:hover { transform: translateY(-1px); border-color: color-mix(in srgb, var(--vsc-accent-blue) 38%, var(--vsc-border-subtle)); box-shadow: 0 18px 36px color-mix(in srgb, var(--vsc-scrim) 16%, transparent); }
    .plugin-card.invalid { border-color: color-mix(in srgb, var(--vsc-error) 45%, transparent); }
    .plugin-card.quarantined { border-color: color-mix(in srgb, var(--vsc-warning) 45%, transparent); }
    .plugin-card-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
    .plugin-availability-action { flex: 0 0 auto; min-width: 72px; }
    .plugin-identity { min-width: 0; }
    .plugin-title-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
    .plugin-version, .plugin-id { font-family: var(--font-mono, monospace); font-size: 11px; opacity: .66; }
    .plugin-description { margin: 7px 0 0; font-size: 12px; opacity: .8; }
    .plugin-status { font-size: 10px; text-transform: uppercase; letter-spacing: .04em; padding: 3px 7px; border-radius: 999px; background: var(--vsc-neutral-overlay-soft); }
    .plugin-status[data-status="active"] { background: color-mix(in srgb, var(--vsc-success) 18%, transparent); }
    .plugin-status[data-status="failed"], .plugin-status[data-status="invalid"] { background: color-mix(in srgb, var(--vsc-error) 18%, transparent); }
    .plugin-status[data-status="quarantined"], .plugin-status[data-status="permissions-required"] { background: color-mix(in srgb, var(--vsc-warning) 18%, transparent); }
    .plugin-reason { background: color-mix(in srgb, var(--vsc-foreground) 4%, transparent); border: 1px solid color-mix(in srgb, var(--vsc-border-subtle) 80%, transparent); }
    .plugin-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
    .plugin-block { border: 1px solid color-mix(in srgb, var(--vsc-border-bright) 48%, transparent); border-radius: 8px; padding: 12px; min-width: 0; background: color-mix(in srgb, var(--vsc-foreground) 2%, transparent); }
    .plugin-block p { margin: 7px 0 10px; font-size: 11px; opacity: .72; }
    .plugin-block-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; font-size: 12px; }
    .state-good { color: var(--vsc-success); font-size: 10px; }
    .state-warn { color: var(--vsc-warning); font-size: 10px; }
    .state-muted { opacity: .6; font-size: 11px; }
    .permission-row { display: flex; align-items: center; gap: 7px; min-height: 25px; font-size: 11px; }
    .permission-row code { overflow-wrap: anywhere; }
    .permission-via-trust { margin-left: auto; opacity: .55; font-size: 10px; }
    .health-grid { margin: 7px 0 0; display: flex; flex-direction: column; gap: 6px; font-size: 11px; }
    .health-grid div { display: grid; grid-template-columns: 92px minmax(0, 1fr); gap: 7px; }
    .health-grid dt { opacity: .6; }
    .health-grid dd { margin: 0; overflow-wrap: anywhere; }
    .error-text { color: var(--vsc-error); }
    .quarantine-box { display: flex; flex-direction: column; align-items: flex-start; gap: 7px; margin-top: 10px; padding-top: 9px; border-top: 1px solid var(--border-color, var(--vsc-neutral-overlay-soft)); font-size: 11px; }
    .plugin-card-actions { display: flex; align-items: center; gap: 8px; justify-content: flex-end; }
    .plugin-working { font-size: 11px; opacity: .6; }
    @media (max-width: 900px) { .plugin-grid { grid-template-columns: 1fr; } .plugins-header { flex-direction: column; } }
  `],
})
export class PluginSettingsPanelComponent implements OnInit {
  @Output() readonly dirtyChange = new EventEmitter<boolean>();

  snapshots: ManagedPluginRuntimeSnapshot[] = [];
  pluginDirectory = '';
  processElevated = false;
  error = '';
  refreshing = false;
  openingDirectory = false;
  private readonly busyIds = new Set<string>();
  private readonly baselinePolicy = new Map<string, PluginPolicyDraft>();
  private readonly draftPolicy = new Map<string, PluginPolicyDraft>();
  private dirty = false;

  constructor(
    private readonly coordinator: PluginLifecycleCoordinatorService,
    private readonly discovery: PluginDiscoveryService,
    private readonly api: ApiService,
    private readonly changeDetector: ChangeDetectorRef,
  ) {}

  ngOnInit(): void {
    void this.refresh();
    void this.loadPluginDirectory();
  }

  trackPlugin(_index: number, snapshot: ManagedPluginRuntimeSnapshot): string {
    return snapshot.plugin.discovered.manifest.id || snapshot.plugin.discovered.path;
  }

  isBusy(snapshot: ManagedPluginRuntimeSnapshot): boolean {
    return this.busyIds.has(snapshot.plugin.discovered.manifest.id);
  }

  availabilityEnabled(snapshot: ManagedPluginRuntimeSnapshot): boolean {
    return snapshot.plugin.state.enabled;
  }

  trustedValue(snapshot: ManagedPluginRuntimeSnapshot): boolean {
    return this.policyFor(snapshot).trusted;
  }

  isTrustedForCurrentVersion(snapshot: ManagedPluginRuntimeSnapshot): boolean {
    const policy = this.policyFor(snapshot);
    if (!policy.trusted) return false;

    // A newly staged trust applies to the currently displayed version.
    if (policy.trusted !== snapshot.plugin.state.trusted) return true;

    return snapshot.plugin.state.trustedVersion === snapshot.plugin.discovered.manifest.version;
  }

  permissionGranted(snapshot: ManagedPluginRuntimeSnapshot, permission: PluginPermission): boolean {
    const policy = this.policyFor(snapshot);
    return policy.trusted || policy.grantedPermissions.includes(permission);
  }

  get hasPendingChanges(): boolean {
    return this.dirty;
  }

  async refresh(): Promise<void> {
    if (this.refreshing) return;
    this.refreshing = true;
    this.error = '';
    this.renderNow();
    try {
      const [snapshots, processElevated] = await withTimeout(
        Promise.all([
          this.coordinator.sync(),
          this.api.isProcessElevated(),
        ]),
        10_000,
        'Plugin refresh timed out.',
      );
      this.snapshots = snapshots;
      this.processElevated = processElevated;
      if (!this.dirty) {
        this.capturePolicy(this.snapshots);
      } else {
        this.reconcileDrafts(this.snapshots);
      }
    } catch (error) {
      this.error = errorMessage(error);
    } finally {
      this.refreshing = false;
      this.renderNow();
    }
  }

  async openPluginDirectory(): Promise<void> {
    if (this.openingDirectory) return;
    this.openingDirectory = true;
    this.error = '';
    this.renderNow();
    try {
      const directory = this.pluginDirectory || await this.discovery.getDirectory();
      this.pluginDirectory = directory;
      await this.api.openFile(directory);
    } catch (error) {
      this.error = errorMessage(error);
    } finally {
      this.openingDirectory = false;
      this.renderNow();
    }
  }

  async setEnabled(snapshot: ManagedPluginRuntimeSnapshot, enabled: boolean): Promise<void> {
    const id = snapshot.plugin.discovered.manifest.id;
    await this.runFor(snapshot, () => this.coordinator.setEnabled(id, enabled));
    this.capturePolicy(this.snapshots);
  }

  toggleTrusted(snapshot: ManagedPluginRuntimeSnapshot): void {
    const trusted = !this.trustedValue(snapshot);
    this.updatePolicy(snapshot, policy => ({ ...policy, trusted }));
  }

  togglePermission(snapshot: ManagedPluginRuntimeSnapshot, permission: PluginPermission, granted: boolean): void {
    this.updatePolicy(snapshot, policy => {
      const permissions = new Set(policy.grantedPermissions);
      if (granted) permissions.add(permission); else permissions.delete(permission);
      return { ...policy, grantedPermissions: [...permissions] };
    });
  }

  async save(): Promise<boolean> {
    if (!this.dirty) return true;

    this.error = '';

    try {
      for (const snapshot of this.snapshots) {
        const id = snapshot.plugin.discovered.manifest.id;
        if (!id) continue;

        const baseline = this.baselinePolicy.get(id);
        const draft = this.draftPolicy.get(id);
        if (!baseline || !draft) continue;

        // Trust and permission edits are saved independently from availability.
        // Enable/disable is an immediate action handled only by the button.
        if (draft.trusted !== baseline.trusted) {
          await withTimeout(
            this.coordinator.setTrusted(id, draft.trusted),
            10_000,
            `Saving trust setting timed out for ${id}.`,
          );
        }

        if (!samePermissions(draft.grantedPermissions, baseline.grantedPermissions)) {
          await withTimeout(
            this.coordinator.setGrantedPermissions(id, draft.grantedPermissions),
            10_000,
            `Saving permission settings timed out for ${id}.`,
          );
        }
      }

      this.snapshots = await withTimeout(
        this.coordinator.sync(),
        10_000,
        'Plugin refresh timed out after saving settings.',
      );
      this.capturePolicy(this.snapshots);
      this.renderNow();
      return true;
    } catch (error) {
      this.error = errorMessage(error);
      this.renderNow();
      return false;
    }
  }

  discard(): void {
    this.capturePolicy(this.snapshots);
    this.renderNow();
  }

  async clearQuarantine(snapshot: ManagedPluginRuntimeSnapshot): Promise<void> {
    await this.runFor(snapshot, () => this.coordinator.clearQuarantine(snapshot.plugin.discovered.manifest.id));
  }



  formatStatus(status: string): string {
    return status.replace(/-/g, ' ');
  }

  formatActivation(snapshot: ManagedPluginRuntimeSnapshot): string {
    const state = snapshot.plugin.state;
    if (state.lastActivationStatus === 'never') return 'Never';
    const suffix = state.lastActivatedAt ? ` · ${new Date(state.lastActivatedAt).toLocaleString()}` : '';
    return `${state.lastActivationStatus}${suffix}`;
  }

  pluginPrivilege(snapshot: ManagedPluginRuntimeSnapshot): PluginBackendPrivilege {
    return snapshot.plugin.discovered.manifest.backend?.privilege === 'elevated'
      ? 'elevated'
      : 'normal';
  }

  pluginPrivilegeMode(snapshot: ManagedPluginRuntimeSnapshot): string {
    if (this.pluginPrivilege(snapshot) !== 'elevated') {
      return 'Runs in normal manager mode.';
    }
    return this.processElevated
      ? 'Manager is currently elevated.'
      : 'Requires elevated manager mode.';
  }

  private async loadPluginDirectory(): Promise<void> {
    try {
      this.pluginDirectory = await this.discovery.getDirectory();
    } catch {
      // Discovery refresh will surface actionable binding errors.
    }
  }

  private policyFor(snapshot: ManagedPluginRuntimeSnapshot): PluginPolicyDraft {
    const id = snapshot.plugin.discovered.manifest.id;
    return this.draftPolicy.get(id) ?? this.policyFromSnapshot(snapshot);
  }

  private policyFromSnapshot(snapshot: ManagedPluginRuntimeSnapshot): PluginPolicyDraft {
    return {
      trusted: snapshot.plugin.state.trusted,
      grantedPermissions: [...snapshot.plugin.state.grantedPermissions],
    };
  }

  private updatePolicy(
    snapshot: ManagedPluginRuntimeSnapshot,
    update: (current: PluginPolicyDraft) => PluginPolicyDraft,
  ): void {
    const id = snapshot.plugin.discovered.manifest.id;
    if (!id) return;

    const current = this.policyFor(snapshot);
    this.draftPolicy.set(id, normalizePolicy(update(current)));
    this.updateDirtyState();
    this.renderNow();
  }

  private capturePolicy(snapshots: readonly ManagedPluginRuntimeSnapshot[]): void {
    this.baselinePolicy.clear();
    this.draftPolicy.clear();

    for (const snapshot of snapshots) {
      const id = snapshot.plugin.discovered.manifest.id;
      if (!id) continue;
      const policy = normalizePolicy(this.policyFromSnapshot(snapshot));
      this.baselinePolicy.set(id, clonePolicy(policy));
      this.draftPolicy.set(id, clonePolicy(policy));
    }

    this.setDirty(false);
  }

  private reconcileDrafts(snapshots: readonly ManagedPluginRuntimeSnapshot[]): void {
    const present = new Set<string>();

    for (const snapshot of snapshots) {
      const id = snapshot.plugin.discovered.manifest.id;
      if (!id) continue;
      present.add(id);

      if (!this.baselinePolicy.has(id)) {
        const policy = normalizePolicy(this.policyFromSnapshot(snapshot));
        this.baselinePolicy.set(id, clonePolicy(policy));
        this.draftPolicy.set(id, clonePolicy(policy));
      }
    }

    for (const id of [...this.baselinePolicy.keys()]) {
      if (!present.has(id)) {
        this.baselinePolicy.delete(id);
        this.draftPolicy.delete(id);
      }
    }

    this.updateDirtyState();
  }

  private updateDirtyState(): void {
    let dirty = false;

    for (const [id, baseline] of this.baselinePolicy) {
      const draft = this.draftPolicy.get(id);
      if (!draft || !samePolicy(baseline, draft)) {
        dirty = true;
        break;
      }
    }

    this.setDirty(dirty);
  }

  private setDirty(value: boolean): void {
    if (this.dirty === value) return;
    this.dirty = value;
    this.dirtyChange.emit(value);
  }

  private async runFor(snapshot: ManagedPluginRuntimeSnapshot, action: () => Promise<unknown>): Promise<void> {
    const id = snapshot.plugin.discovered.manifest.id;
    if (!id || this.busyIds.has(id)) return;
    this.busyIds.add(id);
    this.error = '';
    this.renderNow();
    try {
      await withTimeout(action(), 10_000, `Plugin operation timed out for ${id}.`);
      this.snapshots = await withTimeout(
        this.coordinator.sync(),
        10_000,
        `Plugin refresh timed out for ${id}.`,
      );
    } catch (error) {
      this.error = errorMessage(error);
    } finally {
      this.busyIds.delete(id);
      this.renderNow();
    }
  }

  private renderNow(): void {
    // Wails promises can settle outside Angular's render scheduling. Render this
    // settings subtree explicitly so plugin state never depends on a later UI event.
    this.changeDetector.detectChanges();
  }
}


function clonePolicy(policy: PluginPolicyDraft): PluginPolicyDraft {
  return {
    trusted: policy.trusted,
    grantedPermissions: [...policy.grantedPermissions],
  };
}

function normalizePolicy(policy: PluginPolicyDraft): PluginPolicyDraft {
  return {
    trusted: !!policy.trusted,
    grantedPermissions: [...new Set(policy.grantedPermissions)].sort(),
  };
}

function samePermissions(left: readonly PluginPermission[], right: readonly PluginPermission[]): boolean {
  if (left.length !== right.length) return false;
  const a = [...left].sort();
  const b = [...right].sort();
  return a.every((value, index) => value === b[index]);
}

function samePolicy(left: PluginPolicyDraft, right: PluginPolicyDraft): boolean {
  return left.trusted === right.trusted &&
    samePermissions(left.grantedPermissions, right.grantedPermissions);
}


function withTimeout<T>(promise: Promise<T>, timeoutMs: number, message: string): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const timer = window.setTimeout(() => reject(new Error(message)), timeoutMs);
    promise.then(
      value => {
        window.clearTimeout(timer);
        resolve(value);
      },
      error => {
        window.clearTimeout(timer);
        reject(error);
      },
    );
  });
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
