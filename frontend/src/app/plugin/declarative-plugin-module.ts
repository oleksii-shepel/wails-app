import type { InputTrigger } from '../services/input-system.types';

export interface DeclarativePluginCommand {
  id: string;
  title?: string;
  name?: string;
  aliases?: string[];
  targetCommandId: string;
}

export type DeclarativePlacementWhen = 'always' | 'selection' | 'single-selection';

export interface DeclarativePluginPlacement {
  id: string;
  commandId: string;
  when?: DeclarativePlacementWhen;
}

export interface DeclarativePluginBinding {
  id: string;
  commandId: string;
  trigger: InputTrigger;
  contextId?: string;
  when?: string;
  priority?: number;
  description?: string;
  category?: string;
}

export interface DeclarativePluginModule {
  schemaVersion: 1;
  commands?: DeclarativePluginCommand[];
  bindings?: DeclarativePluginBinding[];
  toolbar?: DeclarativePluginPlacement[];
  contextMenu?: DeclarativePluginPlacement[];
}