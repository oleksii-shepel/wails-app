import { Injectable, NgZone } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import { DIALOG_INPUT_CATALOG } from './dialog-input-catalog';
import type {
  BindingResolution,
  InputBindingDefinition,
  InputContextDefinition,
  MouseGesture,
  InputSourceKind,
  KeyStroke,
  PluginInputContribution,
  InputTrigger,
  CommandDefinition,
  CommandDescriptor,
  CommandState,
  CommandCatalogEntry,
  InputCommandHandler,
  CommandInvocation,
} from './input-system.types';

interface RegisteredCommand {
  registrationId: string;
  handler: InputCommandHandler;
  registrationOrder: number;
}

interface RegisteredBinding extends InputBindingDefinition {
  registrationOrder: number;
  enabled: boolean;
}

type KeyboardRegisteredBinding = RegisteredBinding & {
  trigger: Extract<InputTrigger, { device: 'keyboard' }>;
};

function isKeyboardRegisteredBinding(binding: RegisteredBinding): binding is KeyboardRegisteredBinding {
  return binding.trigger.device === 'keyboard';
}

interface PendingChordState {
  startedAt: number;
  steps: KeyStroke[];
  candidateBindingIds: string[];
}

interface ActiveContextState {
  id: string;
  activatedAt: number;
}

interface KeyboardEventContext {
  inInput: boolean;
  inTextarea: boolean;
  inSelect: boolean;
  inContentEditable: boolean;
  inModal: boolean;
  inQuickAccessMenu: boolean;
  hasActivePane: boolean;
  isFilterInput: boolean;
  inFileEntry: boolean;
  inViewerSurface: boolean;
}

export interface ShortcutDebugTrace {
  stroke: string;
  stage: 'matched' | 'no-match' | 'no-command';
  bindingId?: string;
  commandId?: string;
}

export interface SyntheticKeyboardContext {
  inInput?: boolean;
  inTextarea?: boolean;
  inSelect?: boolean;
  inContentEditable?: boolean;
  inModal?: boolean;
  inQuickAccessMenu?: boolean;
  hasActivePane?: boolean;
  isFilterInput?: boolean;
  inFileEntry?: boolean;
  inViewerSurface?: boolean;
}

export interface SyntheticKeyboardEvent {
  key: string;
  code?: string;
  ctrlKey?: boolean;
  shiftKey?: boolean;
  altKey?: boolean;
  metaKey?: boolean;
}

const SOURCE_PRIORITY: Record<InputSourceKind, number> = {
  default: 10,
  plugin: 20,
  workspace: 30,
  user: 40,
};

const CHORD_TIMEOUT_MS = 1800;

function normalizeCommandReference(value: string): string {
  return value.trim().toLowerCase();
}

@Injectable({
  providedIn: 'root'
})
export class InputEngineService {
  private readonly contexts = new Map<string, InputContextDefinition>();
  private readonly bindings = new Map<string, RegisteredBinding>();
  private readonly bindingRegistrations = new Map<string, Map<string, RegisteredBinding>>();
  private readonly commands = new Map<string, Map<string, RegisteredCommand>>();
  private readonly commandDefinitions = new Map<string, CommandDefinition>();
  private readonly commandEnablement = new Map<string, Map<string, (invocation?: CommandInvocation) => boolean>>();
  private readonly pluginInputRegistrations = new Map<string, {
    commandIds: string[];
    bindingIds: string[];
    contextIds: string[];
  }>();
  private readonly activeContexts = new Map<string, ActiveContextState>();

  private pendingChord: PendingChordState | null = null;
  private pendingChordTimer: number | null = null;
  private registrationCounter = 0;
  private commandRegistrationCounter = 0;

  readonly activeContexts$ = createBehaviorSubject<InputContextDefinition[]>([]);
  readonly pendingChord$ = createBehaviorSubject<PendingChordState | null>(null);
  readonly bindings$ = createBehaviorSubject<InputBindingDefinition[]>([]);
  readonly shortcutDebug$ = createBehaviorSubject<ShortcutDebugTrace | null>(null);

  private keyboardEnabled = true;
  readonly showShortcutDebug = typeof window !== 'undefined'
    && ['localhost', '127.0.0.1'].includes(window.location.hostname);

  constructor(
    private readonly ngZone: NgZone,
  ) {
    this.registerContext({
      id: 'global',
      kind: 'global',
      priority: 0,
      description: 'Global application input context',
    });

    for (const entry of DIALOG_INPUT_CATALOG) {
      this.registerContext(entry.context);
      for (const binding of entry.bindings) {
        this.registerBinding(binding);
      }
    }

    this.initializeKeyboardListener();
    this.initializeMouseListeners();
    this.emitBindings();
  }

  registerCommand(commandId: string, handler: InputCommandHandler, ownerId = 'default'): void {
    const registrations = this.commands.get(commandId) ?? new Map<string, RegisteredCommand>();
    registrations.set(ownerId, {
      registrationId: ownerId,
      handler,
      registrationOrder: ++this.commandRegistrationCounter,
    });
    this.commands.set(commandId, registrations);
  }

