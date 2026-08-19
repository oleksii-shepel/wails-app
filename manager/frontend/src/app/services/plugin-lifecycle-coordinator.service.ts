import { Injectable } from '@angular/core';
import type { PluginActivationEvent, PluginPermission } from '../plugin/plugin-manifest';
import type { PluginPackageSource } from '../plugin/plugin-package';
import {
  PluginInstallationStateService,
  type ManagedPluginPackage,
  type PluginPackageState,
} from './plugin-installation-state.service';
import { DirectoryPluginPackageLoaderService } from './directory-plugin-package-loader.service';
import {
  PluginPlatformService,
  type PluginRuntimeSnapshot,
} from './plugin-platform.service';

const POLICY_SOURCE_OWNER = 'plugin-installation-state';

export type ManagedPluginRuntimeStatus =
  | 'invalid'
  | 'disabled'
  | 'quarantined'
  | 'permissions-required'
  | 'loader-unavailable'
  | 'installed'
  | 'active'
  | 'failed';

export interface ManagedPluginRuntimeSnapshot {
  plugin: ManagedPluginPackage;
  status: ManagedPluginRuntimeStatus;
  runtime?: PluginRuntimeSnapshot;
  reason?: string;
}

export interface PluginActivationEventResult {
  pluginId: string;
  activated: boolean;
  error?: string;
}

@Injectable({ providedIn: 'root' })
export class PluginLifecycleCoordinatorService {
  private managed = new Map<string, ManagedPluginPackage>();

  constructor(
    private readonly installation: PluginInstallationStateService,
    private readonly platform: PluginPlatformService,
    directoryLoader?: DirectoryPluginPackageLoaderService,
  ) {
    if (directoryLoader && !this.platform.hasLoader(directoryLoader.kind)) {
      this.platform.registerLoader(directoryLoader);
    }
  }

  /**
   * Reconciles disk discovery + persisted user policy with the executable
   * plugin platform. This method never broadens permissions and never activates
   * packages by itself.
   */
  async sync(): Promise<ManagedPluginRuntimeSnapshot[]> {
    const packages = await this.installation.refresh();
    this.managed = new Map(packages.map(pkg => [pkg.discovered.manifest.id, pkg]));

    await this.removeIneligibleManagedRuntimes();

    const snapshots: ManagedPluginRuntimeSnapshot[] = [];
    for (const pkg of packages) {
      snapshots.push(await this.reconcilePackage(pkg));
    }
    return snapshots;
  }

  async activate(pluginId: string): Promise<void> {
    const synced = await this.sync();
    const managed = this.mustGetManaged(pluginId);
    const status = synced.find(item => item.plugin.discovered.manifest.id === pluginId);
    if (!managed.readyToActivate) {
      throw new Error(status?.reason ?? `Plugin ${pluginId} is not eligible for activation.`);
    }
    if (!this.platform.isInstalled(pluginId)) {
      throw new Error(status?.reason ?? `Plugin ${pluginId} has no executable package loader.`);
    }

    try {
      await this.platform.activate(pluginId);
      await this.installation.recordActivation(pluginId, true);
    } catch (error) {
      const message = errorMessage(error);
      const state = await this.installation.recordActivation(pluginId, false, message);
      if (state.quarantined) {
        await this.removeRuntime(pluginId);
      }
      throw error;
    }
  }

  async deactivate(pluginId: string): Promise<void> {
    if (!this.platform.isInstalled(pluginId)) return;
    await this.platform.deactivate(pluginId);
  }

  async activateByEvent(event: PluginActivationEvent): Promise<PluginActivationEventResult[]> {
    await this.sync();
    const results: PluginActivationEventResult[] = [];

    for (const pkg of this.managed.values()) {
      if (!pkg.readyToActivate) continue;
      const events = pkg.discovered.manifest.activationEvents ?? ['onStartup'];
      if (!events.includes(event)) continue;
      if (!this.platform.isInstalled(pkg.discovered.manifest.id)) continue;

      try {
        await this.activate(pkg.discovered.manifest.id);
        results.push({ pluginId: pkg.discovered.manifest.id, activated: true });
      } catch (error) {
        results.push({
          pluginId: pkg.discovered.manifest.id,
          activated: false,
          error: errorMessage(error),
        });
      }
    }
    return results;
  }

  activateStartup(): Promise<PluginActivationEventResult[]> {
    return this.activateByEvent('onStartup');
  }

