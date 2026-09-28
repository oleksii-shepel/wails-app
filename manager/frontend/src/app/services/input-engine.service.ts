import { Injectable, NgZone } from '@angular/core';
import { CommandRegistryService } from './command-registry.service';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import { BUILTIN_INPUT_CATALOG } from './dialog-input-catalog';
import { compareStableBindingIdentity, whenClausesCanOverlap } from './input-conflict-policy';
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
  InputBindingConflict,
  InputBindingCustomization,
} from './input-system.types';

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
  inContextMenu: boolean;
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
  inContextMenu?: boolean;
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
const SCROLL_SURFACE_SELECTOR = '.app-scroll-surface';
const SCROLL_SURFACE_EXTRA_FACTOR = 0.15;


@Injectable({
  providedIn: 'root'
})
export class InputEngineService {
  private readonly contexts = new Map<string, InputContextDefinition>();
  private readonly bindings = new Map<string, RegisteredBinding>();
  private readonly bindingRegistrations = new Map<string, Map<string, RegisteredBinding>>();
  private readonly pluginInputRegistrations = new Map<string, {
    commandIds: string[];
    bindingIds: string[];
    contextIds: string[];
  }>();
  private readonly activeContexts = new Map<string, ActiveContextState>();

  private pendingChord: PendingChordState | null = null;
  private pendingChordTimer: number | null = null;
  private mouseDragState: {
    startX: number;
    startY: number;
    button: MouseGesture['button'];
    modifiers?: MouseGesture['modifiers'];
    emitted: boolean;
    target: EventTarget | null;
  } | null = null;
  private registrationCounter = 0;

  readonly activeContexts$ = createBehaviorSubject<InputContextDefinition[]>([]);
  readonly pendingChord$ = createBehaviorSubject<PendingChordState | null>(null);
  readonly bindings$ = createBehaviorSubject<InputBindingDefinition[]>([]);
  readonly shortcutDebug$ = createBehaviorSubject<ShortcutDebugTrace | null>(null);

  private keyboardEnabled = true;
  private readonly handledNativeKeyboardEvents = new WeakSet<KeyboardEvent>();
  readonly showShortcutDebug = typeof window !== 'undefined'
    && ['localhost', '127.0.0.1'].includes(window.location.hostname);

