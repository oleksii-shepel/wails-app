import type {
  ContextMenuContribution,
  FileCategoryResolver,
  PluginRegistryAPI,
  ToolbarActionContribution,
  PluginResultsCollection,
} from './plugin-registry-api';
import type { PluginPermission } from './plugin-manifest';
import type { CommandDescriptor, PluginInputContribution } from '../services/input-system.types';
import type { ViewerContribution } from '../viewers/viewer-contribution';
import type { SurfaceContribution } from '../surfaces/surface-contribution';
import type { RelatedLocationContribution } from '../services/related-location.types';
import type { WorkspaceSurfaceActionContribution } from '../services/workspace-surface-action.types';

/**
 * Plugin-facing registry wrapper. Every successful contribution is recorded and
 * removed in reverse order when the scope is disposed.
 */
export class ScopedPluginRegistry implements PluginRegistryAPI {
  private readonly cleanups: Array<() => void> = [];
  private disposed = false;

  constructor(
    private readonly pluginId: string,
    private readonly host: PluginRegistryAPI,
    private readonly permissions: ReadonlySet<PluginPermission>,
  ) {}

  dispose(): void {
    if (this.disposed) return;
    this.disposed = true;
    for (let i = this.cleanups.length - 1; i >= 0; i--) {
      try { this.cleanups[i](); } catch (error) { console.error(`Plugin ${this.pluginId} cleanup failed:`, error); }
    }
    this.cleanups.length = 0;
  }

  registerInput(contribution: PluginInputContribution): void {
    this.assertActive();
    this.require('commands');
    this.require('keybindings');
    const owned = { ...contribution, pluginId: this.pluginId };
    this.host.registerInput(owned);
    this.cleanups.push(() => this.host.unregisterInput(this.pluginId));
  }

  unregisterInput(pluginId: string): void {
    this.assertOwnId(pluginId);
    this.host.unregisterInput(pluginId);
  }

  registerCommand(command: CommandDescriptor): void {
    this.assertActive(); this.require('commands');
    this.host.registerCommand(command);
    this.cleanups.push(() => this.host.unregisterCommand(command.id));
  }
  unregisterCommand(id: string): void { this.host.unregisterCommand(id); }

  registerViewer(contribution: ViewerContribution): void {
    this.assertActive(); this.require('viewers');
    this.host.registerViewer(contribution);
    this.cleanups.push(() => this.host.unregisterViewer(contribution.id));
  }
  unregisterViewer(id: string): void { this.host.unregisterViewer(id); }

  registerSurface(contribution: SurfaceContribution): void {
    this.assertActive(); this.require('surfaces');
    this.host.registerSurface(contribution);
    this.cleanups.push(() => this.host.unregisterSurface(contribution.id));
  }
  unregisterSurface(id: string): void { this.host.unregisterSurface(id); }

  registerWorkspaceSurfaceAction(
    contribution: WorkspaceSurfaceActionContribution,
  ): void {
    this.assertActive();
    this.require('surfaces');
    this.host.registerWorkspaceSurfaceAction(contribution);
    this.cleanups.push(
      () => this.host.unregisterWorkspaceSurfaceAction(contribution.id),
    );
  }

  unregisterWorkspaceSurfaceAction(id: string): void {
    this.host.unregisterWorkspaceSurfaceAction(id);
  }

  registerToolbarAction(action: ToolbarActionContribution): void {
    this.assertActive(); this.require('toolbar');
    this.host.registerToolbarAction(action);
    this.cleanups.push(() => this.host.unregisterToolbarAction(action.id));
  }
  unregisterToolbarAction(id: string): void { this.host.unregisterToolbarAction(id); }

  registerContextMenuItem(item: ContextMenuContribution): void {
    this.assertActive(); this.require('contextMenu');
    this.host.registerContextMenuItem(item);
    this.cleanups.push(() => this.host.unregisterContextMenuItem(item.id));
  }
  unregisterContextMenuItem(id: string): void { this.host.unregisterContextMenuItem(id); }


  registerFileCategory(resolver: FileCategoryResolver): void {
    this.assertActive(); this.require('fileCategories');
    this.host.registerFileCategory(resolver);
    this.cleanups.push(() => this.host.unregisterFileCategory(resolver.id));
  }
  unregisterFileCategory(id: string): void { this.host.unregisterFileCategory(id); }

  registerRelatedLocation(contribution: RelatedLocationContribution): void {
    this.assertActive(); this.require('relatedLocations');
    this.host.registerRelatedLocation(contribution);
    this.cleanups.push(() => this.host.unregisterRelatedLocation(contribution.id));
  }
  unregisterRelatedLocation(id: string): void { this.host.unregisterRelatedLocation(id); }

  createResultsCollection(collection: PluginResultsCollection): string {
    this.assertActive();
    this.require('collections');
    const path = this.host.createResultsCollection({
      ...collection,
      ownerPluginId: this.pluginId,
    });
    this.cleanups.push(() => this.host.removeResultsCollection(path));
    return path;
  }

  removeResultsCollection(path: string): void {
    this.assertActive();
    this.require('collections');
    this.host.removeResultsCollection(path);
  }

  private require(permission: PluginPermission): void {
    if (!this.permissions.has(permission)) {
      throw new Error(`Plugin ${this.pluginId} is not granted permission ${permission}.`);
    }
  }

  private assertActive(): void {
    if (this.disposed) throw new Error(`Plugin ${this.pluginId} contribution scope is already disposed.`);
  }

  private assertOwnId(pluginId: string): void {
    if (pluginId !== this.pluginId) {
      throw new Error(`Plugin ${this.pluginId} cannot unregister input owned by ${pluginId}.`);
    }
  }
}