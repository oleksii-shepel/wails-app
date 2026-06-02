import type { Type } from '@angular/core';
import type { ViewerContribution, ViewerRequest } from '../viewers/viewer-contribution';
import type { SurfaceContribution, SurfaceRequest } from '../surfaces/surface-contribution';

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
  label: string;
  icon?: string;
  shortcut?: string;
  /** Score: 0 = not applicable; higher wins. */
  score(context: ToolbarContext): number;
  execute(context: ToolbarContext): void | Promise<void>;
}

export interface ToolbarContext {
  selectedPaths: string[];
  currentDirectory: string;
}

export interface ContextMenuContribution {
  id: string;
  label: string;
  icon?: string;
  /** Score: 0 = not applicable; higher wins. */
  score(context: MenuContext): number;
  execute(context: MenuContext): void | Promise<void>;
}

export interface MenuContext {
  targetPath: string;
  selectedPaths: string[];
  currentDirectory: string;
}

export interface SettingsPanelContribution {
  id: string;
  title: string;
  icon?: string;
  load(): Promise<Type<unknown>> | Type<unknown>;
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
  registerViewer(contribution: ViewerContribution): void;
  unregisterViewer(id: string): void;

  registerSurface(contribution: SurfaceContribution): void;
  unregisterSurface(id: string): void;

  registerToolbarAction(action: ToolbarActionContribution): void;
  unregisterToolbarAction(id: string): void;

  registerContextMenuItem(item: ContextMenuContribution): void;
  unregisterContextMenuItem(id: string): void;

  registerSettingsPanel(panel: SettingsPanelContribution): void;
  unregisterSettingsPanel(id: string): void;

  registerFileCategory(resolver: FileCategoryResolver): void;
  unregisterFileCategory(id: string): void;
}
