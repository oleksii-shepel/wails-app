export type InputSourceKind = 'default' | 'plugin' | 'workspace' | 'user';

export type InputContextKind = 'global' | 'pane' | 'dialog' | 'overlay' | 'widget';

export interface InputContextDefinition {
  readonly id: string;
  readonly kind: InputContextKind;
  readonly priority: number;
  readonly description?: string;
  readonly parentId?: string;
  readonly exclusive?: boolean;
}

export type KeyModifier = 'ctrl' | 'shift' | 'alt' | 'meta';

export interface KeyStroke {
  readonly key: string;
  readonly code?: string;
  modifiers?: KeyModifier[];
}

export interface KeyChord {
  steps: KeyStroke[];
}

export type MouseButton = 'left' | 'middle' | 'right';
export type MouseGestureKind = 'click' | 'doubleClick' | 'mousedown' | 'mouseup' | 'drag' | 'wheel';

export interface MouseGesture {
  readonly type: MouseGestureKind;
  readonly button?: MouseButton;
  modifiers?: KeyModifier[];
  readonly direction?: 'up' | 'down' | 'left' | 'right';
}

export type InputTrigger =
  | { readonly device: 'keyboard'; readonly chord: KeyChord }
  | { readonly device: 'mouse'; readonly gesture: MouseGesture };

export interface InputBindingDefinition {
  readonly id: string;
  readonly contextId: string;
  readonly commandId: string;
  readonly source: InputSourceKind;
  readonly description: string;
  readonly category?: string;
  readonly when?: string;
  readonly priority?: number;
  readonly preventDefault?: boolean;
  readonly stopPropagation?: boolean;
  readonly enabled?: boolean;
  trigger: InputTrigger;
}

export interface BindingResolutionScore {
  readonly contextPriority: number;
  readonly sourcePriority: number;
  readonly bindingPriority: number;
}

export interface BindingResolution {
  readonly binding: InputBindingDefinition;
  readonly context?: InputContextDefinition;
  readonly score?: BindingResolutionScore;
}

export type CommandInvocationSource =
  | 'keyboard'
  | 'mouse'
  | 'menu'
  | 'context-menu'
  | 'toolbar'
  | 'palette'
  | 'programmatic'
  | 'plugin';

export interface CommandInvocation {
  readonly source: CommandInvocationSource;
  readonly context?: Record<string, unknown>;
  readonly args?: Record<string, unknown>;
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
  readonly contextId: string;
  readonly triggerLabel: string;
  readonly winnerBindingId: string;
  readonly loserBindingId: string;
  readonly reason: 'same-context' | 'shadowed-by-source' | 'shadowed-by-priority' | 'overlapping-context';
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

export interface DialogBindingCatalogEntry {
  readonly context: InputContextDefinition;
  readonly bindings: readonly InputBindingDefinition[];
}
