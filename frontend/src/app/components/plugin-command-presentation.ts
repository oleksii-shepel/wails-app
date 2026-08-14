import type { CommandState } from '../services/input-system.types';
import type { PresentedCommand } from './presented-command';

export interface PluginCommandPresentationHost {
  getCommandState(commandId: string): CommandState | undefined;
  dispatchCommandById(commandId: string): boolean;
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
  options: { separatorBefore?: boolean } = {},
): PresentedCommand {
  const commandId = contribution.commandId?.trim() || undefined;
  const state = commandId ? host.getCommandState(commandId) : undefined;
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
        host.dispatchCommandById(commandId);
        return;
      }
      await contribution.execute?.(context);
    },
  };
}