  unregisterCommand(commandId: string, ownerId = 'default'): void {
    const registrations = this.commands.get(commandId);
    if (!registrations) return;

    registrations.delete(ownerId);
    if (registrations.size === 0) {
      this.commands.delete(commandId);
    }
  }

  registerCommandDescriptor(descriptor: CommandDescriptor, ownerId = 'default'): void {
    const { execute, isEnabled, ...definition } = descriptor;
    this.registerCommandDefinition(definition);
    this.registerCommand(descriptor.id, execute, ownerId);

    if (isEnabled) {
      const registrations = this.commandEnablement.get(descriptor.id) ?? new Map<string, (invocation?: CommandInvocation) => boolean>();
      registrations.set(ownerId, isEnabled);
      this.commandEnablement.set(descriptor.id, registrations);
    }
  }

  unregisterCommandDescriptor(commandId: string, ownerId = 'default'): void {
    this.unregisterCommand(commandId, ownerId);

    const enablement = this.commandEnablement.get(commandId);
    enablement?.delete(ownerId);
    if (enablement?.size === 0) this.commandEnablement.delete(commandId);

    if (!this.commands.has(commandId)) {
      this.unregisterCommandDefinition(commandId);
    }
  }

  isCommandEnabled(commandId: string, invocation?: CommandInvocation): boolean {
    const active = this.getActiveCommandRegistration(commandId);
    if (!active) return false;

    const predicates = this.commandEnablement.get(commandId);
    if (!predicates || predicates.size === 0) return true;

    // Mirror handler precedence: the most recently registered owner wins.
    const predicate = predicates.get(active.registrationId);
    if (!predicate) return true;

    try {
      return predicate(invocation);
    } catch (error) {
      console.error('Input command availability failed:', error);
      return false;
    }
  }

  getCommandState(commandId: string): CommandState | undefined {
    const definition = this.commandDefinitions.get(commandId);
    if (!definition) return undefined;
    return {
      ...definition,
      registered: this.commands.has(commandId),
      enabled: this.isCommandEnabled(commandId),
    };
  }

  getCommandStates(): CommandState[] {
    return Array.from(this.commandDefinitions.keys())
      .map(id => this.getCommandState(id))
      .filter((state): state is CommandState => !!state);
  }

  /**
   * Returns a normalized command catalog for discovery surfaces. The catalog
   * combines definitions, runtime state, and the preferred keyboard binding so
   * consumers do not have to join the input engine's internal registries.
   */
  getCommandCatalog(options: { activeContextsOnly?: boolean } = {}): CommandCatalogEntry[] {
    const commandIds = new Set<string>([
      ...this.commandDefinitions.keys(),
      ...this.commands.keys(),
      ...Array.from(this.bindings.values(), binding => binding.commandId),
    ]);
    const activeContextIds = options.activeContextsOnly ? this.getActiveContextIds() : null;

    return Array.from(commandIds)
      .map(commandId => this.buildCommandCatalogEntry(commandId))
      .filter((entry): entry is CommandCatalogEntry => !!entry)
      .filter(entry => {
        if (!activeContextIds || !entry.contextIds?.length) return true;
        return entry.contextIds.some(contextId => activeContextIds.has(contextId));
      });
  }

  /**
   * Resolve either a stable command ID or a human-facing command name/alias.
   * Ambiguous aliases intentionally resolve to undefined instead of guessing.
   */
  resolveCommandId(reference: string): string | undefined {
    const raw = reference.trim();
    if (!raw) return undefined;
    if (this.commandDefinitions.has(raw) || this.commands.has(raw)) return raw;

    const normalized = normalizeCommandReference(raw);
    let match: string | undefined;

    for (const definition of this.commandDefinitions.values()) {
      const names = [definition.name, ...(definition.aliases ?? [])].filter((value): value is string => !!value);
      if (!names.some(name => normalizeCommandReference(name) === normalized)) continue;
      if (match && match !== definition.id) return undefined;
      match = definition.id;
    }

    return match;
  }

  dispatchCommandByName(
    reference: string,
    invocation: CommandInvocation = { source: 'programmatic' },
  ): boolean {
    const commandId = this.resolveCommandId(reference);
    return commandId ? this.dispatchCommandById(commandId, invocation) : false;
  }

  getPreferredKeyboardBinding(commandId: string): InputBindingDefinition | undefined {
    let preferred: KeyboardRegisteredBinding | undefined;
    for (const binding of this.bindings.values()) {
      if (binding.commandId !== commandId || !isKeyboardRegisteredBinding(binding)) continue;
      if (!preferred || binding.trigger.chord.steps.length < preferred.trigger.chord.steps.length) {
        preferred = binding;
      }
    }
    if (!preferred) return undefined;
    return { ...preferred, enabled: preferred.enabled };
  }