  constructor(
    private readonly ngZone: NgZone,
    private readonly commandRegistry: CommandRegistryService,
  ) {
    this.registerContext({
      id: 'global',
      kind: 'global',
      priority: 0,
      description: 'Global application input context',
    });

    for (const entry of BUILTIN_INPUT_CATALOG) {
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
    this.commandRegistry.register(commandId, handler, ownerId);
  }

  registerCommands(entries: ReadonlyArray<{ commandId: string; handler: InputCommandHandler }>, ownerId = 'default'): void {
    this.commandRegistry.registerBatch(entries.map(entry => ({ ...entry, ownerId })));
  }

  unregisterCommand(commandId: string, ownerId = 'default'): void {
    this.commandRegistry.unregister(commandId, ownerId);
  }

  unregisterCommands(commandIds: readonly string[], ownerId = 'default'): void {
    this.commandRegistry.unregisterBatch(commandIds.map(commandId => ({ commandId, ownerId })));
  }

  registerCommandDescriptor(descriptor: CommandDescriptor, ownerId = 'default'): void {
    this.commandRegistry.registerDescriptor(descriptor, ownerId);
  }

  unregisterCommandDescriptor(commandId: string, ownerId = 'default'): void {
    this.commandRegistry.unregister(commandId, ownerId);
  }

  isCommandEnabled(commandId: string, invocation?: CommandInvocation): boolean {
    return this.commandRegistry.isEnabled(commandId, invocation);
  }

  getCommandState(commandId: string, invocation?: CommandInvocation): CommandState | undefined {
    return this.commandRegistry.getState(commandId, invocation);
  }

  getCommandStates(): CommandState[] {
    return this.commandRegistry.getStates();
  }

  /**
   * Returns a normalized command catalog for discovery surfaces. Input owns the
   * preferred binding; command metadata and runtime state come from the canonical
   * command registry.
   */
  getCommandCatalog(options: { activeContextsOnly?: boolean } = {}): CommandCatalogEntry[] {
    const commandIds = new Set<string>([
      ...this.commandRegistry.getDefinitions().map(definition => definition.id),
      ...this.commandRegistry.getRegisteredIds(),
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

  resolveCommandId(reference: string): string | undefined {
    return this.commandRegistry.resolve(reference);
  }

  dispatchCommandByName(
    reference: string,
    invocation: CommandInvocation = { source: 'programmatic' },
  ): boolean {
    return this.commandRegistry.dispatchByName(reference, invocation);
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
    const definition = this.commandRegistry.getDefinition(commandId);
    const preferredBinding = this.getPreferredKeyboardBinding(commandId);
    if (!definition && !preferredBinding && !this.commandRegistry.isRegistered(commandId)) return undefined;

    return {
      id: commandId,
      title: definition?.title ?? preferredBinding?.description ?? commandId,
      description: definition?.description ?? preferredBinding?.description,
      category: definition?.category ?? preferredBinding?.category,
      icon: definition?.icon,
      name: definition?.name,
      aliases: definition?.aliases,
      contextIds: definition?.contextIds ?? (preferredBinding ? [preferredBinding.contextId] : undefined),
      registered: this.commandRegistry.isRegistered(commandId),
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

    this.activeContexts.set(contextId, {
      id: contextId,
      activatedAt: Date.now(),
    });

    // A context switch changes who owns the next stroke. Do not carry a
    // half-entered chord from the pane into a dialog/overlay.
    this.clearPendingChord();
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
    this.applyBindingRegistration(binding, ownerId);
    this.emitBindings();
  }

  registerBindings(bindings: readonly InputBindingDefinition[], ownerId = 'default'): void {
    for (const binding of bindings) this.applyBindingRegistration(binding, ownerId);
    this.emitBindings();
  }

  private applyBindingRegistration(binding: InputBindingDefinition, ownerId: string): void {
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
  }

  unregisterBinding(bindingId: string, ownerId = 'default'): void {
    if (this.applyBindingUnregistration(bindingId, ownerId)) this.emitBindings();
  }

  unregisterBindings(bindingIds: readonly string[], ownerId = 'default'): void {
    let changed = false;
    for (const bindingId of bindingIds) {
      if (this.applyBindingUnregistration(bindingId, ownerId)) changed = true;
    }
    if (changed) this.emitBindings();
  }

  private applyBindingUnregistration(bindingId: string, ownerId: string): boolean {
    const registrations = this.bindingRegistrations.get(bindingId);
    if (!registrations) return false;

    registrations.delete(ownerId);
    if (registrations.size === 0) {
      this.bindingRegistrations.delete(bindingId);
      this.bindings.delete(bindingId);
      return true;
    }

    this.bindings.set(bindingId, this.getActiveBindingRegistration(registrations));
    return true;
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
        if (this.commandRegistry.isRegistered(command.id)) {
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


  private userOverrideOwner(bindingId: string): string {
    return `user-binding:${bindingId}`;
  }

  private pluginInputOwner(pluginId: string): string {
    return `plugin-input:${pluginId}`;
  }

  private removePluginInputRegistration(
    ownerId: string,
    registration: { commandIds: string[]; bindingIds: string[]; contextIds: string[] },
  ): void {
    for (const bindingId of registration.bindingIds) {
      // A plugin binding may currently be shadowed by a persisted user
      // customization. Remove that runtime user registration first so plugin
      // unload never leaves an orphan binding pointing at an unloaded command.
      // The persistence layer keeps the customization and reapplies it if the
      // plugin binding appears again later.
      this.clearUserBindingCustomization(bindingId);
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

  applyUserBindingCustomization(
    customization: InputBindingCustomization,
  ): boolean {
    const registrations = this.bindingRegistrations.get(customization.id);
    if (!registrations || registrations.size === 0) return false;

    const ownerId = this.userOverrideOwner(customization.id);
    const base = Array.from(registrations.entries())
      .filter(([owner]) => owner !== ownerId)
      .map(([, binding]) => binding)
      .sort((left, right) => right.registrationOrder - left.registrationOrder)[0];

    if (!base) return false;

    this.registerBinding({
      ...base,
      source: 'user',
      trigger: customization.trigger ?? base.trigger,
      enabled: customization.enabled ?? base.enabled,
    }, ownerId);

    return true;
  }

  clearUserBindingCustomization(bindingId: string): boolean {
    const registrations = this.bindingRegistrations.get(bindingId);
    if (!registrations) return false;

    const ownerId = this.userOverrideOwner(bindingId);
    if (!registrations.has(ownerId)) return false;

    this.unregisterBinding(bindingId, ownerId);
    return true;
  }

  hasUserBindingCustomization(bindingId: string): boolean {
    return this.bindingRegistrations
      .get(bindingId)
      ?.has(this.userOverrideOwner(bindingId)) ?? false;
  }

  inspectBindingConflicts(): InputBindingConflict[] {
    const bindings = Array.from(this.bindings.values())
      .filter(binding => binding.enabled);

    const conflicts: InputBindingConflict[] = [];

    for (let leftIndex = 0; leftIndex < bindings.length; leftIndex += 1) {
      const left = bindings[leftIndex];

      for (let rightIndex = leftIndex + 1; rightIndex < bindings.length; rightIndex += 1) {
        const right = bindings[rightIndex];
        if (!this.sameInputTrigger(left.trigger, right.trigger)) continue;
        if (left.contextId !== right.contextId) continue;
        if (left.commandId === right.commandId) continue;
        if (!whenClausesCanOverlap(left.when, right.when)) continue;

        const winner = this.chooseBestBinding([left, right]).binding;
        const loser = winner.id === left.id ? right : left;
        const sameContext = left.contextId === right.contextId;

        conflicts.push({
          bindingId: left.id,
          conflictingBindingId: right.id,
          contextId: sameContext
            ? left.contextId
            : `${left.contextId} ↔ ${right.contextId}`,
          triggerLabel: this.formatInputTrigger(left.trigger),
          winnerBindingId: winner.id,
          loserBindingId: loser.id,
          reason: sameContext
            ? 'same-context'
            : winner.source !== loser.source
              ? 'shadowed-by-source'
              : (winner.priority ?? 0) !== (loser.priority ?? 0)
                ? 'shadowed-by-priority'
                : 'overlapping-context',
        });
      }
    }

    return conflicts;
  }

  formatInputTrigger(trigger: InputTrigger): string {
    if (trigger.device === 'keyboard') {
      return trigger.chord.steps
        .map(step => this.formatDebugStroke(step))
        .join(' → ');
    }

    const gesture = trigger.gesture;
    const modifiers = (gesture.modifiers ?? []).map(value => {
      if (value === 'ctrl') return 'Ctrl';
      if (value === 'shift') return 'Shift';
      if (value === 'alt') return 'Alt';
      if (value === 'meta') return 'Meta';
      return value;
    });
    const parts: string[] = [
      ...modifiers,
      gesture.button,
      gesture.type,
      gesture.direction,
    ].filter((value): value is Exclude<typeof value, undefined> => value !== undefined);

    return parts.join('+');
  }

  getRegisteredCommandIds(): string[] {
    return this.commandRegistry.getRegisteredIds();
  }

  dispatchCommandById(commandId: string, invocation: CommandInvocation = { source: 'programmatic' }): boolean {
    return this.commandRegistry.dispatch(commandId, invocation);
  }

  updateBindingTrigger(bindingId: string, trigger: InputTrigger): boolean {
    const binding = this.bindings.get(bindingId);
    if (!binding) return false;
    binding.trigger = trigger;
    this.emitBindings();
    return true;
  }

  registerCommandDefinition(def: CommandDefinition): void {
    this.commandRegistry.registerDefinition(def);
  }

  unregisterCommandDefinition(commandId: string): void {
    this.commandRegistry.unregisterDefinition(commandId);
  }

  getCommandDefinitions(): CommandDefinition[] {
    return this.commandRegistry.getDefinitions();
  }

  getCommandDefinition(commandId: string): CommandDefinition | undefined {
    return this.commandRegistry.getDefinition(commandId);
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
    if (!this.commandRegistry.isEnabled(resolution.binding.commandId, invocation)) {
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

    return this.commandRegistry.dispatchAsync(resolution.binding.commandId, invocation);
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
    if (!this.commandRegistry.isEnabled(resolution.binding.commandId, invocation)) return false;

    return this.commandRegistry.dispatchAsync(resolution.binding.commandId, invocation);
  }

  private initializeKeyboardListener(): void {
    const listener = (event: KeyboardEvent) => {
      if (!this.keyboardEnabled || event.isComposing) return;
      if (this.isApplicationInputOwned(event.target)) return;

      // A native keydown must cross the command boundary at most once. This
      // protects the engine from duplicate host/document bridges and from any
      // future secondary capture surface accidentally forwarding the same DOM
      // event back into the global dispatcher.
      if (this.handledNativeKeyboardEvents.has(event)) return;
      this.handledNativeKeyboardEvents.add(event);

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
      if (!this.commandRegistry.isEnabled(resolution.binding.commandId, invocation)) {
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

      this.commandRegistry.dispatch(resolution.binding.commandId, invocation);
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
    document.addEventListener('mousedown', (event) => {
      this.beginMouseDrag(event);
      this.handleMouseEvent('mousedown', event);
    }, listenerOptions);
    document.addEventListener('mousemove', (event) => this.handleMouseDrag(event), listenerOptions);
    document.addEventListener('mouseup', (event) => {
      this.handleMouseEvent('mouseup', event);
      this.mouseDragState = null;
    }, listenerOptions);
    document.addEventListener('wheel', (event) => {
      this.boostPlainScrollSurfaceWheel(event);
      this.handleMouseEvent('wheel', event);
    }, wheelOptions);
  }

  private boostPlainScrollSurfaceWheel(event: WheelEvent): void {
    // Keep command gestures (Ctrl+Wheel viewer zoom, Shift+Wheel horizontal
    // navigation, etc.) exact. Plain scrolling gets only a small additive
    // boost while the browser still performs its native scroll.
    if (
      event.defaultPrevented ||
      event.ctrlKey ||
      event.metaKey ||
      event.shiftKey ||
      event.altKey
    ) {
      return;
    }

    const target = event.target instanceof Element
      ? event.target.closest(SCROLL_SURFACE_SELECTOR)
      : null;
    if (!(target instanceof HTMLElement)) {
      return;
    }

    const unitX = event.deltaMode === 1
      ? 16
      : event.deltaMode === 2
        ? Math.max(1, target.clientWidth)
        : 1;
    const unitY = event.deltaMode === 1
      ? 16
      : event.deltaMode === 2
        ? Math.max(1, target.clientHeight)
        : 1;

    const deltaX = event.deltaX * unitX;
    const deltaY = event.deltaY * unitY;

    if (Math.abs(deltaY) >= Math.abs(deltaX) && target.scrollHeight > target.clientHeight) {
      const maxScrollTop = Math.max(0, target.scrollHeight - target.clientHeight);
      target.scrollTop = Math.max(
        0,
        Math.min(maxScrollTop, target.scrollTop + deltaY * SCROLL_SURFACE_EXTRA_FACTOR),
      );
      return;
    }

    if (deltaX !== 0 && target.scrollWidth > target.clientWidth) {
      const maxScrollLeft = Math.max(0, target.scrollWidth - target.clientWidth);
      target.scrollLeft = Math.max(
        0,
        Math.min(maxScrollLeft, target.scrollLeft + deltaX * SCROLL_SURFACE_EXTRA_FACTOR),
      );
    }
  }

  private beginMouseDrag(event: MouseEvent): void {
    if (this.isApplicationInputOwned(event.target)) {
      this.mouseDragState = null;
      return;
    }

    const modifiers: MouseGesture['modifiers'] = [];
    if (event.ctrlKey || event.metaKey) modifiers.push(event.ctrlKey ? 'ctrl' : 'meta');
    if (event.shiftKey) modifiers.push('shift');
    if (event.altKey) modifiers.push('alt');

    this.mouseDragState = {
      startX: event.clientX,
      startY: event.clientY,
      button: this.normalizeMouseButton(event.button),
      modifiers: modifiers.length ? modifiers.sort() : undefined,
      emitted: false,
      target: event.target,
    };
  }

  private handleMouseDrag(event: MouseEvent): void {
    const state = this.mouseDragState;
    if (!state || state.emitted || (event.buttons === 0 && state.button !== 'right')) return;

    const deltaX = event.clientX - state.startX;
    const deltaY = event.clientY - state.startY;
    const distance = Math.hypot(deltaX, deltaY);
    if (distance < 8) return;

    const direction: MouseGesture['direction'] =
      Math.abs(deltaX) > Math.abs(deltaY)
        ? (deltaX < 0 ? 'left' : 'right')
        : (deltaY < 0 ? 'up' : 'down');

    const gesture: MouseGesture = {
      type: 'drag',
      button: state.button,
      modifiers: state.modifiers,
      direction,
    };

    const resolution = this.resolveMouseGesture(gesture, event);
    if (!resolution) return;

    const invocation: CommandInvocation = {
      source: 'mouse',
      context: {
        bindingId: resolution.binding.id,
        dragStartX: state.startX,
        dragStartY: state.startY,
        clientX: event.clientX,
        clientY: event.clientY,
      },
    };
    if (!this.commandRegistry.isEnabled(resolution.binding.commandId, invocation)) return;

    state.emitted = true;
    this.clearPendingChord();

    if (resolution.binding.preventDefault !== false) event.preventDefault();
    if (resolution.binding.stopPropagation !== false) event.stopPropagation();
    this.commandRegistry.dispatch(resolution.binding.commandId, invocation);
  }

  private handleMouseEvent(kind: string, event: MouseEvent | WheelEvent): void {
    if (!this.keyboardEnabled) return;
    if (this.isApplicationInputOwned(event.target)) return;

    const gesture = this.normalizeMouseGesture(kind, event);
    if (!gesture) return;

    const resolution = this.resolveMouseGesture(gesture, event);
    if (!resolution) return;

    const invocation: CommandInvocation = { source: 'mouse', context: { bindingId: resolution.binding.id } };
    if (!this.commandRegistry.isEnabled(resolution.binding.commandId, invocation)) return;

    this.clearPendingChord();

    if (resolution.binding.preventDefault !== false) {
      event.preventDefault();
    }
    if (resolution.binding.stopPropagation !== false) {
      event.stopPropagation();
    }

    this.commandRegistry.dispatch(resolution.binding.commandId, invocation);
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
      NumpadDivide: '/',
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

    // WebView2 may report accelerator combinations with
    // KeyboardEvent.key === "Unidentified" while still preserving the physical
    // DOM code. Recover the logical key from code only when key itself is
    // unusable. This includes navigation accelerators such as Ctrl+Alt+ArrowLeft.
    if (code && keyMap[code]) {
      return keyMap[code];
    }
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
    const activeContextIds = this.getEffectiveActiveContextIds();

    return Array.from(this.bindings.values()).filter(binding => {
      if (!binding.enabled || binding.trigger.device !== 'keyboard') return false;
      if (!activeContextIds.has(binding.contextId)) return false;
      if (this.isEditableKeyboardContext(keyboardContext) && !this.whenClauseExplicitlyAllowsTyping(binding.when)) return false;
      if (keyboardContext?.inContextMenu) return false;
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

  private matchMouseBindings(gesture: MouseGesture, event?: MouseEvent | WheelEvent): RegisteredBinding[] {
    const activeContextIds = this.getEffectiveActiveContextIds();
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

      // Registration timing is not a semantic shortcut priority. Use a stable
      // identity tie-breaker so plugin/component load order cannot change the
      // winning command between runs.
      return compareStableBindingIdentity(left, right);
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
    const leftMods = [...(left.modifiers ?? [])].sort();
    const rightMods = [...(right.modifiers ?? [])].sort();

    return (
      left.key === right.key &&
      leftMods.length === rightMods.length &&
      leftMods.every((modifier, index) => modifier === rightMods[index])
    );
  }

  private isEditableKeyboardContext(context?: KeyboardEventContext | null): boolean {
    return !!context && (context.inInput || context.inTextarea || context.inSelect || context.inContentEditable);
  }

  private whenClauseExplicitlyAllowsTyping(expression?: string): boolean {
    if (!expression) return false;
    return expression
      .split('&&')
      .map(term => term.trim())
      .some(term => term === 'typing');
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
        return !!context && (context.inInput || context.inTextarea || context.inSelect || context.inContentEditable);
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
      inContextMenu: !!source.inContextMenu,
      inQuickAccessMenu: !!source.inQuickAccessMenu,
      hasActivePane: !!source.hasActivePane,
      isFilterInput: !!source.isFilterInput,
      inFileEntry: !!source.inFileEntry,
      inViewerSurface: !!source.inViewerSurface,
    };
  }

  private isApplicationInputOwned(target: EventTarget | null): boolean {
    const element = target instanceof Element ? target : null;
    return !!element?.closest('[data-input-owned]');
  }

  private getKeyboardEventContext(event: { target: EventTarget | null }): KeyboardEventContext {
    const target = event.target;
    const element = target instanceof Element ? target : null;
    const htmlElement = target instanceof HTMLElement ? target : null;
    const tagName = htmlElement?.tagName?.toLowerCase() ?? '';

    return {
      inInput: tagName === 'input',
      inTextarea: tagName === 'textarea',
      inSelect: tagName === 'select',
      inContentEditable: !!htmlElement?.isContentEditable,
      inModal: !!element?.closest('.modal, [role="dialog"]'),
      inContextMenu: !!element?.closest('[role="menu"]'),
      inQuickAccessMenu: !!element?.closest('.ab-quick-access-drop'),
      hasActivePane: !!element?.closest('.pane'),
      isFilterInput: !!element?.classList.contains('filter-input'),
      inFileEntry: !!element?.closest('.file-entry, .entry[data-file-entry], .tree-row[data-file-entry]'),
      inViewerSurface: !!element?.closest('.viewer-surface'),
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

  private sameInputTrigger(left: InputTrigger, right: InputTrigger): boolean {
    if (left.device !== right.device) return false;

    if (left.device === 'keyboard' && right.device === 'keyboard') {
      if (left.chord.steps.length !== right.chord.steps.length) return false;
      return left.chord.steps.every((step, index) =>
        this.sameStroke(step, right.chord.steps[index])
      );
    }

    return left.device === 'mouse' &&
      right.device === 'mouse' &&
      this.sameGesture(left.gesture, right.gesture);
  }

  private contextsCanOverlap(leftId: string, rightId: string): boolean {
    if (leftId === rightId) return true;
    if (leftId === 'global' || rightId === 'global') return true;

    const left = this.contexts.get(leftId);
    const right = this.contexts.get(rightId);
    if (!left || !right) return false;

    // Exclusive surfaces replace the ordinary context stack. Two bindings can
    // only compete while an exclusive surface is active when both belong to
    // that same exclusive subtree. This prevents the conflict inspector from
    // reporting Escape/Arrow/Enter collisions between mutually-exclusive
    // dialogs and overlays that can never receive the same event.
    const leftExclusiveRoot = this.exclusiveContextRoot(leftId);
    const rightExclusiveRoot = this.exclusiveContextRoot(rightId);
    if (leftExclusiveRoot || rightExclusiveRoot) {
      return !!leftExclusiveRoot && leftExclusiveRoot === rightExclusiveRoot;
    }

    return left.parentId === rightId ||
      right.parentId === leftId ||
      (!!left.parentId && left.parentId === right.parentId);
  }

  private exclusiveContextRoot(contextId: string): string | null {
    let current = this.contexts.get(contextId);
    const visited = new Set<string>();

    while (current && !visited.has(current.id)) {
      visited.add(current.id);
      if (current.exclusive) return current.id;
      current = current.parentId ? this.contexts.get(current.parentId) : undefined;
    }

    return null;
  }

  private sameGesture(left: MouseGesture, right: MouseGesture): boolean {
    const leftMods = (left.modifiers ?? []).join('+');
    const rightMods = (right.modifiers ?? []).join('+');
    return left.type === right.type
      && (left.button ?? null) === (right.button ?? null)
      && (left.direction ?? null) === (right.direction ?? null)
      && leftMods === rightMods;
  }

  private getEffectiveActiveContextIds(): Set<string> {
    const active = this.getActiveContextIds();

    const exclusive = Array.from(this.activeContexts.values())
      .map(state => ({
        state,
        context: this.contexts.get(state.id),
      }))
      .filter(
        (entry): entry is {
          state: ActiveContextState;
          context: InputContextDefinition;
        } => !!entry.context?.exclusive,
      )
      .sort((left, right) =>
        right.context.priority - left.context.priority ||
        right.state.activatedAt - left.state.activatedAt,
      )[0];

    if (!exclusive) {
      return active;
    }

    const result = new Set<string>(['global', exclusive.context.id]);

    // Keep active descendants of the winning exclusive context available for
    // local widgets nested inside the modal surface.
    for (const contextId of active) {
      if (
        contextId !== 'global' &&
        this.isContextDescendantOf(contextId, exclusive.context.id)
      ) {
        result.add(contextId);
      }
    }

    return result;
  }

  private isContextDescendantOf(
    contextId: string,
    ancestorId: string,
  ): boolean {
    let current = this.contexts.get(contextId);
    const visited = new Set<string>();

    while (current?.parentId && !visited.has(current.id)) {
      visited.add(current.id);

      if (current.parentId === ancestorId) {
        return true;
      }

      current = this.contexts.get(current.parentId);
    }

    return false;
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

  private emitActiveContexts(): void {
    const active = Array.from(this.activeContexts.values())
      .sort((left, right) => right.activatedAt - left.activatedAt)
      .map(entry => this.contexts.get(entry.id))
      .filter((context): context is InputContextDefinition => !!context);
    this.activeContexts$.next(active);
  }
}
