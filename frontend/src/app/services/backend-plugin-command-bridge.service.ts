import { Injectable } from '@angular/core';
import type { plugin } from '../../../wailsjs/go/models';
import type { CommandDescriptor, CommandInvocation, InputBindingDefinition, KeyStroke, PluginInputContribution } from './input-system.types';
import { ApiService } from './api.service';
import { InputEngineService } from './input-engine.service';

export type PluginSelectionProvider = () => readonly string[];

const OWNER_PREFIX = 'backend-plugin:';

export function backendPluginCommandId(pluginId: string, commandId: string): string {
  return `plugin.backend.${pluginId}.${commandId}`;
}

export function backendPluginCommandName(pluginId: string, commandId: string): string {
  return `${pluginId}.${commandId}`.toLowerCase();
}

function ownedPluginId(pluginId: string): string {
  return `${OWNER_PREFIX}${pluginId}`;
}

type KeyModifier = NonNullable<KeyStroke['modifiers']>[number];

function normalizeModifier(token: string): KeyModifier | null {
  switch (token.trim().toLowerCase()) {
    case 'ctrl':
    case 'control': return 'ctrl';
    case 'shift': return 'shift';
    case 'alt':
    case 'option': return 'alt';
    case 'cmd':
    case 'command':
    case 'meta':
    case 'win':
    case 'windows': return 'meta';
    default: return null;
  }
}

export function parsePluginHotkey(hotkey: string | null | undefined): KeyStroke | null {
  const raw = (hotkey ?? '').trim();
  if (!raw || /\s/.test(raw)) return null; // Multi-stroke chords are not encoded by backend metadata yet.

  const parts = raw.split('+').map(part => part.trim()).filter(Boolean);
  if (parts.length === 0) return null;

  const key = parts.pop()!.toLowerCase();
  if (!key) return null;

  const modifiers: KeyModifier[] = [];
  for (const part of parts) {
    const modifier = normalizeModifier(part);
    if (!modifier) return null;
    if (!modifiers.includes(modifier)) modifiers.push(modifier);
  }

  return { key, modifiers };
}

function commandNeedsSelection(command: plugin.CommandInfo): boolean {
  return !!command.appliesTo?.length;
}

function invocationPaths(invocation: CommandInvocation | undefined, fallback: readonly string[]): string[] {
  const candidate = invocation?.args?.['paths'];
  if (Array.isArray(candidate) && candidate.every(path => typeof path === 'string')) {
    return [...candidate];
  }
  return [...fallback];
}

@Injectable({ providedIn: 'root' })
export class BackendPluginCommandBridgeService {
  private readonly registeredPluginIds = new Set<string>();
  private selectionProvider: PluginSelectionProvider = () => [];

  constructor(
    private readonly api: ApiService,
    private readonly inputEngine: InputEngineService,
  ) {}

  async start(selectionProvider: PluginSelectionProvider): Promise<void> {
    this.selectionProvider = selectionProvider;
    await this.refresh();
  }

  async refresh(): Promise<void> {
    const plugins = await this.api.listBackendPlugins();
    this.clearRegisteredCommands();

    try {
      for (const info of plugins ?? []) {
        const contribution = this.toInputContribution(info);
        if (contribution.commands.length === 0) continue;

        this.inputEngine.registerPluginInput(contribution);
        this.registeredPluginIds.add(contribution.pluginId);
      }
    } catch (error) {
      this.clearRegisteredCommands();
      throw error;
    }
  }

  stop(): void {
    this.clearRegisteredCommands();
    this.selectionProvider = () => [];
  }

  private toInputContribution(info: plugin.Info): PluginInputContribution {
    const pluginId = ownedPluginId(info.id);
    const commands: CommandDescriptor[] = [];
    const bindings: InputBindingDefinition[] = [];

    for (const command of info.commands ?? []) {
      const id = backendPluginCommandId(info.id, command.id);
      commands.push({
        id,
        title: command.label || command.id,
        description: info.name ? `${info.name} plugin command` : 'Backend plugin command',
        category: 'Plugins',
        icon: command.icon,
        name: backendPluginCommandName(info.id, command.id),
        execute: (invocation) => this.api.invokeBackendPluginCommand(
          info.id,
          command.id,
          invocationPaths(invocation, this.selectionProvider()),
        ),
        isEnabled: (invocation) => !commandNeedsSelection(command)
          || invocationPaths(invocation, this.selectionProvider()).length > 0,
      });

      const stroke = parsePluginHotkey(command.hotkey);
      if (stroke) {
        bindings.push({
          id: `${id}.defaultHotkey`,
          commandId: id,
          contextId: 'global',
          source: 'plugin',
          description: command.label || command.id,
          category: 'Plugins',
          when: '!typing && !quickAccess && hasActivePane',
          preventDefault: true,
          trigger: { device: 'keyboard', chord: { steps: [stroke] } },
        });
      }
    }

    return { pluginId, commands, bindings };
  }

  private clearRegisteredCommands(): void {
    for (const pluginId of this.registeredPluginIds) {
      this.inputEngine.unregisterPluginInput(pluginId);
    }
    this.registeredPluginIds.clear();
  }
}