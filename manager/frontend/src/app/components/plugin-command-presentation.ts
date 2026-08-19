import type { CommandInvocation, CommandInvocationSource, CommandState } from '../services/input-system.types';
import type { PresentedCommand } from './presented-command';

export interface PluginCommandPresentationHost {
  getCommandState(commandId: string, invocation?: CommandInvocation): CommandState | undefined;
  dispatchCommandById(commandId: string, invocation?: CommandInvocation): boolean;
}

export interface PluginCommandPresentationContribution<TContext> {
  id: string;
  label?: string;
  icon?: string;
  shortcut?: string;
  commandId?: string;
  execute?: (context: TContext) => void | Promise<void>;
}

export function presentPluginCommand<TContext>(
  contribution: PluginCommandPresentationContribution<TContext>,
  context: TContext,
  host: PluginCommandPresentationHost,
  options: { separatorBefore?: boolean; source?: CommandInvocationSource } = {},
): PresentedCommand {
  const commandId = contribution.commandId?.trim() || undefined;
  const invocation: CommandInvocation = {
    source: options.source ?? 'context-menu',
    context: context as unknown as Readonly<Record<string, unknown>>,
  };
  const state = commandId ? host.getCommandState(commandId, invocation) : undefined;
  const enabled = commandId ? !!state?.registered && state.enabled : !!contribution.execute;

  return {
    id: `plugin:${contribution.id}`,
    label: contribution.label?.trim() || state?.title || contribution.id,
    icon: 'plugin',
    iconText: contribution.icon,
    shortcut: contribution.shortcut,
    enabled,
    source: 'plugin',
    commandId,
    separatorBefore: options.separatorBefore,
    execute: async () => {
      if (!enabled) return;
      if (commandId) {
        host.dispatchCommandById(commandId, invocation);
        return;
      }
      await contribution.execute?.(context);
    },
  };
}