  private buildCommandCatalogEntry(commandId: string): CommandCatalogEntry | undefined {
    const definition = this.commandDefinitions.get(commandId);
    const preferredBinding = this.getPreferredKeyboardBinding(commandId);
    if (!definition && !preferredBinding && !this.commands.has(commandId)) return undefined;

    return {
      id: commandId,
      title: definition?.title ?? preferredBinding?.description ?? commandId,
      description: definition?.description ?? preferredBinding?.description,
      category: definition?.category ?? preferredBinding?.category,
      icon: definition?.icon,
      name: definition?.name,
      aliases: definition?.aliases,
      contextIds: definition?.contextIds ?? (preferredBinding ? [preferredBinding.contextId] : undefined),
      registered: this.commands.has(commandId),
      enabled: this.isCommandEnabled(commandId),
      preferredBinding,
    };
  }

  registerContext(context: InputContextDefinition): void {
    this.contexts.set(context.id, context);
    this.emitActiveContexts();
  }

  activateContext(contextId: string): void {
    const context = this.contexts.get(contextId);
    if (!context) return;
    this.activeContexts.set(contextId, { id: contextId, activatedAt: Date.now() });
    this.emitActiveContexts();
  }

  deactivateContext(contextId: string): void {
    if (!this.activeContexts.delete(contextId)) return;
    this.emitActiveContexts();
    this.clearPendingChord();
  }

  clearTransientContexts(): void {
    for (const [contextId] of this.activeContexts) {
      if (contextId !== 'global') {
        this.activeContexts.delete(contextId);
      }
    }
    this.emitActiveContexts();
    this.clearPendingChord();
  }

  registerBinding(binding: InputBindingDefinition, ownerId = 'default'): void {
    const registrations = this.bindingRegistrations.get(binding.id) ?? new Map<string, RegisteredBinding>();
    registrations.set(ownerId, {
      ...binding,
      preventDefault: binding.preventDefault !== false,
      stopPropagation: binding.stopPropagation !== false,
      enabled: binding.enabled !== false,
      registrationOrder: ++this.registrationCounter,
    });
    this.bindingRegistrations.set(binding.id, registrations);
    this.bindings.set(binding.id, this.getActiveBindingRegistration(registrations));
    this.emitBindings();
  }

  unregisterBinding(bindingId: string, ownerId = 'default'): void {
    const registrations = this.bindingRegistrations.get(bindingId);
    if (!registrations) return;

    registrations.delete(ownerId);
    if (registrations.size === 0) {
      this.bindingRegistrations.delete(bindingId);
      this.bindings.delete(bindingId);
      this.emitBindings();
      return;
    }

    this.bindings.set(bindingId, this.getActiveBindingRegistration(registrations));
    this.emitBindings();
  }

  setBindingEnabled(bindingId: string, enabled: boolean): void {
    const binding = this.bindings.get(bindingId);
    if (!binding) return;
    binding.enabled = enabled;
    this.emitBindings();
  }

  setKeyboardEnabled(enabled: boolean): void {
    this.keyboardEnabled = enabled;
    if (!enabled) {
      this.clearPendingChord();
    }
  }

  /**
   * Register a plugin's command descriptors, contexts and default bindings as
   * one owned unit. Re-registering the same plugin replaces its previous unit.
   */
  registerPluginInput(contribution: PluginInputContribution): void {
    const pluginId = contribution.pluginId.trim();
    if (!pluginId) throw new Error('Plugin input contribution requires pluginId.');

    this.unregisterPluginInput(pluginId);

    const ownerId = this.pluginInputOwner(pluginId);
    const contextIds: string[] = [];
    const commandIds: string[] = [];
    const bindingIds: string[] = [];

    try {
      for (const context of contribution.contexts ?? []) {
        if (this.contexts.has(context.id)) {
          throw new Error(`Plugin ${pluginId} cannot replace input context ${context.id}.`);
        }
        this.registerContext(context);
        contextIds.push(context.id);
      }

      for (const command of contribution.commands) {
        if (this.commands.has(command.id)) {
          throw new Error(`Plugin ${pluginId} cannot replace command ${command.id}.`);
        }
        this.registerCommandDescriptor(command, ownerId);
        commandIds.push(command.id);
      }

      for (const binding of contribution.bindings ?? []) {
        if (this.bindings.has(binding.id)) {
          throw new Error(`Plugin ${pluginId} cannot replace input binding ${binding.id}.`);
        }
        this.registerBinding({ ...binding, source: 'plugin' }, ownerId);
        bindingIds.push(binding.id);
      }

      this.pluginInputRegistrations.set(pluginId, { commandIds, bindingIds, contextIds });
    } catch (error) {
      this.removePluginInputRegistration(ownerId, { commandIds, bindingIds, contextIds });
      throw error;
    }
  }

  unregisterPluginInput(pluginId: string): void {
    const registration = this.pluginInputRegistrations.get(pluginId);
    if (!registration) return;

    this.removePluginInputRegistration(this.pluginInputOwner(pluginId), registration);
    this.pluginInputRegistrations.delete(pluginId);
  }


  private pluginInputOwner(pluginId: string): string {
    return `plugin-input:${pluginId}`;
  }

