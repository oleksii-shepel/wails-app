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

export interface PluginInputContribution {
  pluginId: string;
  commands: string[];
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