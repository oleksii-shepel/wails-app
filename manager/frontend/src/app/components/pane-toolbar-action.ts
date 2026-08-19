import type { ActionAvailability } from '../services/action-availability';
import type { ToolbarActionContribution, ToolbarContext } from '../plugin/plugin-registry-api';
import type { PresentedCommand } from './presented-command';
import { presentPluginCommand, type PluginCommandPresentationHost } from './plugin-command-presentation';

export interface BuiltinPaneToolbarHandlers {
  createFolder(): void;
  deleteSelected(): void;
  copyToOther(): void;
  moveToOther(): void;
  refresh(): void;
}

export function buildBuiltinPaneToolbarActions(
  availability: ActionAvailability,
  handlers: BuiltinPaneToolbarHandlers,
): PresentedCommand[] {
  return [
    {
      id: 'new-folder',
      label: 'New folder',
      icon: 'new-folder',
      enabled: availability.createFolder,
      source: 'builtin',
      commandId: 'pane.files.newFolder',
      execute: handlers.createFolder,
    },
    {
      id: 'delete',
      label: 'Delete',
      icon: 'delete',
      enabled: availability.deleteSelected,
      source: 'builtin',
      commandId: 'pane.files.delete',
      execute: handlers.deleteSelected,
    },
    {
      id: 'copy-to-other',
      label: 'Copy to other pane',
      icon: 'copy',
      shortcut: 'F5',
      enabled: availability.copyToOther,
      source: 'builtin',
      commandId: 'pane.ctrl.copyToOther',
      execute: handlers.copyToOther,
    },
    {
      id: 'move-to-other',
      label: 'Move to other pane',
      icon: 'move',
      shortcut: 'F6',
      enabled: availability.moveToOther,
      source: 'builtin',
      commandId: 'pane.ctrl.moveToOther',
      execute: handlers.moveToOther,
    },
    {
      id: 'refresh',
      label: 'Refresh',
      icon: 'refresh',
      shortcut: 'Ctrl+R',
      enabled: true,
      source: 'builtin',
      commandId: 'pane.nav.refresh',
      execute: handlers.refresh,
    },
  ];
}

export function adaptPluginToolbarActions(
  contributions: readonly ToolbarActionContribution[],
  context: ToolbarContext,
  commands: PluginCommandPresentationHost,
): PresentedCommand[] {
  return contributions.map(contribution => presentPluginCommand(contribution, context, commands, { source: 'toolbar' }));
}