  /**
   * Best-effort lifecycle shutdown for application teardown. Every installed
   * plugin gets a chance to deactivate; one failing hook never blocks cleanup
   * of the remaining plugin scopes.
   */
  async shutdown(): Promise<readonly { pluginId: string; error?: string }[]> {
    const results: Array<{ pluginId: string; error?: string }> = [];
    const runtimes = [...this.platform.list()].reverse();

    for (const runtime of runtimes) {
      try {
        await this.platform.deactivate(runtime.manifest.id);
        results.push({ pluginId: runtime.manifest.id });
      } catch (error) {
        results.push({ pluginId: runtime.manifest.id, error: errorMessage(error) });
      }
    }

    return results;
  }

  async setEnabled(pluginId: string, enabled: boolean): Promise<PluginPackageState> {
    const state = await this.installation.setEnabled(pluginId, enabled);
    await this.sync();
    return state;
  }

  async setTrusted(pluginId: string, trusted: boolean): Promise<PluginPackageState> {
    const state = await this.installation.setTrusted(pluginId, trusted);
    await this.sync();
    return state;
  }

  async setGrantedPermissions(
    pluginId: string,
    permissions: readonly PluginPermission[],
  ): Promise<PluginPackageState> {
    const state = await this.installation.setGrantedPermissions(pluginId, permissions);
    await this.sync();
    return state;
  }

  async clearQuarantine(pluginId: string): Promise<PluginPackageState> {
    const state = await this.installation.clearQuarantine(pluginId);
    await this.sync();
    return state;
  }

  private async reconcilePackage(pkg: ManagedPluginPackage): Promise<ManagedPluginRuntimeSnapshot> {
    const id = pkg.discovered.manifest.id;

    if (!pkg.discovered.valid) {
      return { plugin: pkg, status: 'invalid', reason: pkg.discovered.errors.join(' ') || 'Invalid plugin package.' };
    }
    if (!pkg.state.enabled) {
      return { plugin: pkg, status: 'disabled', reason: 'Plugin is disabled.' };
    }
    if (pkg.state.quarantined) {
      return { plugin: pkg, status: 'quarantined', reason: pkg.state.quarantineReason || 'Plugin is quarantined.' };
    }
    if (pkg.missingPermissions.length) {
      return {
        plugin: pkg,
        status: 'permissions-required',
        reason: `Missing permissions: ${pkg.missingPermissions.join(', ')}.`,
      };
    }

    const source = this.sourceFor(pkg);
    let runtime = this.platform.get(id);
    const staleRuntime = runtime && (
      runtime.manifest.version !== pkg.discovered.manifest.version ||
      runtime.source?.uri !== source.uri ||
      !samePermissions(runtime.grantedPermissions, pkg.effectiveGrantedPermissions)
    );

    if (staleRuntime) {
      await this.removeRuntime(id);
      runtime = undefined;
    }

    if (!runtime) {
      if (!this.platform.hasLoader(source.kind)) {
        return {
          plugin: pkg,
          status: 'loader-unavailable',
          reason: `No safe ${source.kind} plugin loader is registered.`,
        };
      }
      await this.platform.installFromSource(source, {
        trusted: false,
        grantedPermissions: pkg.effectiveGrantedPermissions,
        activate: false,
      });
      runtime = this.platform.get(id);
    }

    return {
      plugin: pkg,
      runtime,
      status: runtime?.state === 'active'
        ? 'active'
        : runtime?.state === 'failed'
          ? 'failed'
          : 'installed',
      reason: runtime?.error,
    };
  }

  private async removeIneligibleManagedRuntimes(): Promise<void> {
    for (const runtime of this.platform.list()) {
      if (runtime.source?.metadata?.['managedBy'] !== POLICY_SOURCE_OWNER) continue;
      const pkg = this.managed.get(runtime.manifest.id);
      if (!pkg || !pkg.readyToActivate) {
        await this.removeRuntime(runtime.manifest.id);
      }
    }
  }

  private async removeRuntime(pluginId: string): Promise<void> {
    if (!this.platform.isInstalled(pluginId)) return;
    await this.platform.uninstall(pluginId);
  }

  private sourceFor(pkg: ManagedPluginPackage): PluginPackageSource {
    return {
      kind: 'directory',
      uri: pkg.discovered.path,
      metadata: {
        managedBy: POLICY_SOURCE_OWNER,
        pluginId: pkg.discovered.manifest.id,
        manifestPath: pkg.discovered.manifestPath,
        version: pkg.discovered.manifest.version,
      },
    };
  }

  private mustGetManaged(pluginId: string): ManagedPluginPackage {
    const pkg = this.managed.get(pluginId);
    if (!pkg) throw new Error(`Plugin ${pluginId} is not a discovered package.`);
    return pkg;
  }
}

function samePermissions(a: readonly PluginPermission[], b: readonly PluginPermission[]): boolean {
  if (a.length !== b.length) return false;
  const left = new Set(a);
  return b.every(permission => left.has(permission));
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}