  private removePluginInputRegistration(
    ownerId: string,
    registration: { commandIds: string[]; bindingIds: string[]; contextIds: string[] },
  ): void {
    for (const bindingId of registration.bindingIds) {
      this.unregisterBinding(bindingId, ownerId);
    }
    for (const commandId of registration.commandIds) {
      this.unregisterCommandDescriptor(commandId, ownerId);
    }
    for (const contextId of registration.contextIds) {
      this.activeContexts.delete(contextId);
      this.contexts.delete(contextId);
    }
    if (registration.contextIds.length > 0) {
      this.emitActiveContexts();
      this.clearPendingChord();
    }
  }

  getBindingSnapshot(): InputBindingDefinition[] {
    return Array.from(this.bindings.values()).map(binding => ({
      ...binding,
      enabled: binding.enabled,
    }));
  }

  getBinding(bindingId: string): InputBindingDefinition | undefined {
    const binding = this.bindings.get(bindingId);
    if (!binding) return undefined;
    return { ...binding, enabled: binding.enabled };
  }

  getRegisteredCommandIds(): string[] {
    return Array.from(this.commands.keys());
  }

  dispatchCommandById(commandId: string, invocation: CommandInvocation = { source: 'programmatic' }): boolean {
    const handler = this.getCommandHandler(commandId, invocation);
    if (!handler) return false;
    return this.executeCommand(handler, invocation);
  }

  updateBindingTrigger(bindingId: string, trigger: InputTrigger): boolean {
    const binding = this.bindings.get(bindingId);
    if (!binding) return false;
    binding.trigger = trigger;
    this.emitBindings();
    return true;
  }

  registerCommandDefinition(def: CommandDefinition): void {
    this.commandDefinitions.set(def.id, def);
  }

  unregisterCommandDefinition(commandId: string): void {
    this.commandDefinitions.delete(commandId);
  }

  getCommandDefinitions(): CommandDefinition[] {
    return Array.from(this.commandDefinitions.values());
  }

  getCommandDefinition(commandId: string): CommandDefinition | undefined {
    return this.commandDefinitions.get(commandId);
  }

  getContextSnapshot(): InputContextDefinition[] {
    return Array.from(this.contexts.values());
  }

  resolveKeyStroke(
    stroke: KeyStroke,
    context?: KeyboardEvent | KeyboardEventContext | SyntheticKeyboardContext | null,
  ): BindingResolution | null {
    const currentSequence = this.pendingChord ? [...this.pendingChord.steps, stroke] : [stroke];
    const candidates = this.matchBindings(currentSequence, this.resolveKeyboardContext(context));

    if (candidates.length === 0) {
      if (this.pendingChord) {
        this.clearPendingChord();
      }
      return null;
    }

    const completeMatches = candidates.filter(candidate =>
      candidate.trigger.device === 'keyboard' &&
      candidate.trigger.chord.steps.length === currentSequence.length,
    );

    const partialMatches = candidates.filter(candidate =>
      candidate.trigger.device === 'keyboard' &&
      candidate.trigger.chord.steps.length > currentSequence.length,
    );

    if (partialMatches.length > 0) {
      this.beginPendingChord(currentSequence, partialMatches.map(binding => binding.id));
    } else {
      this.clearPendingChord();
    }

    if (completeMatches.length === 0) {
      return null;
    }

    return this.chooseBestBinding(completeMatches);
  }

  async dispatchKeyStroke(stroke: KeyStroke, options?: { context?: SyntheticKeyboardContext | null }): Promise<boolean> {
    if (!this.keyboardEnabled) return false;

    const resolution = this.resolveKeyStroke(stroke, options?.context ?? null);
    if (!resolution) {
      this.emitShortcutDebug({
        stroke: this.formatDebugStroke(stroke),
        stage: 'no-match',
      });
      return false;
    }

    const invocation: CommandInvocation = { source: 'keyboard', context: { bindingId: resolution.binding.id } };
    const command = this.getCommandHandler(resolution.binding.commandId, invocation);
    if (!command) {
      this.emitShortcutDebug({
        stroke: this.formatDebugStroke(stroke),
        stage: 'no-command',
        bindingId: resolution.binding.id,
        commandId: resolution.binding.commandId,
      });
      return false;
    }

    this.emitShortcutDebug({
      stroke: this.formatDebugStroke(stroke),
      stage: 'matched',
      bindingId: resolution.binding.id,
      commandId: resolution.binding.commandId,
    });

    return this.executeCommandAsync(command, invocation);
  }

  async dispatchKeyboardEvent(
    event: SyntheticKeyboardEvent,
    options?: { context?: SyntheticKeyboardContext | null },
  ): Promise<boolean> {
    return this.dispatchKeyStroke(this.normalizeKeyStroke(event), options);
  }

  async dispatchMouseGesture(gesture: MouseGesture): Promise<boolean> {
    const resolution = this.resolveMouseGesture(gesture);
    if (!resolution) return false;

    const invocation: CommandInvocation = { source: 'mouse', context: { bindingId: resolution.binding.id } };
    const command = this.getCommandHandler(resolution.binding.commandId, invocation);
    if (!command) return false;

    return this.executeCommandAsync(command, invocation);
  }

