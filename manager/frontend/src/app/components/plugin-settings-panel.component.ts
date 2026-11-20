import { CommonModule } from '@angular/common';
import { ChangeDetectorRef, Component, EventEmitter, OnInit, Output } from '@angular/core';
import { FormControl, FormGroup, ReactiveFormsModule, Validators } from '@angular/forms';
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

interface HostCommandOption {
  readonly id: string;
  readonly label: string;
}

const HOST_COMMAND_OPTIONS: readonly HostCommandOption[] = [
  { id: 'pane.nav.refresh', label: 'Refresh Active Pane (pane.nav.refresh)' },
  { id: 'pane.compare.toggle', label: 'Compare Panes (pane.compare.toggle)' },
  { id: 'pane.tools.diskUsage', label: 'Analyze Disk Usage (pane.tools.diskUsage)' },
  { id: 'pane.tools.findDuplicates', label: 'Find Duplicates (pane.tools.findDuplicates)' },
  { id: 'pane.files.batchRename', label: 'Batch Rename (pane.files.batchRename)' },
  { id: 'pane.files.createArchive', label: 'Create ZIP Archive (pane.files.createArchive)' },
  { id: 'pane.nav.openCommandPalette', label: 'Open Command Palette (pane.nav.openCommandPalette)' },
  { id: 'reassembly.navigation.trash', label: 'Open Recycle Bin (reassembly.navigation.trash)' },
];

