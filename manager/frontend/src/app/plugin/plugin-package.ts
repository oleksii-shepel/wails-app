import type { PluginRegistryAPI } from './plugin-registry-api';
import type { PluginManifest, PluginPermission } from './plugin-manifest';

export type MaybePromise<T> = T | Promise<T>;

export interface PluginPackage {
  manifest: PluginManifest;
  activate(context: PluginActivationContext): MaybePromise<void>;
  deactivate?(context: PluginActivationContext): MaybePromise<void>;
}

export interface PluginActivationContext {
  readonly pluginId: string;
  readonly manifest: PluginManifest;
  readonly api: PluginRegistryAPI;
  readonly permissions: ReadonlySet<PluginPermission>;
  readonly host: {
    readonly pluginApiVersion: string;
  };
  hasPermission(permission: PluginPermission): boolean;
}

export interface PluginPackageSource {
  kind: string;
  uri: string;
  metadata?: Readonly<Record<string, unknown>>;
}

/** Loader abstraction for future directory/zip/remote package discovery. */
export interface PluginPackageLoader {
  readonly kind: string;
  load(source: PluginPackageSource): Promise<PluginPackage>;
}