  private initializeKeyboardListener(): void {
    const listener = (event: KeyboardEvent) => {
      if (!this.keyboardEnabled || event.isComposing) return;

      const stroke = this.normalizeKeyStroke(event);
      const resolution = this.resolveKeyStroke(stroke, event);
      if (!resolution) {
        this.emitShortcutDebug({
          stroke: this.formatDebugStroke(stroke),
          stage: 'no-match',
        });
        return;
      }

      const invocation: CommandInvocation = { source: 'keyboard', context: { bindingId: resolution.binding.id } };
      const command = this.getCommandHandler(resolution.binding.commandId, invocation);
      if (!command) {
        this.emitShortcutDebug({
          stroke: this.formatDebugStroke(stroke),
          stage: 'no-command',
          bindingId: resolution.binding.id,
          commandId: resolution.binding.commandId,
        });
        return;
      }

      this.emitShortcutDebug({
        stroke: this.formatDebugStroke(stroke),
        stage: 'matched',
        bindingId: resolution.binding.id,
        commandId: resolution.binding.commandId,
      });

      if (resolution.binding.preventDefault !== false) {
        event.preventDefault();
      }
      if (resolution.binding.stopPropagation !== false) {
        event.stopPropagation();
      }

      this.executeCommand(command, invocation);
    };

    // One canonical keyboard boundary for the entire application.
    //
    // Do not register the same listener on document as well. A window capture
    // listener already observes keyboard input before application components,
    // while duplicate capture listeners make unmatched keys and chord state
    // travel through the engine twice.
    window.addEventListener('keydown', listener, { capture: true });
  }

  private initializeMouseListeners(): void {
    const listenerOptions = { capture: true } as AddEventListenerOptions;
    const wheelOptions = { capture: true, passive: false } as AddEventListenerOptions;

    document.addEventListener('click', (event) => this.handleMouseEvent('click', event), listenerOptions);
    document.addEventListener('dblclick', (event) => this.handleMouseEvent('dblclick', event), listenerOptions);
    document.addEventListener('auxclick', (event) => this.handleMouseEvent('auxclick', event), listenerOptions);
    document.addEventListener('contextmenu', (event) => this.handleMouseEvent('contextmenu', event), listenerOptions);
    document.addEventListener('mousedown', (event) => this.handleMouseEvent('mousedown', event), listenerOptions);
    document.addEventListener('mouseup', (event) => this.handleMouseEvent('mouseup', event), listenerOptions);
    document.addEventListener('wheel', (event) => this.handleMouseEvent('wheel', event), wheelOptions);
  }

  private handleMouseEvent(kind: string, event: MouseEvent | WheelEvent): void {
    if (!this.keyboardEnabled) return;

    const gesture = this.normalizeMouseGesture(kind, event);
    if (!gesture) return;

    const resolution = this.resolveMouseGesture(gesture, event);
    if (!resolution) return;

    const invocation: CommandInvocation = { source: 'mouse', context: { bindingId: resolution.binding.id } };
    const command = this.getCommandHandler(resolution.binding.commandId, invocation);
    if (!command) return;

    this.clearPendingChord();

    if (resolution.binding.preventDefault !== false) {
      event.preventDefault();
    }
    if (resolution.binding.stopPropagation !== false) {
      event.stopPropagation();
    }

    this.executeCommand(command, invocation);
  }

  private resolveMouseGesture(gesture: MouseGesture, event?: MouseEvent | WheelEvent): BindingResolution | null {
    const candidates = this.matchMouseBindings(gesture, event);
    if (candidates.length === 0) {
      return null;
    }

    return this.chooseBestBinding(candidates);
  }

  private normalizeKeyStroke(event: SyntheticKeyboardEvent): KeyStroke {
    const modifiers: KeyStroke['modifiers'] = [];
    if (event.ctrlKey || event.metaKey) modifiers.push(event.ctrlKey ? 'ctrl' : 'meta');
    if (event.shiftKey) modifiers.push('shift');
    if (event.altKey) modifiers.push('alt');

    return {
      key: this.normalizeKey(event.key, event.code),
      modifiers: modifiers.length > 0 ? modifiers.sort() : undefined,
    };
  }

