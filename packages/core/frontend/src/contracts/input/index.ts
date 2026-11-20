export type InputSourceKind = 'default' | 'plugin' | 'workspace' | 'user';

export type KeyModifier = 'ctrl' | 'shift' | 'alt' | 'meta';

export interface KeyStroke {
  readonly key: string;
  modifiers?: KeyModifier[];
}

export interface KeyChord {
  readonly steps: KeyStroke[];
}

export interface MouseGesture {
  readonly type: 'click' | 'doubleClick' | 'mousedown' | 'mouseup' | 'drag' | 'wheel';
  readonly button?: 'left' | 'middle' | 'right';
  readonly direction?: 'up' | 'down' | 'left' | 'right';
  modifiers?: KeyModifier[];
}

export type InputTrigger =
  | { readonly device: 'keyboard'; readonly chord: KeyChord }
  | { readonly device: 'mouse'; readonly gesture: MouseGesture };

export interface InputContextDefinition {
  readonly id: string;
  readonly kind?: 'global' | 'pane' | 'overlay' | 'dialog' | 'viewer' | (string & {});
  readonly parentId?: string;
  readonly priority: number;
  readonly exclusive?: boolean;
  readonly description?: string;
}

export interface InputBindingDefinition {
  readonly id: string;
  readonly commandId: string;
  readonly contextId: string;
  trigger: InputTrigger;
  readonly when?: string;
  readonly priority?: number;
  readonly description: string;
  readonly category?: string;
  readonly source: InputSourceKind;
  readonly enabled?: boolean;
  readonly preventDefault?: boolean;
  readonly stopPropagation?: boolean;
}

export interface BindingResolutionScore {
  readonly contextPriority: number;
  readonly sourcePriority: number;
  readonly bindingPriority: number;
}

export interface BindingResolution {
  readonly binding: InputBindingDefinition;
  readonly score?: BindingResolutionScore;
}

export interface DialogBindingCatalogEntry {
  readonly context: InputContextDefinition;
  readonly bindings: readonly InputBindingDefinition[];
}

export type CommandInvocationSource =
  | 'keyboard'
  | 'mouse'
  | 'menu'
  | 'toolbar'
  | 'context-menu'
  | 'palette'
  | 'plugin'
  | 'console'
  | 'programmatic'
  | (string & {});

export interface CommandInvocation {
  readonly source: CommandInvocationSource;
  readonly args?: Readonly<Record<string, unknown>>;
  readonly context?: Readonly<Record<string, unknown>>;
}

export type InputCommandHandler = (invocation?: CommandInvocation) => void | Promise<void>;

export interface CommandDefinition {
  readonly id: string;
  readonly title: string;
  readonly description?: string;
  readonly category?: string;
  readonly icon?: string;
  readonly name?: string;
  readonly aliases?: readonly string[];
  readonly contextIds?: readonly string[];
}

export interface CommandDescriptor extends CommandDefinition {
  readonly execute: InputCommandHandler;
  readonly isEnabled?: (invocation?: CommandInvocation) => boolean;
}

export interface CommandState extends CommandDefinition {
  readonly registered: boolean;
  readonly enabled: boolean;
}

export interface CommandCatalogEntry extends CommandState {
  readonly preferredBinding?: InputBindingDefinition;
}

export interface InputBindingConflict {
  readonly bindingId: string;
  readonly conflictingBindingId: string;
  readonly contextId?: string;
  readonly triggerLabel?: string;
  readonly winnerBindingId?: string;
  readonly loserBindingId?: string;
  readonly reason?: string;
}

export interface InputBindingCustomization {
  readonly id: string;
  readonly trigger?: InputTrigger;
  readonly enabled?: boolean;
}

export interface PluginInputContribution {
  readonly pluginId: string;
  readonly contexts?: readonly InputContextDefinition[];
  readonly commands: readonly CommandDescriptor[];
  readonly bindings?: readonly InputBindingDefinition[];
}
