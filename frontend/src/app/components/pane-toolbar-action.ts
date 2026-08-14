import type { ActionAvailability } from '../services/action-availability';
import type { ToolbarActionContribution, ToolbarContext } from '../plugin/plugin-registry-api';

export type PaneToolbarActionIcon = 'new-folder' | 'delete' | 'copy' | 'move' | 'refresh' | 'plugin';

export interface PaneToolbarAction {
  readonly id: string;
  readonly label: string;
  readonly icon: PaneToolbarActionIcon;
  readonly iconText?: string;
  readonly shortcut?: string;
  readonly enabled: boolean;
  readonly source: 'builtin' | 'plugin';
  execute(): void | Promise<void>;
}

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
): PaneToolbarAction[] {
  return [
    {
      id: 'new-folder',
      label: 'New folder',
      icon: 'new-folder',
      enabled: availability.createFolder,
      source: 'builtin',
      execute: handlers.createFolder,
    },
    {
      id: 'delete',
      label: 'Delete',
      icon: 'delete',
      enabled: availability.deleteSelected,
      source: 'builtin',
      execute: handlers.deleteSelected,
    },
    {
      id: 'copy-to-other',
      label: 'Copy to other pane',
      icon: 'copy',
      shortcut: 'F5',
      enabled: availability.copyToOther,
      source: 'builtin',
      execute: handlers.copyToOther,
    },
    {
      id: 'move-to-other',
      label: 'Move to other pane',
      icon: 'move',
      shortcut: 'F6',
      enabled: availability.moveToOther,
      source: 'builtin',
      execute: handlers.moveToOther,
    },
    {
      id: 'refresh',
      label: 'Refresh',
      icon: 'refresh',
      shortcut: 'Ctrl+R',
      enabled: true,
      source: 'builtin',
      execute: handlers.refresh,
    },
  ];
}

export function adaptPluginToolbarActions(
  contributions: readonly ToolbarActionContribution[],
  context: ToolbarContext,
): PaneToolbarAction[] {
  return contributions.map((contribution) => ({
    id: `plugin:${contribution.id}`,
    label: contribution.label,
    icon: 'plugin',
    iconText: contribution.icon,
    shortcut: contribution.shortcut,
    enabled: true,
    source: 'plugin',
    execute: () => contribution.execute(context),
  }));
}