  private normalizeKey(key: string, code?: string): string {
    const codeMap: Record<string, string> = {
      NumpadAdd: 'numpadadd',
      NumpadSubtract: 'numpadsubtract',
      NumpadMultiply: 'numpadmultiply',
      // Match OEM punctuation by physical key so shortcuts stay stable across
      // keyboard layouts and WebView2 hosts.
      Slash: '/',
      Comma: ',',
      Period: '.',
      Semicolon: ';',
      Quote: '\'',
      Backquote: '`',
      BracketLeft: '[',
      BracketRight: ']',
      Backslash: '\\',
      Minus: '-',
      Equal: '=',
    };

    if (code && codeMap[code]) {
      return codeMap[code];
    }

    const keyMap: Record<string, string> = {
      ' ': 'space',
      Escape: 'escape',
      Enter: 'enter',
      Tab: 'tab',
      Insert: 'insert',
      Backspace: 'backspace',
      Delete: 'delete',
      Home: 'home',
      End: 'end',
      PageUp: 'pageup',
      PageDown: 'pagedown',
      ArrowUp: 'arrowup',
      ArrowDown: 'arrowdown',
      ArrowLeft: 'arrowleft',
      ArrowRight: 'arrowright',
    };

    const normalizedKey = keyMap[key] ?? key.toLowerCase();
    if (normalizedKey && normalizedKey !== 'unidentified') {
      return normalizedKey;
    }

    // WebView2 may report accelerator combinations (for example Ctrl+Shift+P)
    // with KeyboardEvent.key === "Unidentified" while still preserving the
    // physical DOM code ("KeyP"). Recover the logical key from code only when
    // key itself is unusable. Normal browser keyboard semantics remain unchanged.
    if (code?.startsWith('Key') && code.length === 4) {
      return code.slice(3).toLowerCase();
    }
    if (code?.startsWith('Digit') && code.length === 6) {
      return code.slice(5);
    }
    if (/^F(?:[1-9]|1[0-9]|2[0-4])$/.test(code ?? '')) {
      return code!.toLowerCase();
    }

    return normalizedKey;
  }

  private matchBindings(sequence: KeyStroke[], keyboardContext?: KeyboardEventContext | null): RegisteredBinding[] {
    const activeContextIds = this.getActiveContextIds();
    const editableOwnerContextId = this.editableOwnerContextId(
      keyboardContext,
      activeContextIds,
    );

    return Array.from(this.bindings.values()).filter(binding => {
      if (!binding.enabled || binding.trigger.device !== 'keyboard') return false;
      if (!activeContextIds.has(binding.contextId)) return false;

      // Text controls are sovereign. If a dialog/widget/overlay currently owns
      // the focused editor, only that highest-priority interactive context may
      // consume editing keys. Background pane/global/overlay shortcuts must
      // yield so ordinary text, digits, Delete, arrows, etc. reach the control.
      //
      // Modifier shortcuts remain eligible when no interactive owner exists;
      // this preserves global accelerators in ordinary app text fields.
      if (
        editableOwnerContextId &&
        binding.contextId !== editableOwnerContextId
      ) {
        return false;
      }

      if (!this.whenClauseAllows(binding.when, keyboardContext)) return false;

      const steps = binding.trigger.chord.steps;
      if (steps.length < sequence.length) return false;

      for (let i = 0; i < sequence.length; i++) {
        if (!this.sameStroke(steps[i], sequence[i])) {
          return false;
        }
      }

      return true;
    });
  }

  private editableOwnerContextId(
    keyboardContext: KeyboardEventContext | null | undefined,
    activeContextIds: Set<string>,
  ): string | null {
    if (!this.isTypingContext(keyboardContext)) {
      return null;
    }

    let owner: InputContextDefinition | null = null;

    for (const contextId of activeContextIds) {
      const context = this.contexts.get(contextId);
      if (!context) continue;

      if (
        context.kind !== 'dialog' &&
        context.kind !== 'overlay' &&
        context.kind !== 'widget'
      ) {
        continue;
      }

      if (
        !owner ||
        context.priority > owner.priority ||
        (
          context.priority === owner.priority &&
          (this.activeContexts.get(context.id)?.activatedAt ?? 0) >
            (this.activeContexts.get(owner.id)?.activatedAt ?? 0)
        )
      ) {
        owner = context;
      }
    }

    return owner?.id ?? null;
  }

  private isTypingContext(
    context: KeyboardEventContext | null | undefined,
  ): boolean {
    return !!context && (
      context.inInput ||
      context.inTextarea ||
      context.inSelect ||
      context.inContentEditable
    );
  }

  private matchMouseBindings(gesture: MouseGesture, event?: MouseEvent | WheelEvent): RegisteredBinding[] {
    const activeContextIds = this.getActiveContextIds();
    const mouseContext = event ? this.getKeyboardEventContext(event) : null;

    return Array.from(this.bindings.values()).filter(binding => {
      if (!binding.enabled || binding.trigger.device !== 'mouse') return false;
      if (!activeContextIds.has(binding.contextId)) return false;
      if (!this.whenClauseAllows(binding.when, mouseContext)) return false;

      return this.sameGesture(binding.trigger.gesture, gesture);
    });
  }

