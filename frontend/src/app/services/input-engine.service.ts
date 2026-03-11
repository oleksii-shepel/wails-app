import { Injectable } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import { DIALOG_INPUT_CATALOG } from './dialog-input-catalog';
import type {
  BindingResolution,
  InputBindingDefinition,
  InputContextDefinition,
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
    return Array.from(this.bindings.values()).map(binding => ({ ...binding }));
  }

  getContextSnapshot(): InputContextDefinition[] {
    return Array.from(this.contexts.values());
  }

  resolveKeyStroke(stroke: KeyStroke): BindingResolution | null {
    const currentSequence = this.pendingChord ? [...this.pendingChord.steps, stroke] : [stroke];
    const candidates = this.matchBindings(currentSequence);

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

  private initializeKeyboardListener(): void {
    document.addEventListener('keydown', (event) => {
      if (event.isComposing) return;

      const stroke = this.normalizeKeyStroke(event);
      const resolution = this.resolveKeyStroke(stroke);
      if (!resolution) return;

      const command = this.getCommandHandler(resolution.binding.commandId);
      if (!command) return;

      event.preventDefault();
      event.stopPropagation();

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
      Backspace: 'backspace',
      Delete: 'delete',
      ArrowUp: 'arrowup',
      ArrowDown: 'arrowdown',
      ArrowLeft: 'arrowleft',
      ArrowRight: 'arrowright',
    };

    return keyMap[key] ?? key.toLowerCase();
  }

  private matchBindings(sequence: KeyStroke[]): RegisteredBinding[] {
    const activeContextIds = this.getActiveContextIds();

    return Array.from(this.bindings.values()).filter(binding => {
      if (!binding.enabled || binding.trigger.device !== 'keyboard') return false;
      if (!activeContextIds.has(binding.contextId)) return false;
      if (!this.whenClauseAllows(binding.when)) return false;

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

  private whenClauseAllows(expression?: string): boolean {
    if (!expression || expression.trim() === '' || expression === 'true') {
      return true;
    }
    return false;
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