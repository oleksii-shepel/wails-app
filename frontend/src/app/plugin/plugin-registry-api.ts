import type { ViewerContribution, ViewerRequest } from '../viewers/viewer-contribution';
import type { SurfaceContribution, SurfaceRequest } from '../surfaces/surface-contribution';
import type { CommandDescriptor, PluginInputContribution } from '../services/input-system.types';
import type { RelatedLocationContribution } from '../services/related-location.types';
import type { WorkspaceSurfaceActionContribution } from '../services/workspace-surface-action.types';
import type { FileInfo } from '../shared/protocol-enhanced';
import type { ResultsPanelMetadata } from '../services/results-panel.service';
import type { PanelSource, PanelSourceKind } from '../services/panel-source';

export interface PluginDescriptor {
  id: string;
  name: string;
  version: string;
  description?: string;
  author?: string;
  schemes?: string[];
  extensions?: string[];
  commands?: PluginCommandInfo[];
}


export interface PluginResultsCollection {
  /** Host-owned when called through a ScopedPluginRegistry. */
  ownerPluginId?: string;
  id: string;
  title: string;
  entries: readonly FileInfo[];
  metadata?: ResultsPanelMetadata;
}

export interface PanelColumnContext {
  readonly currentDirectory: string;
  readonly sourceId: string;
  readonly sourceKind: PanelSourceKind;
}

export interface PanelColumnContribution {
  readonly id: string;
  readonly label: string;
  readonly width?: string;
  score(context: PanelColumnContext): number;
  value(entry: FileInfo, context: PanelColumnContext): string | number | null | undefined;
}

export interface PluginCommandInfo {
  id: string;
  label: string;
  icon?: string;
  hotkey?: string;
  appliesTo?: string[];
}

export interface ToolbarActionContribution {
  id: string;
  /** Optional when commandId points at a registered command descriptor. */
  label?: string;
  icon?: string;
  shortcut?: string;
  /** Preferred execution path. Command metadata and enabled state come from the command registry. */
  commandId?: string;
  /** Compatibility path for existing plugins that have not registered a command descriptor yet. */
  execute?(context: ToolbarContext): void | Promise<void>;
  /** Score: 0 = not applicable; higher wins. */
  score(context: ToolbarContext): number;
}

export interface ToolbarContext {
  selectedPaths: string[];
  currentDirectory: string;
}

export interface ContextMenuContribution {
  id: string;
  /** Optional when commandId points at a registered command descriptor. */
  label?: string;
  icon?: string;
  shortcut?: string;
  /** Preferred execution path. Command metadata and enabled state come from the command registry. */
  commandId?: string;
  /** Compatibility path for existing callback-based contributions. */
  execute?(context: MenuContext): void | Promise<void>;
  /** Score: 0 = not applicable; higher wins. */
  score(context: MenuContext): number;
}

export interface MenuContext {
  targetPath: string;
  selectedPaths: string[];
  currentDirectory: string;
}


export interface FileCategoryResolver {
  id: string;
  resolve(path: string, mimeType?: string | null): FileCategory | null;
}

export interface FileCategory {
  category: string;
  icon?: string;
  color?: string;
}

export interface PluginRegistryAPI {
  /** Preferred: register commands, contexts, and default bindings as one owned unit. */
  registerInput(contribution: PluginInputContribution): void;
  unregisterInput(pluginId: string): void;

  /** Compatibility for command-only plugins. Prefer registerInput for new plugins. */
  registerCommand(command: CommandDescriptor): void;
  unregisterCommand(id: string): void;

  registerViewer(contribution: ViewerContribution): void;
  unregisterViewer(id: string): void;

  registerSurface(contribution: SurfaceContribution): void;
  unregisterSurface(id: string): void;

  registerWorkspaceSurfaceAction(
    contribution: WorkspaceSurfaceActionContribution,
  ): void;
  unregisterWorkspaceSurfaceAction(id: string): void;

  registerToolbarAction(action: ToolbarActionContribution): void;
  unregisterToolbarAction(id: string): void;

  registerContextMenuItem(item: ContextMenuContribution): void;
  unregisterContextMenuItem(id: string): void;

  registerFileCategory(resolver: FileCategoryResolver): void;
  unregisterFileCategory(id: string): void;

  registerRelatedLocation(contribution: RelatedLocationContribution): void;
  unregisterRelatedLocation(id: string): void;

  registerPanelSource(source: PanelSource): void;
  unregisterPanelSource(id: string): void;

  registerPanelColumn(column: PanelColumnContribution): void;
  unregisterPanelColumn(id: string): void;

  createResultsCollection(collection: PluginResultsCollection): string;
  removeResultsCollection(path: string): void;
}