  private chooseBestBinding(bindings: RegisteredBinding[]): BindingResolution {
    const sorted = [...bindings].sort((left, right) => {
      const leftContextPriority = this.contexts.get(left.contextId)?.priority ?? 0;
      const rightContextPriority = this.contexts.get(right.contextId)?.priority ?? 0;
      if (leftContextPriority !== rightContextPriority) {
        return rightContextPriority - leftContextPriority;
      }

      const leftSourcePriority = SOURCE_PRIORITY[left.source];
      const rightSourcePriority = SOURCE_PRIORITY[right.source];
      if (leftSourcePriority !== rightSourcePriority) {
        return rightSourcePriority - leftSourcePriority;
      }

      const leftBindingPriority = left.priority ?? 0;
      const rightBindingPriority = right.priority ?? 0;
      if (leftBindingPriority !== rightBindingPriority) {
        return rightBindingPriority - leftBindingPriority;
      }

      return right.registrationOrder - left.registrationOrder;
    });

    const best = sorted[0];
    return {
      binding: best,
      score: {
        contextPriority: this.contexts.get(best.contextId)?.priority ?? 0,
        sourcePriority: SOURCE_PRIORITY[best.source],
        bindingPriority: best.priority ?? 0,
      },
    };
  }

  private sameStroke(left: KeyStroke, right: KeyStroke): boolean {
    const leftMods = this.normalizeModifiers(left.modifiers);
    const rightMods = this.normalizeModifiers(right.modifiers);

    return (
      left.key === right.key &&
      leftMods.length === rightMods.length &&
      leftMods.every((modifier, index) => modifier === rightMods[index])
    );
  }

  private normalizeModifiers(
    modifiers: KeyStroke['modifiers'],
  ): NonNullable<KeyStroke['modifiers']> {
    return [...(modifiers ?? [])].sort();
  }

  private whenClauseAllows(expression?: string, context?: KeyboardEventContext | null): boolean {
    if (!expression || expression.trim() === '' || expression === 'true') {
      return true;
    }

    const terms = expression.split('&&').map(term => term.trim()).filter(Boolean);
    if (terms.length === 0) {
      return true;
    }

    return terms.every(term => {
      const isNegated = term.startsWith('!');
      const token = isNegated ? term.slice(1).trim() : term;
      const value = this.resolveWhenToken(token, context);
      return isNegated ? !value : value;
    });
  }

  private resolveWhenToken(token: string, context?: KeyboardEventContext | null): boolean {
    switch (token) {
      case 'typing':
        return this.isTypingContext(context);
      case 'modal':
        return !!context?.inModal;
      case 'quickAccess':
        return !!context?.inQuickAccessMenu;
      case 'activePane':
        return !!context?.hasActivePane;
      case 'filterInput':
        return !!context?.isFilterInput;
      case 'fileEntry':
        return !!context?.inFileEntry;
      case 'viewerSurface':
        return !!context?.inViewerSurface;
      default:
        return false;
    }
  }

  private resolveKeyboardContext(
    source?: KeyboardEvent | KeyboardEventContext | SyntheticKeyboardContext | null,
  ): KeyboardEventContext | null {
    if (!source) {
      return null;
    }
    if ('target' in source) {
      return this.getKeyboardEventContext(source);
    }

    return {
      inInput: !!source.inInput,
      inTextarea: !!source.inTextarea,
      inSelect: !!source.inSelect,
      inContentEditable: !!source.inContentEditable,
      inModal: !!source.inModal,
      inQuickAccessMenu: !!source.inQuickAccessMenu,
      hasActivePane: !!source.hasActivePane,
      isFilterInput: !!source.isFilterInput,
      inFileEntry: !!source.inFileEntry,
      inViewerSurface: !!source.inViewerSurface,
    };
  }

  private getKeyboardEventContext(event: { target: EventTarget | null }): KeyboardEventContext {
    const target = event.target as HTMLElement | null;
    const tagName = target?.tagName.toLowerCase() ?? '';

    return {
      inInput: tagName === 'input',
      inTextarea: tagName === 'textarea',
      inSelect: tagName === 'select',
      inContentEditable: !!target?.isContentEditable,
      inModal: !!target?.closest('.modal, [role="dialog"]'),
      inQuickAccessMenu: !!target?.closest('.ab-quick-access-drop'),
      hasActivePane: !!target?.closest('.pane'),
      isFilterInput: !!target?.classList.contains('filter-input'),
      inFileEntry: !!target?.closest('.file-entry'),
      inViewerSurface: !!target?.closest('.viewer-surface'),
    };
  }

  private normalizeMouseGesture(kind: string, event: MouseEvent | WheelEvent): MouseGesture | null {
    const modifiers: MouseGesture['modifiers'] = [];
    if (event.ctrlKey || event.metaKey) modifiers.push(event.ctrlKey ? 'ctrl' : 'meta');
    if (event.shiftKey) modifiers.push('shift');
    if (event.altKey) modifiers.push('alt');

    switch (kind) {
      case 'click':
        return { type: 'click', button: this.normalizeMouseButton(event.button), modifiers: modifiers.length ? modifiers.sort() : undefined };
      case 'dblclick':
        return { type: 'doubleClick', button: this.normalizeMouseButton(event.button), modifiers: modifiers.length ? modifiers.sort() : undefined };
      case 'auxclick':
      case 'contextmenu':
      case 'mousedown':
      case 'mouseup':
        return {
          type: kind === 'contextmenu' ? 'click' : (kind as 'mousedown' | 'mouseup'),
          button: this.normalizeMouseButton(event.button),
          modifiers: modifiers.length ? modifiers.sort() : undefined,
        };
      case 'wheel': {
        const wheelEvent = event as WheelEvent;
        const direction = Math.abs(wheelEvent.deltaX) > Math.abs(wheelEvent.deltaY)
          ? (wheelEvent.deltaX < 0 ? 'left' : 'right')
          : (wheelEvent.deltaY < 0 ? 'up' : 'down');
        return {
          type: 'wheel',
          modifiers: modifiers.length ? modifiers.sort() : undefined,
          direction,
        };
      }
      default:
        return null;
    }
  }

