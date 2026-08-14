import type { ViewerContribution, ViewerRequest } from '../viewers/viewer-contribution';
import type { SurfaceContribution, SurfaceRequest } from '../surfaces/surface-contribution';
import type { CommandDescriptor, PluginInputContribution } from '../services/input-system.types';

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

  registerToolbarAction(action: ToolbarActionContribution): void;
  unregisterToolbarAction(id: string): void;

  registerContextMenuItem(item: ContextMenuContribution): void;
  unregisterContextMenuItem(id: string): void;

  registerFileCategory(resolver: FileCategoryResolver): void;
  unregisterFileCategory(id: string): void;
}