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

export interface DeclarativePluginModule {
  schemaVersion: 1;
  commands?: DeclarativePluginCommand[];
  toolbar?: DeclarativePluginPlacement[];
  contextMenu?: DeclarativePluginPlacement[];
}
