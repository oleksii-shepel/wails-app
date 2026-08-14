export type InputContextKind = 'global' | 'pane' | 'dialog' | 'overlay' | 'widget';

export type InputSourceKind = 'default' | 'workspace' | 'user' | 'plugin';

export type InputDeviceKind = 'keyboard' | 'mouse';

export interface KeyStroke {
  key: string;
  modifiers?: Array<'ctrl' | 'shift' | 'alt' | 'meta'>;
}

export interface KeyChord {
  steps: KeyStroke[];
}

export interface MouseGesture {
  type:
    | 'click'
    | 'doubleClick'
    | 'mousedown'
    | 'mouseup'
    | 'drag'
    | 'wheel';
  button?: 'left' | 'middle' | 'right';
  modifiers?: Array<'ctrl' | 'shift' | 'alt' | 'meta'>;
  direction?: 'up' | 'down' | 'left' | 'right';
}

export type InputTrigger =
  | { device: 'keyboard'; chord: KeyChord }
  | { device: 'mouse'; gesture: MouseGesture };

export interface InputContextDefinition {
  id: string;
  kind: InputContextKind;
  priority: number;
  parentId?: string;
  description: string;
}

export interface InputBindingDefinition {
  id: string;
  commandId: string;
  trigger: InputTrigger;
  contextId: string;
  source: InputSourceKind;
  priority?: number;
  when?: string;
  description: string;
  /** Display category used by the keybinding editor (e.g. 'Navigation', 'File Operations'). */
  category?: string;
  /** Runtime enable/disable state reflected in snapshots. */
  enabled?: boolean;
  preventDefault?: boolean;
  stopPropagation?: boolean;
}

export interface CommandDefinition {
  id: string;
  title: string;
  description?: string;
  category?: string;
  icon?: string;
  /** Only show in command palette when these contexts are active. Empty = always show. */
  contextIds?: string[];
}

export type InputCommandHandler = () => void | Promise<void>;

/**
 * Runtime command descriptor. Keeps discoverable command metadata and execution
 * together while allowing availability to stay context-driven.
 */
export interface CommandDescriptor extends CommandDefinition {
  execute: InputCommandHandler;
  isEnabled?: () => boolean;
}

export interface CommandState extends CommandDefinition {
  enabled: boolean;
  registered: boolean;
}

/** Normalized command metadata for discovery UIs such as the palette or console. */
export interface CommandCatalogEntry extends CommandState {
  preferredBinding?: InputBindingDefinition;
}

/**
 * An owned plugin input bundle. Commands, contexts, and default bindings are
 * registered and removed together under pluginId.
 */
export interface PluginInputContribution {
  pluginId: string;
  commands: CommandDescriptor[];
  contexts?: InputContextDefinition[];
  bindings?: InputBindingDefinition[];
}

/** @deprecated Compatibility shape for the old binding-only plugin API. */
export interface PluginBindingContribution {
  pluginId: string;
  commands?: string[];
  contexts?: InputContextDefinition[];
  bindings: InputBindingDefinition[];
}

export interface DialogBindingCatalogEntry {
  context: InputContextDefinition;
  bindings: InputBindingDefinition[];
}

export interface BindingResolution {
  binding: InputBindingDefinition;
  score: {
    contextPriority: number;
    sourcePriority: number;
    bindingPriority: number;
  };
}