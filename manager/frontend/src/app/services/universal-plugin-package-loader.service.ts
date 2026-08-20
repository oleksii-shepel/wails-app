import { Injectable } from '@angular/core';
import type {
  PluginPackage,
  PluginPackageLoader,
  PluginPackageSource,
} from '../plugin/plugin-package';
import type { PluginManifest } from '../plugin/plugin-manifest';

interface UniversalPluginBindings {
  PrepareUniversalPluginPackage(pluginId: string): Promise<{
    manifest: PluginManifest;
    kind: 'archive';
    archivePath: string;
    cachePath: string;
    valid: boolean;
    errors?: string[];
  }>;
  ReadUniversalPluginFrontendEntry(pluginId: string): Promise<string>;
  CallUniversalPluginBackend(pluginId: string, method: string, params: unknown): Promise<unknown>;
  StopUniversalPluginBackend(pluginId: string): Promise<void>;
}

interface UniversalFrontendContext {
  readonly pluginId: string;
  readonly manifest: PluginManifest;
  readonly api: any;
  readonly backend: {
    call<T = unknown>(method: string, params?: unknown): Promise<T>;
  };
  hasPermission(permission: string): boolean;
}

interface UniversalFrontendModule {
  activate(
    context: UniversalFrontendContext,
  ): void | (() => void | Promise<void>) | Promise<void | (() => void | Promise<void>)>;
}

function appBindings(): UniversalPluginBindings {
  const bindings = (window as any)?.go?.app?.App as UniversalPluginBindings | undefined;
  if (
    !bindings?.PrepareUniversalPluginPackage ||
    !bindings?.ReadUniversalPluginFrontendEntry ||
    !bindings?.CallUniversalPluginBackend ||
    !bindings?.StopUniversalPluginBackend
  ) {
    throw new Error(
      'Universal plugin bindings are unavailable. Regenerate Wails bindings and restart the application.',
    );
  }
  return bindings;
}

async function importFrontend(source: string, pluginId: string): Promise<UniversalFrontendModule> {
  const blob = new Blob([source], { type: 'text/javascript' });
  const url = URL.createObjectURL(blob);
  try {
    const module = await import(/* @vite-ignore */ url) as Partial<UniversalFrontendModule>;
    if (typeof module.activate !== 'function') {
      throw new Error(`Universal plugin ${pluginId} frontend does not export activate().`);
    }
    return module as UniversalFrontendModule;
  } finally {
    URL.revokeObjectURL(url);
  }
}

@Injectable({ providedIn: 'root' })
export class UniversalPluginPackageLoaderService implements PluginPackageLoader {
  readonly kind = 'archive';

  async load(source: PluginPackageSource): Promise<PluginPackage> {
    const pluginId = source.metadata?.['pluginId'];
    if (typeof pluginId !== 'string' || !pluginId) {
      throw new Error('Managed archive plugin source is missing pluginId metadata.');
    }

    const prepared = await appBindings().PrepareUniversalPluginPackage(pluginId);
    if (!prepared.valid) {
      throw new Error(
        `Universal plugin ${pluginId} is invalid: ${(prepared.errors ?? []).join(' ')}`,
      );
    }

    const sourceText = await appBindings().ReadUniversalPluginFrontendEntry(pluginId);
    const frontend = await importFrontend(sourceText, pluginId);
    let dispose: void | (() => void | Promise<void>);

    return {
      manifest: prepared.manifest,
      activate: async context => {
        const bindings = appBindings();
        dispose = await frontend.activate({
          pluginId,
          manifest: context.manifest,
          api: context.api,
          backend: {
            call: <T>(method: string, params?: unknown) =>
              bindings.CallUniversalPluginBackend(pluginId, method, params ?? null) as Promise<T>,
          },
          hasPermission: permission =>
            context.permissions.has(permission as any),
        });
      },
      deactivate: async () => {
        try {
          await dispose?.();
        } finally {
          dispose = undefined;
          await appBindings().StopUniversalPluginBackend(pluginId);
        }
      },
    };
  }
}
