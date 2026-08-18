import { Injectable, NgZone } from '@angular/core';
import { createBehaviorSubject } from '@epikodelabs/streamix';
import type {
  CommandDefinition,
  CommandDescriptor,
  CommandInvocation,
  CommandState,
  InputCommandHandler,
} from './input-system.types';

interface RegisteredCommand {
  readonly ownerId: string;
  readonly handler: InputCommandHandler;
  readonly isEnabled?: (invocation?: CommandInvocation) => boolean;
  readonly definition?: CommandDefinition;
  readonly registrationOrder: number;
}

function normalizeReference(value: string): string {
  return value.trim().toLowerCase();
}

/**
 * Canonical application action layer.
 *
 * Input, menus, toolbars, palettes and plugins all resolve and execute actions
 * through this registry. Multiple owners may register the same command ID; the
 * most recent registration wins and unregistering it restores the previous one.
 */
@Injectable({ providedIn: 'root' })
export class CommandRegistryService {
  private readonly registrations = new Map<string, Map<string, RegisteredCommand>>();
  private readonly standaloneDefinitions = new Map<string, CommandDefinition>();
  private registrationCounter = 0;

  readonly commands$ = createBehaviorSubject<CommandState[]>([]);

  constructor(private readonly ngZone: NgZone) {}

  register(commandId: string, handler: InputCommandHandler, ownerId = 'default'): void {
    const definitions = this.registrations.get(commandId) ?? new Map<string, RegisteredCommand>();
    const previous = definitions.get(ownerId);
    definitions.set(ownerId, {
      ownerId,
      handler,
      definition: previous?.definition,
      isEnabled: previous?.isEnabled,
      registrationOrder: ++this.registrationCounter,
    });
    this.registrations.set(commandId, definitions);
    this.emit();
  }

  registerDescriptor(descriptor: CommandDescriptor, ownerId = 'default'): void {
    const { execute, isEnabled, ...definition } = descriptor;
    const registrations = this.registrations.get(descriptor.id) ?? new Map<string, RegisteredCommand>();
    registrations.set(ownerId, {
      ownerId,
      handler: execute,
      isEnabled,
      definition,
      registrationOrder: ++this.registrationCounter,
    });
    this.registrations.set(descriptor.id, registrations);
    this.emit();
  }

  unregister(commandId: string, ownerId = 'default'): void {
    const registrations = this.registrations.get(commandId);
    if (!registrations) return;
    registrations.delete(ownerId);
    if (registrations.size === 0) this.registrations.delete(commandId);
    this.emit();
  }

  registerDefinition(definition: CommandDefinition): void {
    this.standaloneDefinitions.set(definition.id, definition);
    this.emit();
  }

  unregisterDefinition(commandId: string): void {
    this.standaloneDefinitions.delete(commandId);
    this.emit();
  }

  getDefinition(commandId: string): CommandDefinition | undefined {
    return this.getActive(commandId)?.definition ?? this.standaloneDefinitions.get(commandId);
  }

  getDefinitions(): CommandDefinition[] {
    const ids = new Set<string>([
      ...this.standaloneDefinitions.keys(),
      ...this.registrations.keys(),
    ]);
    return Array.from(ids)
      .map(id => this.getDefinition(id))
      .filter((definition): definition is CommandDefinition => !!definition);
  }

  getRegisteredIds(): string[] {
    return Array.from(this.registrations.keys());
  }

  isRegistered(commandId: string): boolean {
    return this.registrations.has(commandId);
  }

  isEnabled(commandId: string, invocation?: CommandInvocation): boolean {
    const active = this.getActive(commandId);
    if (!active) return false;
    if (!active.isEnabled) return true;
    try {
      return active.isEnabled(invocation);
    } catch (error) {
      console.error(`Command enablement failed for ${commandId}:`, error);
      return false;
    }
  }

  getState(commandId: string, invocation?: CommandInvocation): CommandState | undefined {
    const definition = this.getDefinition(commandId);
    if (!definition) return undefined;
    return {
      ...definition,
      registered: this.isRegistered(commandId),
      enabled: this.isEnabled(commandId, invocation),
    };
  }

  getStates(): CommandState[] {
    const ids = new Set<string>([
      ...this.standaloneDefinitions.keys(),
      ...this.registrations.keys(),
    ]);
    return Array.from(ids)
      .map(id => this.getState(id))
      .filter((state): state is CommandState => !!state);
  }

  resolve(reference: string): string | undefined {
    const raw = reference.trim();
    if (!raw) return undefined;
    if (this.registrations.has(raw) || this.getDefinition(raw)) return raw;

    const normalized = normalizeReference(raw);
    let match: string | undefined;
    for (const definition of this.getDefinitions()) {
      const names = [definition.name, ...(definition.aliases ?? [])]
        .filter((value): value is string => !!value);
      if (!names.some(name => normalizeReference(name) === normalized)) continue;
      if (match && match !== definition.id) return undefined;
      match = definition.id;
    }
    return match;
  }

  dispatch(commandId: string, invocation: CommandInvocation = { source: 'programmatic' }): boolean {
    const active = this.getActive(commandId);
    if (!active || !this.isEnabled(commandId, invocation)) return false;

    try {
      this.ngZone.run(() => {
        const result = active.handler(invocation);
        if (result instanceof Promise) {
          result.catch(error => console.error(`Command ${commandId} failed:`, error));
        }
      });
      return true;
    } catch (error) {
      console.error(`Command ${commandId} failed:`, error);
      return false;
    }
  }

  async dispatchAsync(commandId: string, invocation: CommandInvocation = { source: 'programmatic' }): Promise<boolean> {
    const active = this.getActive(commandId);
    if (!active || !this.isEnabled(commandId, invocation)) return false;

    try {
      await this.ngZone.run(async () => {
        await active.handler(invocation);
      });
      return true;
    } catch (error) {
      console.error(`Command ${commandId} failed:`, error);
      return false;
    }
  }

  dispatchByName(reference: string, invocation: CommandInvocation = { source: 'programmatic' }): boolean {
    const commandId = this.resolve(reference);
    return commandId ? this.dispatch(commandId, invocation) : false;
  }

  private getActive(commandId: string): RegisteredCommand | undefined {
    const registrations = this.registrations.get(commandId);
    if (!registrations?.size) return undefined;
    let active: RegisteredCommand | undefined;
    for (const registration of registrations.values()) {
      if (!active || registration.registrationOrder > active.registrationOrder) active = registration;
    }
    return active;
  }

  private emit(): void {
    this.commands$.next(this.getStates());
  }
}