  private normalizeMouseButton(button: number): MouseGesture['button'] {
    switch (button) {
      case 1:
        return 'middle';
      case 2:
        return 'right';
      case 0:
      default:
        return 'left';
    }
  }

  private sameGesture(left: MouseGesture, right: MouseGesture): boolean {
    const leftMods = (left.modifiers ?? []).join('+');
    const rightMods = (right.modifiers ?? []).join('+');
    return left.type === right.type
      && (left.button ?? null) === (right.button ?? null)
      && (left.direction ?? null) === (right.direction ?? null)
      && leftMods === rightMods;
  }

  getActiveContextIds(): Set<string> {
    const ids = new Set<string>(['global']);
    for (const contextId of this.activeContexts.keys()) {
      ids.add(contextId);
    }
    return ids;
  }

  private beginPendingChord(steps: KeyStroke[], candidateBindingIds: string[]): void {
    this.clearPendingChordTimer();
    this.pendingChord = {
      startedAt: Date.now(),
      steps,
      candidateBindingIds,
    };
    this.pendingChord$.next(this.pendingChord);
    this.pendingChordTimer = window.setTimeout(() => {
      this.clearPendingChord();
    }, CHORD_TIMEOUT_MS);
  }

  private clearPendingChord(): void {
    this.clearPendingChordTimer();
    this.pendingChord = null;
    this.pendingChord$.next(null);
  }

  private clearPendingChordTimer(): void {
    if (this.pendingChordTimer === null) return;
    window.clearTimeout(this.pendingChordTimer);
    this.pendingChordTimer = null;
  }

  private emitBindings(): void {
    this.bindings$.next(this.getBindingSnapshot());
  }

  private getCommandHandler(commandId: string, invocation?: CommandInvocation): InputCommandHandler | null {
    if (!this.isCommandEnabled(commandId, invocation)) return null;
    return this.getActiveCommandRegistration(commandId)?.handler ?? null;
  }

  private emitShortcutDebug(trace: ShortcutDebugTrace): void {
    if (!this.showShortcutDebug) return;
    this.ngZone.run(() => {
      this.shortcutDebug$.next(trace);
    });
  }

  private formatDebugStroke(stroke: KeyStroke): string {
    const parts: string[] = [...(stroke.modifiers ?? []).map(modifier => {
      switch (modifier) {
        case 'ctrl': return 'Ctrl';
        case 'shift': return 'Shift';
        case 'alt': return 'Alt';
        case 'meta': return 'Meta';
        default: return modifier;
      }
    })];
    parts.push(stroke.key);
    return parts.join('+');
  }

  private getActiveBindingRegistration(registrations: Map<string, RegisteredBinding>): RegisteredBinding {
    let activeRegistration: RegisteredBinding | null = null;
    for (const registration of registrations.values()) {
      if (!activeRegistration || registration.registrationOrder > activeRegistration.registrationOrder) {
        activeRegistration = registration;
      }
    }

    if (!activeRegistration) {
      throw new Error('Binding registration set is unexpectedly empty.');
    }

    return activeRegistration;
  }

  private executeCommand(command: InputCommandHandler, invocation: CommandInvocation): boolean {
    try {
      this.ngZone.run(() => {
        const result = command(invocation);
        if (result instanceof Promise) {
          result.catch(error => console.error('Input command failed:', error));
        }
      });
      return true;
    } catch (error) {
      console.error('Input command failed:', error);
      return false;
    }
  }

  private async executeCommandAsync(command: InputCommandHandler, invocation: CommandInvocation): Promise<boolean> {
    try {
      await this.ngZone.run(async () => {
        await command(invocation);
      });
      return true;
    } catch (error) {
      console.error('Input command failed:', error);
      return false;
    }
  }

  private getActiveCommandRegistration(commandId: string): RegisteredCommand | null {
    const registrations = this.commands.get(commandId);
    if (!registrations || registrations.size === 0) return null;

    let activeRegistration: RegisteredCommand | null = null;
    for (const registration of registrations.values()) {
      if (!activeRegistration || registration.registrationOrder > activeRegistration.registrationOrder) {
        activeRegistration = registration;
      }
    }
    return activeRegistration;
  }

  private emitActiveContexts(): void {
    const active = Array.from(this.activeContexts.values())
      .sort((left, right) => right.activatedAt - left.activatedAt)
      .map(entry => this.contexts.get(entry.id))
      .filter((context): context is InputContextDefinition => !!context);
    this.activeContexts$.next(active);
  }
}