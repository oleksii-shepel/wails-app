import { Injectable } from '@angular/core';
import type { PluginManifest } from '../plugin/plugin-manifest';

export interface DiscoveredPluginPackage {
  manifest: PluginManifest;
  path: string;
  manifestPath: string;
  valid: boolean;
  errors: string[];
  warnings: string[];
}

interface PluginDiscoveryBindings {
  GetPluginDirectory(): Promise<string>;
  DiscoverPluginPackages(): Promise<unknown[]>;
}

function appBindings(): PluginDiscoveryBindings {
  const bindings = (window as any)?.go?.app?.App as PluginDiscoveryBindings | undefined;
  if (!bindings?.GetPluginDirectory || !bindings?.DiscoverPluginPackages) {
    throw new Error('Plugin discovery bindings are unavailable. Regenerate Wails bindings and restart the application.');
  }
  return bindings;
}

@Injectable({ providedIn: 'root' })
export class PluginDiscoveryService {
  getDirectory(): Promise<string> {
    return appBindings().GetPluginDirectory();
  }

  async discover(): Promise<DiscoveredPluginPackage[]> {
    const raw = await appBindings().DiscoverPluginPackages();
    return (raw ?? []).map(normalizeDiscoveredPluginPackage);
  }
}

export function normalizeDiscoveredPluginPackage(value: unknown): DiscoveredPluginPackage {
  const raw = (value ?? {}) as Record<string, unknown>;
  return {
    manifest: (raw['manifest'] ?? {}) as PluginManifest,
    path: typeof raw['path'] === 'string' ? raw['path'] : '',
    manifestPath: typeof raw['manifestPath'] === 'string' ? raw['manifestPath'] : '',
    valid: raw['valid'] === true,
    errors: stringArray(raw['errors']),
    warnings: stringArray(raw['warnings']),
  };
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
}
