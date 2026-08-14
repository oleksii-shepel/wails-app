import { Injectable } from '@angular/core';
import type { ToolbarContext, MenuContext } from '../plugin/plugin-registry-api';
import type { DeclarativePluginModule, DeclarativePluginPlacement } from '../plugin/declarative-plugin-module';
import type { PluginPackage, PluginPackageLoader, PluginPackageSource } from '../plugin/plugin-package';
import type { PluginManifest } from '../plugin/plugin-manifest';
import { InputEngineService } from './input-engine.service';

interface DirectoryPluginBindings {
  ReadPluginFrontendModule(pluginId: string): Promise<unknown>;
  DiscoverPluginPackages(): Promise<unknown[]>;
}

function appBindings(): DirectoryPluginBindings {
  const bindings = (window as any)?.go?.app?.App as DirectoryPluginBindings | undefined;
  if (!bindings?.ReadPluginFrontendModule || !bindings?.DiscoverPluginPackages) {
    throw new Error('Directory plugin loader bindings are unavailable. Regenerate Wails bindings and restart the application.');
  }
  return bindings;
}

@Injectable({ providedIn: 'root' })
export class DirectoryPluginPackageLoaderService implements PluginPackageLoader {
  readonly kind = 'directory';

  constructor(private readonly inputEngine: InputEngineService) {}

  async load(source: PluginPackageSource): Promise<PluginPackage> {
    const pluginId = source.metadata?.['pluginId'];
    if (typeof pluginId !== 'string' || !pluginId) {
      throw new Error('Managed directory plugin source is missing pluginId metadata.');
    }

    const manifest = await this.loadManifest(pluginId);
    const module = normalizeModule(await appBindings().ReadPluginFrontendModule(pluginId));
    return this.buildPackage(manifest, module);
  }

  private async loadManifest(pluginId: string): Promise<PluginManifest> {
    const packages = await appBindings().DiscoverPluginPackages();
    for (const value of packages ?? []) {
      const raw = (value ?? {}) as Record<string, unknown>;
      const manifest = (raw['manifest'] ?? {}) as PluginManifest;
      if (manifest.id === pluginId && raw['valid'] === true) return manifest;
    }
    throw new Error(`Plugin ${pluginId} is not a valid discovered directory package.`);
  }

  private buildPackage(manifest: PluginManifest, module: DeclarativePluginModule): PluginPackage {
    return {
      manifest,
      activate: context => {
        const commandIds = new Map<string, string>();

        for (const command of module.commands ?? []) {
          const id = `plugin.directory.${manifest.id}.${command.id}`;
          commandIds.set(command.id, id);
          context.api.registerCommand({
            id,
            title: command.title?.trim() || command.id,
            category: manifest.name || 'Plugins',
            name: `${manifest.id}.${command.name?.trim() || command.id}`,
            aliases: command.aliases,
            execute: invocation => {
              const dispatched = this.inputEngine.dispatchCommandById(command.targetCommandId, invocation);
              if (!dispatched) throw new Error(`Target command ${command.targetCommandId} is unavailable.`);
            },
            isEnabled: invocation => this.inputEngine.isCommandEnabled(command.targetCommandId, invocation),
          });
        }

        for (const placement of module.toolbar ?? []) {
          context.api.registerToolbarAction({
            id: `${manifest.id}.toolbar.${placement.id}`,
            commandId: mustResolveCommand(commandIds, placement),
            score: toolbarScore(placement),
          });
        }

        for (const placement of module.contextMenu ?? []) {
          context.api.registerContextMenuItem({
            id: `${manifest.id}.contextMenu.${placement.id}`,
            commandId: mustResolveCommand(commandIds, placement),
            score: menuScore(placement),
          });
        }
      },
    };
  }
}

function normalizeModule(value: unknown): DeclarativePluginModule {
  const raw = (value ?? {}) as Record<string, unknown>;
  if (raw['schemaVersion'] !== 1) throw new Error(`Unsupported declarative plugin module schema ${String(raw['schemaVersion'])}.`);
  return raw as unknown as DeclarativePluginModule;
}

function mustResolveCommand(commands: ReadonlyMap<string, string>, placement: DeclarativePluginPlacement): string {
  const id = commands.get(placement.commandId);
  if (!id) throw new Error(`Plugin placement ${placement.id} references unknown command ${placement.commandId}.`);
  return id;
}

function placementMatches(when: DeclarativePluginPlacement['when'], selectedCount: number): boolean {
  switch (when ?? 'always') {
    case 'always': return true;
    case 'selection': return selectedCount > 0;
    case 'single-selection': return selectedCount === 1;
  }
}

function toolbarScore(placement: DeclarativePluginPlacement): (context: ToolbarContext) => number {
  return context => placementMatches(placement.when, context.selectedPaths.length) ? 100 : 0;
}

function menuScore(placement: DeclarativePluginPlacement): (context: MenuContext) => number {
  return context => placementMatches(placement.when, context.selectedPaths.length) ? 100 : 0;
}
