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

  /**
   * When active, bindings from lower non-global contexts are hidden.
   * Global bindings remain available for application-wide commands such as F1.
   */
  exclusive?: boolean;
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
  /** Canonical human-facing name used by interactive invocation surfaces. */
  name?: string;
  /** Additional human-facing names. Keep aliases globally unambiguous. */
  aliases?: string[];
  /** Only show in command palette when these contexts are active. Empty = always show. */
  contextIds?: string[];
}

export type CommandInvocationSource =
  | 'keyboard'
  | 'mouse'
  | 'palette'
  | 'toolbar'
  | 'context-menu'
  | 'console'
  | 'api'
  | 'programmatic';

export type CommandArguments = Readonly<Record<string, unknown>>;

/** Structured command invocation shared by every execution surface. */
export interface CommandInvocation {
  source: CommandInvocationSource;
  args?: CommandArguments;
  context?: Readonly<Record<string, unknown>>;
}

export type InputCommandHandler = (invocation?: CommandInvocation) => void | Promise<void>;

/**
 * Runtime command descriptor. Keeps discoverable command metadata and execution
 * together while allowing availability to stay context-driven.
 */
export interface CommandDescriptor extends CommandDefinition {
  execute: InputCommandHandler;
  isEnabled?: (invocation?: CommandInvocation) => boolean;
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

export interface InputBindingConflict {
  readonly bindingId: string;
  readonly conflictingBindingId: string;
  readonly contextId: string;
  readonly triggerLabel: string;
  readonly winnerBindingId: string;
  readonly loserBindingId: string;
  readonly reason:
    | 'same-context'
    | 'overlapping-context'
    | 'shadowed-by-source'
    | 'shadowed-by-priority';
}

export interface InputBindingCustomization {
  readonly id: string;
  readonly trigger?: InputTrigger;
  readonly enabled?: boolean;
}