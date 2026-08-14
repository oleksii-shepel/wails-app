import { Injectable } from '@angular/core';
import type { PluginManifest, PluginPermission } from '../plugin/plugin-manifest';
import {
  PluginDiscoveryService,
  type DiscoveredPluginPackage,
} from './plugin-discovery.service';

export type PluginActivationHealth = 'never' | 'success' | 'failed';

export interface PluginPackageState {
  pluginId: string;
  enabled: boolean;
  trusted: boolean;
  trustedVersion?: string;
  grantedPermissions: PluginPermission[];
  quarantined: boolean;
  quarantineReason?: string;
  lastActivationStatus: PluginActivationHealth;
  lastActivationError?: string;
  lastActivatedAt?: number;
  lastSeenVersion?: string;
  updatedAt: number;
}

export interface ManagedPluginPackage {
  discovered: DiscoveredPluginPackage;
  state: PluginPackageState;
  requestedPermissions: PluginPermission[];
  effectiveGrantedPermissions: PluginPermission[];
  missingPermissions: PluginPermission[];
  readyToActivate: boolean;
}

export interface PluginPackageStatePatch {
  enabled?: boolean;
  trusted?: boolean;
  grantedPermissions?: PluginPermission[];
  clearQuarantine?: boolean;
}

interface PluginStateBindings {
  GetPluginPackageStates(): Promise<unknown[]>;
  GetPluginPackageState(pluginId: string): Promise<unknown>;
  UpdatePluginPackageState(pluginId: string, patch: PluginPackageStatePatch): Promise<unknown>;
  QuarantinePluginPackage(pluginId: string, reason: string): Promise<unknown>;
  RecordPluginActivation(pluginId: string, success: boolean, activationError: string): Promise<unknown>;
}

function appBindings(): PluginStateBindings {
  const bindings = (window as any)?.go?.app?.App as PluginStateBindings | undefined;
  if (!bindings?.GetPluginPackageStates || !bindings?.UpdatePluginPackageState) {
    throw new Error('Plugin state bindings are unavailable. Regenerate Wails bindings and restart the application.');
  }
  return bindings;
}

@Injectable({ providedIn: 'root' })
export class PluginInstallationStateService {
  constructor(private readonly discovery: PluginDiscoveryService) {}

  async refresh(): Promise<ManagedPluginPackage[]> {
    const discovered = await this.discovery.discover();
    const states = (await appBindings().GetPluginPackageStates()).map(normalizePluginPackageState);
    const byId = new Map(states.map(state => [state.pluginId, state]));

    return discovered.map(pkg => {
      const state = byId.get(pkg.manifest.id) ?? defaultPluginPackageState(pkg.manifest.id, pkg.manifest.version);
      return buildManagedPluginPackage(pkg, state);
    });
  }

  async getState(pluginId: string): Promise<PluginPackageState> {
    return normalizePluginPackageState(await appBindings().GetPluginPackageState(pluginId));
  }

  async setEnabled(pluginId: string, enabled: boolean): Promise<PluginPackageState> {
    return this.update(pluginId, { enabled });
  }

  async setTrusted(pluginId: string, trusted: boolean): Promise<PluginPackageState> {
    return this.update(pluginId, { trusted });
  }

  async setGrantedPermissions(pluginId: string, permissions: readonly PluginPermission[]): Promise<PluginPackageState> {
    return this.update(pluginId, { grantedPermissions: [...permissions] });
  }

  async clearQuarantine(pluginId: string): Promise<PluginPackageState> {
    return this.update(pluginId, { clearQuarantine: true });
  }

  async quarantine(pluginId: string, reason: string): Promise<PluginPackageState> {
    return normalizePluginPackageState(await appBindings().QuarantinePluginPackage(pluginId, reason));
  }

  async recordActivation(pluginId: string, success: boolean, error = ''): Promise<PluginPackageState> {
    return normalizePluginPackageState(await appBindings().RecordPluginActivation(pluginId, success, error));
  }

  private async update(pluginId: string, patch: PluginPackageStatePatch): Promise<PluginPackageState> {
    return normalizePluginPackageState(await appBindings().UpdatePluginPackageState(pluginId, patch));
  }
}

export function buildManagedPluginPackage(
  discovered: DiscoveredPluginPackage,
  state: PluginPackageState,
): ManagedPluginPackage {
  const requestedPermissions = [...(discovered.manifest.permissions ?? [])];
  const trustedForCurrentVersion = state.trusted && state.trustedVersion === discovered.manifest.version;
  const effectiveGrantedPermissions = trustedForCurrentVersion
    ? requestedPermissions
    : requestedPermissions.filter(permission => state.grantedPermissions.includes(permission));
  const granted = new Set(effectiveGrantedPermissions);
  const missingPermissions = requestedPermissions.filter(permission => !granted.has(permission));

  return {
    discovered,
    state,
    requestedPermissions,
    effectiveGrantedPermissions,
    missingPermissions,
    readyToActivate:
      discovered.valid &&
      state.enabled &&
      !state.quarantined &&
      missingPermissions.length === 0,
  };
}

export function normalizePluginPackageState(value: unknown): PluginPackageState {
  const raw = (value ?? {}) as Record<string, unknown>;
  return {
    pluginId: stringValue(raw['pluginId']),
    enabled: raw['enabled'] === true,
    trusted: raw['trusted'] === true,
    trustedVersion: optionalString(raw['trustedVersion']),
    grantedPermissions: stringArray(raw['grantedPermissions']) as PluginPermission[],
    quarantined: raw['quarantined'] === true,
    quarantineReason: optionalString(raw['quarantineReason']),
    lastActivationStatus: activationStatus(raw['lastActivationStatus']),
    lastActivationError: optionalString(raw['lastActivationError']),
    lastActivatedAt: optionalNumber(raw['lastActivatedAt']),
    lastSeenVersion: optionalString(raw['lastSeenVersion']),
    updatedAt: numberValue(raw['updatedAt']),
  };
}

function defaultPluginPackageState(pluginId: string, version?: string): PluginPackageState {
  return {
    pluginId,
    enabled: false,
    trusted: false,
    trustedVersion: undefined,
    grantedPermissions: [],
    quarantined: false,
    lastActivationStatus: 'never',
    lastSeenVersion: version,
    updatedAt: 0,
  };
}

function activationStatus(value: unknown): PluginActivationHealth {
  return value === 'success' || value === 'failed' ? value : 'never';
}
function stringValue(value: unknown): string { return typeof value === 'string' ? value : ''; }
function optionalString(value: unknown): string | undefined { return typeof value === 'string' && value ? value : undefined; }
function numberValue(value: unknown): number { return typeof value === 'number' && Number.isFinite(value) ? value : 0; }
function optionalNumber(value: unknown): number | undefined { return typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : undefined; }
function stringArray(value: unknown): string[] { return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []; }