@Component({
  selector: 'app-plugin-settings-panel',
  standalone: true,
  imports: [CommonModule, ReactiveFormsModule],
  template: `
    <section class="plugins-settings" aria-label="Plugins">
      <div class="plugins-header">
        <div class="plugins-heading">
          <h3 class="section-title">Plugins</h3>
          <p class="section-desc">Manage discovered plugin packages, trust, availability, permissions and runtime health.</p>
        </div>
        <div class="plugins-header-actions">
          <button type="button" class="btn btn-primary btn-small" (click)="toggleStudio()">
            <svg class="btn-icon" width="14" height="14" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
              @if (studioOpen) {
                <path d="M4.646 4.646a.5.5 0 0 1 .708 0L8 7.293l2.646-2.647a.5.5 0 0 1 .708.708L8.707 8l2.647 2.646a.5.5 0 0 1-.708.708L8 8.707l-2.646 2.647a.5.5 0 0 1-.708-.708L7.293 8 4.646 5.354a.5.5 0 0 1 0-.708z"/>
              } @else {
                <path d="M8 2a.5.5 0 0 1 .5.5v5h5a.5.5 0 0 1 0 1h-5v5a.5.5 0 0 1-1 0v-5h-5a.5.5 0 0 1 0-1h5v-5A.5.5 0 0 1 8 2z"/>
              }
            </svg>
            {{ studioOpen ? 'Close Builder' : 'New Plugin' }}
          </button>
          <button type="button" class="btn btn-secondary btn-small" [disabled]="refreshing" (click)="refresh()">
            {{ refreshing ? 'Refreshing…' : 'Refresh' }}
          </button>
          <button type="button" class="btn btn-secondary btn-small" [disabled]="openingDirectory" (click)="openPluginDirectory()">
            Open Plugin Folder
          </button>
        </div>
      </div>

      @if (studioOpen) {
        <form class="plugin-studio" [formGroup]="studioForm" (ngSubmit)="createCustomPlugin()" aria-label="Create declarative plugin">
          <div class="plugin-studio-header">
            <div>
              <strong>New Declarative Extension</strong>
              <p>Generates a sandboxed data-only package (<code>plugin.json</code> + <code>frontend.json</code>) stored in the plugin directory.</p>
            </div>
          </div>

          <div class="plugin-studio-grid">
            <label class="studio-field">
              <span>Plugin ID</span>
              <input type="text" formControlName="id" placeholder="custom.workspace-shortcuts" />
            </label>
            <label class="studio-field">
              <span>Display Name</span>
              <input type="text" formControlName="name" placeholder="Workspace Shortcuts" />
            </label>
            <label class="studio-field studio-span-2">
              <span>Description</span>
              <input type="text" formControlName="description" placeholder="Adds custom command palette, toolbar, and context menu actions" />
            </label>
            <label class="studio-field">
              <span>Command Title</span>
              <input type="text" formControlName="commandTitle" placeholder="Quick Compare Panes" />
            </label>
            <label class="studio-field">
              <span>Target Host Command</span>
              <select formControlName="targetCommandId">
                @for (opt of hostCommandOptions; track opt.id) {
                  <option [value]="opt.id">{{ opt.label }}</option>
                }
              </select>
            </label>
            <label class="studio-field">
              <span>Optional Shortcut (Ctrl+Alt+Key)</span>
              <input type="text" formControlName="shortcutKey" maxlength="1" placeholder="k" />
            </label>
            <div class="studio-checks">
              <label class="permission-row">
                <input type="checkbox" formControlName="addToToolbar" />
                <span>Expose in Pane Toolbar</span>
              </label>
              <label class="permission-row">
                <input type="checkbox" formControlName="addToContextMenu" />
                <span>Expose in File Context Menu</span>
              </label>
              <label class="permission-row">
                <input type="checkbox" formControlName="activateNow" />
                <span>Trust &amp; Activate Immediately</span>
              </label>
            </div>
          </div>

          @if (studioError) {
            <div class="plugins-banner error" role="alert">{{ studioError }}</div>
          }

          <div class="plugin-studio-actions">
            <button type="button" class="btn btn-secondary btn-small" (click)="toggleStudio()">Cancel</button>
            <button type="submit" class="btn btn-primary btn-small" [disabled]="creatingPlugin || studioForm.invalid">
              {{ creatingPlugin ? 'Creating…' : 'Create Extension' }}
            </button>
          </div>
        </form>
      }

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

          <div class="plugin-card-header-actions">
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
            <button
              type="button"
              class="btn btn-secondary btn-small"
              [disabled]="isBusy(snapshot)"
              (click)="removePlugin(snapshot)"
              title="Delete plugin package"
            >
              Remove
            </button>
          </div>
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
    .plugin-card-header-actions { display: flex; align-items: center; gap: 8px; flex: 0 0 auto; }
    .plugin-working { font-size: 11px; opacity: .6; }
    .btn-icon { font-size: 15px; width: 15px; height: 15px; line-height: 15px; margin-right: 4px; vertical-align: text-bottom; }
    .plugin-studio { border: 1px solid color-mix(in srgb, var(--vsc-accent-blue) 42%, var(--vsc-border-subtle)); border-radius: 10px; padding: 14px; background: color-mix(in srgb, var(--vsc-foreground) 3%, var(--vsc-editor-background)); display: flex; flex-direction: column; gap: 12px; }
    .plugin-studio-header p { margin: 4px 0 0; font-size: 11px; opacity: .75; }
    .plugin-studio-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
    .studio-span-2 { grid-column: span 2; }
    .studio-field { display: flex; flex-direction: column; gap: 4px; font-size: 11px; }
    .studio-field span { opacity: .8; font-weight: 500; }
    .studio-field input, .studio-field select { background: var(--vsc-input-background); color: var(--vsc-input-foreground); border: 1px solid var(--vsc-border); border-radius: 4px; padding: 6px 8px; font: inherit; }
    .studio-checks { display: flex; flex-direction: column; justify-content: center; gap: 4px; }
    .plugin-studio-actions { display: flex; justify-content: flex-end; gap: 8px; }
    @media (max-width: 900px) { .plugin-grid, .plugin-studio-grid { grid-template-columns: 1fr; } .studio-span-2 { grid-column: span 1; } .plugins-header { flex-direction: column; } }
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
  studioOpen = false;
  creatingPlugin = false;
  studioError = '';
  readonly hostCommandOptions = HOST_COMMAND_OPTIONS;
  readonly studioForm = new FormGroup({
    id: new FormControl('custom.quick-actions', { nonNullable: true, validators: [Validators.required] }),
    name: new FormControl('Custom Quick Actions', { nonNullable: true, validators: [Validators.required] }),
    description: new FormControl('Adds quick pane actions via declarative plugin contributions.', { nonNullable: true }),
    commandTitle: new FormControl('Quick Compare Panes', { nonNullable: true, validators: [Validators.required] }),
    targetCommandId: new FormControl('pane.compare.toggle', { nonNullable: true, validators: [Validators.required] }),
    shortcutKey: new FormControl('', { nonNullable: true }),
    addToToolbar: new FormControl(true, { nonNullable: true }),
    addToContextMenu: new FormControl(true, { nonNullable: true }),
    activateNow: new FormControl(true, { nonNullable: true }),
  });
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

  toggleStudio(): void {
    this.studioOpen = !this.studioOpen;
    this.studioError = '';
    this.renderNow();
  }

  async createCustomPlugin(): Promise<void> {
    if (this.creatingPlugin || this.studioForm.invalid) return;
    this.creatingPlugin = true;
    this.studioError = '';
    this.renderNow();
    try {
      const value = this.studioForm.getRawValue();
      const commandId = value.commandTitle
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-+|-+$/g, '') || 'custom-action';

      const res = await fetch('/api/plugins/create', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          id: value.id.trim().toLowerCase(),
          name: value.name.trim(),
          description: value.description.trim(),
          autoTrust: value.activateNow,
          autoEnable: value.activateNow,
          commands: [{
            id: commandId,
            title: value.commandTitle.trim(),
            targetCommandId: value.targetCommandId,
            shortcutKey: value.shortcutKey.trim(),
            addToToolbar: value.addToToolbar,
            addToContextMenu: value.addToContextMenu,
          }],
        }),
      });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) {
        throw new Error(data?.error || `Failed to create plugin (HTTP ${res.status})`);
      }

      if (value.activateNow) {
        await this.coordinator.activate(value.id.trim().toLowerCase());
      } else {
        await this.coordinator.sync();
      }
      this.studioOpen = false;
      await this.refresh();
    } catch (error) {
      this.studioError = errorMessage(error);
    } finally {
      this.creatingPlugin = false;
      this.renderNow();
    }
  }

  async removePlugin(snapshot: ManagedPluginRuntimeSnapshot): Promise<void> {
    const id = snapshot.plugin.discovered.manifest.id;
    if (!id) return;
    await this.runFor(snapshot, async () => {
      await this.coordinator.deactivate(id).catch(() => undefined);
      const res = await fetch(`/api/plugins/${encodeURIComponent(id)}`, { method: 'DELETE' });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data?.error || `Failed to remove plugin (HTTP ${res.status})`);
      }
    });
    this.capturePolicy(this.snapshots);
  }

  async setEnabled(snapshot: ManagedPluginRuntimeSnapshot, enabled: boolean): Promise<void> {
    const id = snapshot.plugin.discovered.manifest.id;
    await this.runFor(snapshot, async () => {
      if (!enabled) {
        await this.coordinator.deactivate(id).catch(() => undefined);
        await this.coordinator.setEnabled(id, false);
        return;
      }
      await this.coordinator.setEnabled(id, true);
      await this.coordinator.activate(id);
    });
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
