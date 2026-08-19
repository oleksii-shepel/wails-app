import { Injectable } from '@angular/core';
import { PluginManagerService } from './plugin-manager.service';
import {
  PLUGIN_API_VERSION,
  type PluginActivationEvent,
  type PluginManifest,
  type PluginPermission,
  validatePluginManifest,
} from '../plugin/plugin-manifest';
import type {
  PluginActivationContext,
  PluginPackage,
  PluginPackageLoader,
  PluginPackageSource,
} from '../plugin/plugin-package';
import { ScopedPluginRegistry } from '../plugin/scoped-plugin-registry';

export type PluginLifecycleState =
  | 'installed'
  | 'activating'
  | 'active'
  | 'deactivating'
  | 'inactive'
  | 'failed'
  | 'incompatible';

export interface PluginRuntimeSnapshot {
  manifest: PluginManifest;
  state: PluginLifecycleState;
  grantedPermissions: readonly PluginPermission[];
  missingPermissions: readonly PluginPermission[];
  error?: string;
  source?: PluginPackageSource;
}

export interface InstallPluginOptions {
  trusted?: boolean;
  grantedPermissions?: readonly PluginPermission[];
  source?: PluginPackageSource;
  activate?: boolean;
}

interface PluginRuntime {
  pkg: PluginPackage;
  state: PluginLifecycleState;
  granted: Set<PluginPermission>;
  scope?: ScopedPluginRegistry;
  context?: PluginActivationContext;
  error?: string;
  source?: PluginPackageSource;
}

@Injectable({ providedIn: 'root' })
export class PluginPlatformService {
  private readonly runtimes = new Map<string, PluginRuntime>();
  private readonly loaders = new Map<string, PluginPackageLoader>();

  constructor(private readonly contributions: PluginManagerService) {}

  async install(pkg: PluginPackage, options: InstallPluginOptions = {}): Promise<void> {
    const manifest = pkg.manifest;
    if (this.runtimes.has(manifest.id)) {
      throw new Error(`Plugin ${manifest.id} is already installed.`);
    }

    const validation = validatePluginManifest(manifest, PLUGIN_API_VERSION);
    const requested = new Set(manifest.permissions ?? []);
    const granted = options.trusted
      ? new Set(requested)
      : new Set(options.grantedPermissions ?? []);

    const runtime: PluginRuntime = {
      pkg,
      state: validation.valid ? 'installed' : 'incompatible',
      granted,
      source: options.source,
      error: validation.valid ? undefined : validation.errors.join(' '),
    };
    this.runtimes.set(manifest.id, runtime);

    if (!validation.valid) return;
    if (options.activate) await this.activate(manifest.id);
  }

  async uninstall(pluginId: string): Promise<void> {
    const runtime = this.mustGet(pluginId);
    if (runtime.state === 'active' || runtime.state === 'failed') {
      await this.deactivate(pluginId);
    }
    this.runtimes.delete(pluginId);
  }

  async activate(pluginId: string): Promise<void> {
    const runtime = this.mustGet(pluginId);
    if (runtime.state === 'active') return;
    if (runtime.state === 'incompatible') {
      throw new Error(runtime.error ?? `Plugin ${pluginId} is incompatible.`);
    }

    const missing = this.missingPermissions(runtime);
    if (missing.length) {
      throw new Error(`Plugin ${pluginId} is missing permissions: ${missing.join(', ')}.`);
    }

    runtime.state = 'activating';
    runtime.error = undefined;
    const scope = new ScopedPluginRegistry(pluginId, this.contributions, runtime.granted);
    const context: PluginActivationContext = {
      pluginId,
      manifest: runtime.pkg.manifest,
      api: scope,
      permissions: runtime.granted,
      host: { pluginApiVersion: PLUGIN_API_VERSION },
      hasPermission: permission => runtime.granted.has(permission),
    };
    runtime.scope = scope;
    runtime.context = context;

    try {
      await runtime.pkg.activate(context);
      runtime.state = 'active';
    } catch (error) {
      scope.dispose();
      runtime.scope = undefined;
      runtime.context = undefined;
      runtime.state = 'failed';
      runtime.error = error instanceof Error ? error.message : String(error);
      throw error;
    }
  }

  async deactivate(pluginId: string): Promise<void> {
    const runtime = this.mustGet(pluginId);
    if (runtime.state !== 'active' && runtime.state !== 'failed') return;

    runtime.state = 'deactivating';
    let deactivateError: unknown;
    try {
      if (runtime.context) await runtime.pkg.deactivate?.(runtime.context);
    } catch (error) {
      deactivateError = error;
    } finally {
      runtime.scope?.dispose();
      runtime.scope = undefined;
      runtime.context = undefined;
      runtime.state = 'inactive';
    }

    if (deactivateError) throw deactivateError;
  }

  async activateByEvent(event: PluginActivationEvent): Promise<string[]> {
    const activated: string[] = [];
    for (const [id, runtime] of this.runtimes) {
      if (runtime.state !== 'installed' && runtime.state !== 'inactive') continue;
      const events = runtime.pkg.manifest.activationEvents ?? ['onStartup'];
      if (!events.includes(event)) continue;
      await this.activate(id);
      activated.push(id);
    }
    return activated;
  }

  setGrantedPermissions(pluginId: string, permissions: readonly PluginPermission[]): void {
    const runtime = this.mustGet(pluginId);
    if (runtime.state === 'active' || runtime.state === 'activating') {
      throw new Error(`Deactivate plugin ${pluginId} before changing permissions.`);
    }
    runtime.granted = new Set(permissions);
  }

  list(): PluginRuntimeSnapshot[] {
    return Array.from(this.runtimes.values()).map(runtime => this.snapshot(runtime));
  }

  get(pluginId: string): PluginRuntimeSnapshot | undefined {
    const runtime = this.runtimes.get(pluginId);
    return runtime ? this.snapshot(runtime) : undefined;
  }

  registerLoader(loader: PluginPackageLoader): void {
    if (this.loaders.has(loader.kind)) throw new Error(`Plugin loader ${loader.kind} is already registered.`);
    this.loaders.set(loader.kind, loader);
  }

  unregisterLoader(kind: string): void { this.loaders.delete(kind); }

  hasLoader(kind: string): boolean { return this.loaders.has(kind); }

  isInstalled(pluginId: string): boolean { return this.runtimes.has(pluginId); }

  async installFromSource(source: PluginPackageSource, options: Omit<InstallPluginOptions, 'source'> = {}): Promise<void> {
    const loader = this.loaders.get(source.kind);
    if (!loader) throw new Error(`No plugin package loader registered for ${source.kind}.`);
    const pkg = await loader.load(source);
    await this.install(pkg, { ...options, source });
  }

  private mustGet(pluginId: string): PluginRuntime {
    const runtime = this.runtimes.get(pluginId);
    if (!runtime) throw new Error(`Plugin ${pluginId} is not installed.`);
    return runtime;
  }

  private missingPermissions(runtime: PluginRuntime): PluginPermission[] {
    return (runtime.pkg.manifest.permissions ?? []).filter(permission => !runtime.granted.has(permission));
  }

  private snapshot(runtime: PluginRuntime): PluginRuntimeSnapshot {
    return {
      manifest: runtime.pkg.manifest,
      state: runtime.state,
      grantedPermissions: Array.from(runtime.granted),
      missingPermissions: this.missingPermissions(runtime),
      error: runtime.error,
      source: runtime.source,
    };
  }
}