import { Injectable, inject } from '@angular/core';
import type { Type } from '@angular/core';
import type {
  PluginDescriptor,
  PluginRegistryAPI,
  ToolbarActionContribution,
  ToolbarContext,
  ContextMenuContribution,
  MenuContext,
  SettingsPanelContribution,
  FileCategoryResolver,
} from '../plugin/plugin-registry-api';
import type { ViewerContribution } from '../viewers/viewer-contribution';
import type { SurfaceContribution } from '../surfaces/surface-contribution';
import { ViewerRegistryService } from '../viewers/viewer-registry.service';
import { SurfaceRegistryService } from '../surfaces/surface-registry.service';
import { InputEngineService } from './input-engine.service';
import type { CommandDescriptor } from './input-system.types';

export interface PluginBackend {
  id: string;
  onRegister(api: PluginRegistryAPI): void;
  onUnregister(api: PluginRegistryAPI): void;
}

@Injectable({ providedIn: 'root' })
export class PluginManagerService implements PluginRegistryAPI {
  private readonly viewerRegistry = inject(ViewerRegistryService);
  private readonly surfaceRegistry = inject(SurfaceRegistryService);
  private readonly inputEngine = inject(InputEngineService);

  private readonly backends = new Map<string, PluginBackend>();
  private readonly descriptors = new Map<string, PluginDescriptor>();

  readonly toolbarActions: ToolbarActionContribution[] = [];
  readonly contextMenuItems: ContextMenuContribution[] = [];
  readonly settingsPanels: SettingsPanelContribution[] = [];
  readonly fileCategories: FileCategoryResolver[] = [];

  constructor() {}

  /** Register a plugin backend (typically called by a plugin module at init). */
  registerBackend(backend: PluginBackend, descriptor?: PluginDescriptor): void {
    if (this.backends.has(backend.id)) {
      console.warn(`Plugin backend ${backend.id} already registered; skipping.`);
      return;
    }
    this.backends.set(backend.id, backend);
    if (descriptor) {
      this.descriptors.set(backend.id, descriptor);
    }
    backend.onRegister(this);
  }

  /** Unregister a plugin backend and all of its contributions. */
  unregisterBackend(id: string): void {
    const backend = this.backends.get(id);
    if (!backend) return;
    backend.onUnregister(this);
    this.backends.delete(id);
    this.descriptors.delete(id);
  }

  /** List all registered plugin descriptors. */
  listPlugins(): PluginDescriptor[] {
    return Array.from(this.descriptors.values());
  }

  /** Get a single plugin descriptor. */
  getPlugin(id: string): PluginDescriptor | undefined {
    return this.descriptors.get(id);
  }

  // ── PluginRegistryAPI implementation ───────────────────────────────────────

  registerCommand(command: CommandDescriptor): void {
    this.inputEngine.registerCommandDescriptor(command, this.commandOwner(command.id));
  }

  unregisterCommand(id: string): void {
    this.inputEngine.unregisterCommandDescriptor(id, this.commandOwner(id));
  }

  registerViewer(contribution: ViewerContribution): void {
    this.viewerRegistry.register(contribution);
  }

  unregisterViewer(id: string): void {
    this.viewerRegistry.unregister(id);
  }

  registerSurface(contribution: SurfaceContribution): void {
    this.surfaceRegistry.register(contribution);
  }

  unregisterSurface(id: string): void {
    this.surfaceRegistry.unregister(id);
  }

  registerToolbarAction(action: ToolbarActionContribution): void {
    const i = this.toolbarActions.findIndex(x => x.id === action.id);
    if (i >= 0) this.toolbarActions.splice(i, 1, action);
    else this.toolbarActions.push(action);
  }

  unregisterToolbarAction(id: string): void {
    const i = this.toolbarActions.findIndex(x => x.id === id);
    if (i >= 0) this.toolbarActions.splice(i, 1);
  }

  registerContextMenuItem(item: ContextMenuContribution): void {
    const i = this.contextMenuItems.findIndex(x => x.id === item.id);
    if (i >= 0) this.contextMenuItems.splice(i, 1, item);
    else this.contextMenuItems.push(item);
  }

  unregisterContextMenuItem(id: string): void {
    const i = this.contextMenuItems.findIndex(x => x.id === id);
    if (i >= 0) this.contextMenuItems.splice(i, 1);
  }

  registerSettingsPanel(panel: SettingsPanelContribution): void {
    const i = this.settingsPanels.findIndex(x => x.id === panel.id);
    if (i >= 0) this.settingsPanels.splice(i, 1, panel);
    else this.settingsPanels.push(panel);
  }

  unregisterSettingsPanel(id: string): void {
    const i = this.settingsPanels.findIndex(x => x.id === id);
    if (i >= 0) this.settingsPanels.splice(i, 1);
  }

  registerFileCategory(resolver: FileCategoryResolver): void {
    const idx = this.fileCategories.findIndex(r => r.id === resolver.id);
    if (idx >= 0) {
      this.fileCategories.splice(idx, 1, resolver);
    } else {
      this.fileCategories.push(resolver);
    }
  }

  unregisterFileCategory(id: string): void {
    const idx = this.fileCategories.findIndex(r => r.id === id);
    if (idx >= 0) {
      this.fileCategories.splice(idx, 1);
    }
  }

  /** Resolve toolbar actions applicable to the given context. */
  resolveToolbarActions(context: ToolbarContext): ToolbarActionContribution[] {
    return this.toolbarActions
      .map(a => ({ action: a, score: a.score(context) }))
      .filter(x => x.score > 0)
      .sort((a, b) => b.score - a.score)
      .map(x => x.action);
  }

  /** Resolve context menu items applicable to the given context. */
  resolveContextMenuItems(context: MenuContext): ContextMenuContribution[] {
    return this.contextMenuItems
      .map(i => ({ item: i, score: i.score(context) }))
      .filter(x => x.score > 0)
      .sort((a, b) => b.score - a.score)
      .map(x => x.item);
  }

  /** Resolve a file category for the given path. */
  resolveFileCategory(path: string, mimeType?: string | null): import('../plugin/plugin-registry-api').FileCategory | null {
    for (const resolver of this.fileCategories) {
      const cat = resolver.resolve(path, mimeType);
      if (cat) return cat;
    }
    return null;
  }

  private commandOwner(commandId: string): string {
    return `plugin-command:${commandId}`;
  }

}