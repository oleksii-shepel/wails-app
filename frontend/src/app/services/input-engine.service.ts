import { Injectable } from '@angular/core';
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
} from './input-system.types';

type InputCommandHandler = () => void | Promise<void>;

interface RegisteredCommand {
  registrationId: string;
  handler: InputCommandHandler;
  registrationOrder: number;
}

interface RegisteredBinding extends InputBindingDefinition {
  registrationOrder: number;
  enabled: boolean;
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

const SOURCE_PRIORITY: Record<InputSourceKind, number> = {
  default: 10,
  plugin: 20,
  workspace: 30,
  user: 40,
};

const CHORD_TIMEOUT_MS = 1800;

@Injectable({
  providedIn: 'root'
})
export class InputEngineService {
  private readonly contexts = new Map<string, InputContextDefinition>();
  private readonly bindings = new Map<string, RegisteredBinding>();
  private readonly commands = new Map<string, Map<string, RegisteredCommand>>();
  private readonly pluginBindingIds = new Map<string, string[]>();
  private readonly activeContexts = new Map<string, ActiveContextState>();

  private pendingChord: PendingChordState | null = null;
  private pendingChordTimer: number | null = null;
  private registrationCounter = 0;
  private commandRegistrationCounter = 0;

  readonly activeContexts$ = createBehaviorSubject<InputContextDefinition[]>([]);
  readonly pendingChord$ = createBehaviorSubject<PendingChordState | null>(null);
  readonly bindings$ = createBehaviorSubject<InputBindingDefinition[]>([]);

  private keyboardEnabled = true;

  constructor() {
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

  registerBinding(binding: InputBindingDefinition): void {
    this.bindings.set(binding.id, {
      ...binding,
      preventDefault: binding.preventDefault !== false,
      stopPropagation: binding.stopPropagation !== false,
      enabled: true,
      registrationOrder: ++this.registrationCounter,
    });
    this.emitBindings();
  }

  unregisterBinding(bindingId: string): void {
    if (!this.bindings.delete(bindingId)) return;
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

  registerPluginBindings(contribution: PluginInputContribution): void {
    const ids: string[] = [];

    for (const context of contribution.contexts ?? []) {
      this.registerContext(context);
    }

    for (const binding of contribution.bindings) {
      this.registerBinding(binding);
      ids.push(binding.id);
    }

    this.pluginBindingIds.set(contribution.pluginId, ids);
  }

  unregisterPluginBindings(pluginId: string): void {
    const ids = this.pluginBindingIds.get(pluginId) ?? [];
    for (const id of ids) {
      this.unregisterBinding(id);
    }
    this.pluginBindingIds.delete(pluginId);
  }

  getBindingSnapshot(): InputBindingDefinition[] {
    return Array.from(this.bindings.values()).map(binding => ({
      ...binding,
      enabled: binding.enabled,
    }));
  }

  getContextSnapshot(): InputContextDefinition[] {
    return Array.from(this.contexts.values());
  }

  resolveKeyStroke(stroke: KeyStroke, event?: KeyboardEvent): BindingResolution | null {
    const currentSequence = this.pendingChord ? [...this.pendingChord.steps, stroke] : [stroke];
    const candidates = this.matchBindings(currentSequence, event);

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

  async dispatchKeyStroke(stroke: KeyStroke): Promise<boolean> {
    const resolution = this.resolveKeyStroke(stroke);
    if (!resolution) return false;

    const command = this.getCommandHandler(resolution.binding.commandId);
    if (!command) return false;

    try {
      await command();
      return true;
    } catch (error) {
      console.error('Input command failed:', error);
      return false;
    }
  }

  async dispatchMouseGesture(gesture: MouseGesture): Promise<boolean> {
    const resolution = this.resolveMouseGesture(gesture);
    if (!resolution) return false;

    const command = this.getCommandHandler(resolution.binding.commandId);
    if (!command) return false;

    try {
      await command();
      return true;
    } catch (error) {
      console.error('Input command failed:', error);
      return false;
    }
  }

  private initializeKeyboardListener(): void {
    document.addEventListener('keydown', (event) => {
      if (!this.keyboardEnabled || event.isComposing) return;

      const stroke = this.normalizeKeyStroke(event);
      const resolution = this.resolveKeyStroke(stroke, event);
      if (!resolution) return;

      const command = this.getCommandHandler(resolution.binding.commandId);
      if (!command) return;

      if (resolution.binding.preventDefault !== false) {
        event.preventDefault();
      }
      if (resolution.binding.stopPropagation !== false) {
        event.stopPropagation();
      }

      try {
        const result = command();
        if (result instanceof Promise) {
          result.catch(error => console.error('Input command failed:', error));
        }
      } catch (error) {
        console.error('Input command failed:', error);
      }
    }, { capture: true });
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

    const command = this.getCommandHandler(resolution.binding.commandId);
    if (!command) return;

    this.clearPendingChord();

    if (resolution.binding.preventDefault !== false) {
      event.preventDefault();
    }
    if (resolution.binding.stopPropagation !== false) {
      event.stopPropagation();
    }

    try {
      const result = command();
      if (result instanceof Promise) {
        result.catch(error => console.error('Input command failed:', error));
      }
    } catch (error) {
      console.error('Input command failed:', error);
    }
  }

  private resolveMouseGesture(gesture: MouseGesture, event?: MouseEvent | WheelEvent): BindingResolution | null {
    const candidates = this.matchMouseBindings(gesture, event);
    if (candidates.length === 0) {
      return null;
    }

    return this.chooseBestBinding(candidates);
  }

  private normalizeKeyStroke(event: KeyboardEvent): KeyStroke {
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

    return keyMap[key] ?? key.toLowerCase();
  }

  private matchBindings(sequence: KeyStroke[], event?: KeyboardEvent): RegisteredBinding[] {
    const activeContextIds = this.getActiveContextIds();
    const keyboardContext = event ? this.getKeyboardEventContext(event) : null;

    return Array.from(this.bindings.values()).filter(binding => {
      if (!binding.enabled || binding.trigger.device !== 'keyboard') return false;
      if (!activeContextIds.has(binding.contextId)) return false;
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
    const leftMods = (left.modifiers ?? []).join('+');
    const rightMods = (right.modifiers ?? []).join('+');
    return left.key === right.key && leftMods === rightMods;
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

  private getActiveContextIds(): Set<string> {
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

  private getCommandHandler(commandId: string): InputCommandHandler | null {
    const registrations = this.commands.get(commandId);
    if (!registrations || registrations.size === 0) {
      return null;
    }

    let activeRegistration: RegisteredCommand | null = null;
    for (const registration of registrations.values()) {
      if (!activeRegistration || registration.registrationOrder > activeRegistration.registrationOrder) {
        activeRegistration = registration;
      }
    }

    return activeRegistration?.handler ?? null;
  }

  private emitActiveContexts(): void {
    const active = Array.from(this.activeContexts.values())
      .sort((left, right) => right.activatedAt - left.activatedAt)
      .map(entry => this.contexts.get(entry.id))
      .filter((context): context is InputContextDefinition => !!context);
    this.activeContexts$.next(active);
  }
}