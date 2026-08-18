import { Injectable, inject } from '@angular/core';
import type { Type } from '@angular/core';
import type {
  PluginDescriptor,
  PluginRegistryAPI,
  ToolbarActionContribution,
  ToolbarContext,
  ContextMenuContribution,
  MenuContext,
  FileCategoryResolver,
  PanelColumnContribution,
  PanelColumnContext,
  PluginResultsCollection,
} from '../plugin/plugin-registry-api';
import type { ViewerContribution } from '../viewers/viewer-contribution';
import type { SurfaceContribution } from '../surfaces/surface-contribution';
import { ViewerRegistryService } from '../viewers/viewer-registry.service';
import { SurfaceRegistryService } from '../surfaces/surface-registry.service';
import { InputEngineService } from './input-engine.service';
import type { CommandDescriptor, PluginInputContribution } from './input-system.types';
import { RelatedLocationService } from './related-location.service';
import type { RelatedLocationContribution } from './related-location.types';
import { WorkspaceSurfaceActionRegistryService } from './workspace-surface-action-registry.service';
import type { WorkspaceSurfaceActionContribution } from './workspace-surface-action.types';
import type { PanelSource } from './panel-source';
import { PanelSourceRegistryService, RegistryPanelSource, RemotePanelSource } from './panel-source-registry.service';
import { DirectorySourceService } from './directory-source.service';
import { RemoteConnectionsService } from './remote-connections.service';
import { ResilienceService } from './resilience.service';
import { CustomizationRegistryService } from './customization-registry.service';
import type {
  ThemeContribution,
  ColumnProfileContribution,
  LayoutPresetContribution,
  WorkspaceTemplateContribution,
} from './customization-registry.service';
import { ResultsPanelService } from './results-panel.service';
import { ApiService } from './api.service';
import { ConfirmDialogService } from './confirm-dialog.service';
import { AutorunPanelSource } from './autorun-panel-source.service';
import { SystemSurfacePanelSource, systemField } from './system-surface-panel-source';
import { SYSTEM_SURFACES } from './system-surface-definitions';

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
  private readonly relatedLocations = inject(RelatedLocationService);
  private readonly workspaceSurfaceActions =
    inject(WorkspaceSurfaceActionRegistryService);
  private readonly panelSources = inject(PanelSourceRegistryService);
  private readonly directories = inject(DirectorySourceService);
  private readonly remoteConnections = inject(RemoteConnectionsService);
  private readonly resilience = inject(ResilienceService);
  private readonly customizations = inject(CustomizationRegistryService);
  private readonly resultsPanels = inject(ResultsPanelService);
  private readonly api = inject(ApiService);
  private readonly confirmDialog = inject(ConfirmDialogService);
  private readonly autorunSource = inject(AutorunPanelSource);

  private readonly backends = new Map<string, PluginBackend>();
  private readonly descriptors = new Map<string, PluginDescriptor>();
  private registeringBackendId: string | null = null;
  private readonly actionCommands = new Map<string, { commandId: string; ownerId: string }>();

  readonly toolbarActions: ToolbarActionContribution[] = [];
  readonly contextMenuItems: ContextMenuContribution[] = [];
  readonly fileCategories: FileCategoryResolver[] = [];
  readonly panelColumns: PanelColumnContribution[] = [];

  constructor() {
    this.registerBuiltinRegistryProvider();
    this.registerBuiltinAutorunProvider();
    this.registerBuiltinSystemProviders();
    this.registerBuiltinRemoteProviders();
  }

  /** Register a plugin backend (typically called by a plugin module at init). */
  registerBackend(backend: PluginBackend, descriptor?: PluginDescriptor): void {
    if (this.backends.has(backend.id)) {
      console.warn(`Plugin backend ${backend.id} already registered; skipping.`);
      return;
    }

    this.backends.set(backend.id, backend);
    if (descriptor) this.descriptors.set(backend.id, descriptor);

    this.registeringBackendId = backend.id;
    try {
      backend.onRegister(this);
    } catch (error) {
      this.inputEngine.unregisterPluginInput(backend.id);
      this.backends.delete(backend.id);
      this.descriptors.delete(backend.id);
      throw error;
    } finally {
      this.registeringBackendId = null;
    }
  }

  /** Unregister a plugin backend and all of its contributions. */
  unregisterBackend(id: string): void {
    const backend = this.backends.get(id);
    if (!backend) return;

    try {
      backend.onUnregister(this);
    } finally {
      // Preferred plugin input bundles are host-owned: cleanup does not depend
      // on the plugin remembering to unregister every command and binding.
      this.inputEngine.unregisterPluginInput(id);
      this.customizations.unregisterOwner(id);
      this.backends.delete(id);
      this.descriptors.delete(id);
    }
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

  registerInput(contribution: PluginInputContribution): void {
    if (this.registeringBackendId && contribution.pluginId !== this.registeringBackendId) {
      throw new Error(
        `Plugin ${this.registeringBackendId} cannot register input owned by ${contribution.pluginId}.`,
      );
    }
    this.inputEngine.registerPluginInput(contribution);
  }

  unregisterInput(pluginId: string): void {
    this.inputEngine.unregisterPluginInput(pluginId);
  }

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

  registerWorkspaceSurfaceAction(
    contribution: WorkspaceSurfaceActionContribution,
  ): void {
    this.workspaceSurfaceActions.register(contribution);
  }

  unregisterWorkspaceSurfaceAction(id: string): void {
    this.workspaceSurfaceActions.unregister(id);
  }

  registerToolbarAction(action: ToolbarActionContribution): void {
    const normalized = this.normalizeToolbarAction(action);
    const i = this.toolbarActions.findIndex(x => x.id === action.id);
    if (i >= 0) this.toolbarActions.splice(i, 1, normalized);
    else this.toolbarActions.push(normalized);
  }

  unregisterToolbarAction(id: string): void {
    const i = this.toolbarActions.findIndex(x => x.id === id);
    if (i >= 0) this.toolbarActions.splice(i, 1);
    this.unregisterActionCommand(`toolbar:${id}`);
  }

  registerContextMenuItem(item: ContextMenuContribution): void {
    const normalized = this.normalizeContextMenuItem(item);
    const i = this.contextMenuItems.findIndex(x => x.id === item.id);
    if (i >= 0) this.contextMenuItems.splice(i, 1, normalized);
    else this.contextMenuItems.push(normalized);
  }

  unregisterContextMenuItem(id: string): void {
    const i = this.contextMenuItems.findIndex(x => x.id === id);
    if (i >= 0) this.contextMenuItems.splice(i, 1);
    this.unregisterActionCommand(`context-menu:${id}`);
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

  registerPanelSource(source: PanelSource): void {
    this.panelSources.register(source);
  }

  unregisterPanelSource(id: string): void {
    this.panelSources.unregister(id);
  }

  registerPanelColumn(column: PanelColumnContribution): void {
    const index = this.panelColumns.findIndex(candidate => candidate.id === column.id);
    if (index >= 0) this.panelColumns.splice(index, 1, column);
    else this.panelColumns.push(column);
  }

  unregisterPanelColumn(id: string): void {
    const index = this.panelColumns.findIndex(column => column.id === id);
    if (index >= 0) this.panelColumns.splice(index, 1);
  }

  resolvePanelColumns(context: PanelColumnContext): PanelColumnContribution[] {
    return this.panelColumns
      .map(column => ({ column, score: column.score(context) }))
      .filter(candidate => candidate.score > 0)
      .sort((left, right) => right.score - left.score || left.column.id.localeCompare(right.column.id))
      .map(candidate => candidate.column);
  }

  registerRelatedLocation(contribution: RelatedLocationContribution): void {
    this.relatedLocations.register(contribution);
  }

  unregisterRelatedLocation(id: string): void {
    this.relatedLocations.unregister(id);
  }

  registerTheme(theme: ThemeContribution): void {
    this.customizations.registerTheme(theme, this.contributionOwner());
  }

  unregisterTheme(id: string): void {
    this.customizations.unregisterTheme(id, this.contributionOwner());
  }

  registerColumnProfile(profile: ColumnProfileContribution): void {
    this.customizations.registerColumnProfile(profile, this.contributionOwner());
  }

  unregisterColumnProfile(id: string): void {
    this.customizations.unregisterColumnProfile(id, this.contributionOwner());
  }

  registerLayoutPreset(preset: LayoutPresetContribution): void {
    this.customizations.registerLayoutPreset(preset, this.contributionOwner());
  }

  unregisterLayoutPreset(id: string): void {
    this.customizations.unregisterLayoutPreset(id, this.contributionOwner());
  }

  registerWorkspaceTemplate(template: WorkspaceTemplateContribution): void {
    this.customizations.registerWorkspaceTemplate(template, this.contributionOwner());
  }

  unregisterWorkspaceTemplate(id: string): void {
    this.customizations.unregisterWorkspaceTemplate(id, this.contributionOwner());
  }

  createResultsCollection(collection: PluginResultsCollection): string {
    const pluginId = this.registeringBackendId ?? collection.ownerPluginId ?? 'plugin';
    const panel = this.resultsPanels.createPluginCollection(
      pluginId,
      collection.id,
      collection.title,
      collection.entries,
      collection.metadata,
    );

    if (collection.refresh) {
      this.resultsPanels.registerRefresher(panel.path, async () => ({
        entries: await collection.refresh!(),
      }));
    }

    return panel.path;
  }

  removeResultsCollection(path: string): void {
    this.resultsPanels.remove(path);
  }

  private normalizeToolbarAction(action: ToolbarActionContribution): ToolbarActionContribution {
    if (action.commandId?.trim() || !action.execute) return action;

    const commandId = this.registerActionCommand(
      `toolbar:${action.id}`,
      action.id,
      action.label,
      'Toolbar',
      async invocation => {
        const context = invocation?.context as unknown as ToolbarContext | undefined;
        if (context) await action.execute?.(context);
      },
      invocation => {
        const context = invocation?.context as unknown as ToolbarContext | undefined;
        return !!context && action.score(context) > 0;
      },
    );
    return { ...action, commandId, execute: undefined };
  }

  private normalizeContextMenuItem(item: ContextMenuContribution): ContextMenuContribution {
    if (item.commandId?.trim() || !item.execute) return item;

    const commandId = this.registerActionCommand(
      `context-menu:${item.id}`,
      item.id,
      item.label,
      'Context Menu',
      async invocation => {
        const context = invocation?.context as unknown as MenuContext | undefined;
        if (context) await item.execute?.(context);
      },
      invocation => {
        const context = invocation?.context as unknown as MenuContext | undefined;
        return !!context && item.score(context) > 0;
      },
    );
    return { ...item, commandId, execute: undefined };
  }

  private registerActionCommand(
    key: string,
    contributionId: string,
    label: string | undefined,
    category: string,
    execute: CommandDescriptor['execute'],
    isEnabled: NonNullable<CommandDescriptor['isEnabled']>,
  ): string {
    this.unregisterActionCommand(key);
    const pluginId = this.registeringBackendId ?? 'host';
    const commandId = `plugin.${pluginId}.action.${key.replace(':', '.')}`;
    const ownerId = `plugin-action:${pluginId}:${key}`;
    this.inputEngine.registerCommandDescriptor({
      id: commandId,
      title: label?.trim() || contributionId,
      category,
      description: `Action contributed by ${pluginId}`,
      execute,
      isEnabled,
    }, ownerId);
    this.actionCommands.set(key, { commandId, ownerId });
    return commandId;
  }

  private unregisterActionCommand(key: string): void {
    const registration = this.actionCommands.get(key);
    if (!registration) return;
    this.inputEngine.unregisterCommandDescriptor(registration.commandId, registration.ownerId);
    this.actionCommands.delete(key);
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

  private registerBuiltinRegistryProvider(): void {
    const source = new RegistryPanelSource(this.directories);
    const isRegistry = (path: string): boolean => /^registry:\/\//i.test((path ?? '').trim());

    this.registerBackend({
      id: 'builtin.registry',
      onRegister: api => {
        api.registerPanelSource(source);

        api.registerPanelColumn({
          id: 'builtin.registry.kind',
          label: 'Kind',
          width: '90px',
          score: context => context.sourceKind === 'registry' ? 100 : 0,
          value: entry => entry.registryKind ?? '',
        });
        api.registerPanelColumn({
          id: 'builtin.registry.type',
          label: 'Registry Type',
          width: '120px',
          score: context => context.sourceKind === 'registry' ? 95 : 0,
          value: entry => entry.registryValueType ?? '',
        });
        api.registerPanelColumn({
          id: 'builtin.registry.data',
          label: 'Data',
          score: context => context.sourceKind === 'registry' ? 90 : 0,
          value: entry => entry.registryPreview ?? '',
        });

        api.registerContextMenuItem({
          id: 'builtin.registry.newKey',
          label: 'New Registry Key',
          score: context => isRegistry(context.currentDirectory) ? 95 : 0,
          execute: async context => {
            const name = (await this.confirmDialog.prompt({
              title: 'New Registry Key',
              message: 'Enter the key name.',
              confirmText: 'Create',
              historyKey: 'reassembly.registry.newKey',
            }))?.trim();
            if (!name) return;
            const path = await this.api.joinPaths([context.currentDirectory, name]);
            await this.api.createDirectory(path, false);
          },
        });

        api.registerContextMenuItem({
          id: 'builtin.registry.newValue',
          label: 'New Registry Value',
          score: context => isRegistry(context.currentDirectory) ? 94 : 0,
          execute: async context => {
            const name = (await this.confirmDialog.prompt({
              title: 'New Registry Value',
              message: 'Enter the value name.',
              confirmText: 'Next',
              historyKey: 'reassembly.registry.newValue.name',
            }))?.trim();
            if (!name) return;
            const value = await this.confirmDialog.prompt({
              title: `Value — ${name}`,
              message: 'Enter the value data.',
              confirmText: 'Create',
              defaultValue: '',
              historyKey: 'reassembly.registry.newValue.data',
            });
            if (value === null) return;
            const path = await this.api.joinPaths([context.currentDirectory, name]);
            await this.api.writeFile(path, value, 'utf8');
          },
        });

        api.registerContextMenuItem({
          id: 'builtin.registry.editValue',
          label: 'Edit Registry Value',
          score: context => isRegistry(context.targetPath) && context.targetPath !== context.currentDirectory ? 90 : 0,
          execute: async context => {
            const info = await this.api.getFileInfo(context.targetPath);
            if (String(info.registryKind ?? '').toLowerCase() !== 'value') return;
            const current = await this.api.readFile(context.targetPath, 'utf8');
            const value = await this.confirmDialog.prompt({
              title: `Edit Registry Value — ${info.name}`,
              message: info.registryValueType ? `Type: ${info.registryValueType}` : 'Value data:',
              confirmText: 'Save',
              defaultValue: current.content ?? '',
              historyKey: 'reassembly.registry.editValue',
            });
            if (value === null) return;
            await this.api.writeFile(context.targetPath, value, 'utf8');
          },
        });

        api.registerContextMenuItem({
          id: 'builtin.registry.delete',
          label: 'Delete Registry Entry',
          score: context => isRegistry(context.targetPath) && context.targetPath !== context.currentDirectory ? 85 : 0,
          execute: async context => {
            const confirmed = await this.confirmDialog.confirm({
              title: 'Delete Registry Entry',
              message: `Delete ${context.targetPath}?`,
              confirmText: 'Delete',
              danger: true,
            });
            if (!confirmed) return;
            await this.api.batchDelete([context.targetPath], true, false);
          },
        });
      },
      onUnregister: api => {
        api.unregisterContextMenuItem('builtin.registry.delete');
        api.unregisterContextMenuItem('builtin.registry.editValue');
        api.unregisterContextMenuItem('builtin.registry.newValue');
        api.unregisterContextMenuItem('builtin.registry.newKey');
        api.unregisterPanelColumn('builtin.registry.data');
        api.unregisterPanelColumn('builtin.registry.type');
        api.unregisterPanelColumn('builtin.registry.kind');
        api.unregisterPanelSource(source.id);
      },
    }, {
      id: 'builtin.registry',
      name: 'Windows Registry',
      version: '1.0.0',
      description: 'Built-in Registry panel-source provider and Registry actions.',
      schemes: ['registry'],
    });
  }

  private registerBuiltinAutorunProvider(): void {
    const source = this.autorunSource;
    const isEntry = (path: string): boolean => /^autorun:\/\/entry\//i.test(path ?? '');
    const recordFor = async (path: string) => {
      const id = source.decodeEntryPath(path);
      if (!id) return null;
      const records = await source.records();
      return records.find(record => record.id === id) ?? null;
    };

    this.registerBackend({
      id: 'builtin.autorun',
      onRegister: api => {
        api.registerPanelSource(source);
        api.registerPanelColumn({
          id: 'builtin.autorun.enabled', label: 'Enabled', width: '82px',
          score: context => context.sourceId === source.id ? 100 : 0,
          value: entry => (entry as any).autorunEnabled === undefined ? '' : ((entry as any).autorunEnabled ? 'Yes' : 'No'),
        });
        api.registerPanelColumn({
          id: 'builtin.autorun.origin', label: 'Source', width: '120px',
          score: context => context.sourceId === source.id ? 96 : 0,
          value: entry => String((entry as any).autorunOrigin ?? '').replaceAll('-', ' '),
        });
        api.registerPanelColumn({
          id: 'builtin.autorun.scope', label: 'Scope', width: '92px',
          score: context => context.sourceId === source.id ? 94 : 0,
          value: entry => (entry as any).autorunScope === 'machine' ? 'All Users' : ((entry as any).autorunScope ? 'Current User' : ''),
        });
        api.registerPanelColumn({
          id: 'builtin.autorun.command', label: 'Command',
          score: context => context.sourceId === source.id ? 90 : 0,
          value: entry => (entry as any).autorunCommand ?? '',
        });

        api.registerContextMenuItem({
          id: 'builtin.autorun.toggle',
          label: 'Enable / Disable Autorun',
          score: context => isEntry(context.targetPath) ? 100 : 0,
          execute: async context => {
            const record = await recordFor(context.targetPath); if (!record) return;
            await source.setEnabled(record.id, !record.enabled);
          },
        });
        api.registerContextMenuItem({
          id: 'builtin.autorun.openExecutable',
          label: 'Open Autorun Target',
          score: context => isEntry(context.targetPath) ? 92 : 0,
          execute: async context => {
            const record = await recordFor(context.targetPath); if (!record?.executable) return;
            await this.api.openFile(record.executable);
          },
        });
        api.registerContextMenuItem({
          id: 'builtin.autorun.delete',
          label: 'Delete Autorun Entry',
          score: context => isEntry(context.targetPath) ? 88 : 0,
          execute: async context => {
            const record = await recordFor(context.targetPath); if (!record) return;
            const confirmed = await this.confirmDialog.confirm({
              title: 'Delete Autorun Entry',
              message: `Permanently remove ${record.name} from ${record.sourcePath}?`,
              confirmText: 'Delete', danger: true,
            });
            if (confirmed) await source.delete(record.id);
          },
        });
      },
      onUnregister: api => {
        api.unregisterContextMenuItem('builtin.autorun.delete');
        api.unregisterContextMenuItem('builtin.autorun.openExecutable');
        api.unregisterContextMenuItem('builtin.autorun.toggle');
        api.unregisterPanelColumn('builtin.autorun.command');
        api.unregisterPanelColumn('builtin.autorun.scope');
        api.unregisterPanelColumn('builtin.autorun.origin');
        api.unregisterPanelColumn('builtin.autorun.enabled');
        api.unregisterPanelSource(source.id);
      },
    }, {
      id: 'builtin.autorun', name: 'Windows Autorun', version: '1.0.0',
      description: 'Aggregates Registry Run/RunOnce and Startup folders into autorun://.',
      schemes: ['autorun'],
    });
  }

  private registerBuiltinSystemProviders(): void {
    for (const descriptor of SYSTEM_SURFACES) {
      const source = new SystemSurfacePanelSource(descriptor);
      this.registerBackend({
        id: source.id,
        onRegister: api => {
          api.registerPanelSource(source);
          for (const column of descriptor.columns) {
            api.registerPanelColumn({
              id: `${source.id}.${column.id}`,
              label: column.label,
              width: column.width,
              score: context => context.sourceId === source.id ? 100 : 0,
              value: entry => systemField(entry, column.field),
            });
          }
        },
        onUnregister: api => {
          for (const column of descriptor.columns) {
            api.unregisterPanelColumn(`${source.id}.${column.id}`);
          }
          api.unregisterPanelSource(source.id);
        },
      }, {
        id: source.id,
        name: descriptor.title,
        version: '1.0.0',
        description: `${descriptor.title} system surface provider.`,
        schemes: [descriptor.kind],
      });
    }
  }

  private registerBuiltinRemoteProviders(): void {
    const providers = [
      { id: 'builtin.remote.smb', name: 'SMB', scheme: 'smb' },
      { id: 'builtin.remote.s3', name: 'S3', scheme: 's3' },
      { id: 'builtin.remote.gdrive', name: 'Google Drive', scheme: 'gdrive' },
      { id: 'builtin.remote.dropbox', name: 'Dropbox', scheme: 'dropbox' },
      { id: 'builtin.remote.onedrive', name: 'OneDrive', scheme: 'onedrive' },
    ] as const;

    for (const provider of providers) {
      const source = new RemotePanelSource(
        this.directories,
        this.remoteConnections,
        this.resilience,
        provider.id,
        provider.name,
        [provider.scheme],
      );
      this.registerBackend({
        id: provider.id,
        onRegister: api => api.registerPanelSource(source),
        onUnregister: api => api.unregisterPanelSource(source.id),
      }, {
        id: provider.id,
        name: provider.name,
        version: '1.0.0',
        description: `Built-in ${provider.name} panel-source provider.`,
        schemes: [provider.scheme],
      });
    }
  }

  private contributionOwner(): string {
    return this.registeringBackendId ?? 'host';
  }

  private commandOwner(commandId: string): string {
    return `plugin-command:${commandId}`;
  }

}