import {
  RegistryKeyEditorService,
} from './registry-key-editor.service';
import {
  RegistryValueEditorService,
} from './registry-value-editor.service';
import {
  RegistryPanelSource,
  type RegistryPanelSourceHost,
} from './registry-panel-source';
import type { RegistryFileInfo } from './registry-model';

export interface RegistryCommandInvocation {
  source?: string;
  context?: Readonly<Record<string, unknown>>;
}

export interface RegistryCommandDescriptor {
  id: string;
  title: string;
  description?: string;
  category?: string;
  execute(invocation?: RegistryCommandInvocation): void | Promise<void>;
  isEnabled?(invocation?: RegistryCommandInvocation): boolean;
}

export interface RegistryMenuContext {
  currentDirectory: string;
  targetPath: string;
  selectedPaths: string[];
}

export interface RegistryToolbarContext {
  selectedPaths: string[];
  currentDirectory: string;
}

export interface RegistryToolbarContribution {
  id: string;
  commandId?: string;
  label?: string;
  score(context: RegistryToolbarContext): number;
}

export interface RegistryContextMenuContribution {
  id: string;
  commandId?: string;
  label?: string;
  score(context: RegistryMenuContext): number;
}

export interface RegistryPluginApi {
  registerPanelSource(source: RegistryPanelSource): void;
  unregisterPanelSource(id: string): void;
  registerCommand(command: RegistryCommandDescriptor): void;
  unregisterCommand(id: string): void;
  registerToolbarAction(action: RegistryToolbarContribution): void;
  unregisterToolbarAction(id: string): void;
  registerContextMenuItem(item: RegistryContextMenuContribution): void;
  unregisterContextMenuItem(id: string): void;
}

export interface RegistryPluginOperations {
  joinPaths(parts: string[]): Promise<string>;
  createDirectory(path: string, recursive: boolean): Promise<unknown>;
  writeFile(path: string, content: string, encoding: string): Promise<unknown>;
  getFileInfo(path: string): Promise<RegistryFileInfo>;
  moveFile(source: string, destination: string): Promise<unknown>;
  readFile(path: string, encoding: string): Promise<{ content?: string }>;
  writeRegistryValue(path: string, type: string, data: string): Promise<unknown>;
  deletePath(path: string, recursive: boolean, useTrash: boolean): Promise<unknown>;
  prompt(request: {
    title: string;
    message: string;
    confirmText: string;
    defaultValue?: string;
    historyKey?: string;
  }): Promise<string | null>;
  confirm(request: {
    title: string;
    message: string;
    confirmText: string;
    danger?: boolean;
  }): Promise<boolean>;
}

export interface RegistryPluginOptions {
  panelSource: RegistryPanelSourceHost;
  operations: RegistryPluginOperations;
  keyEditor: RegistryKeyEditorService;
  valueEditor: RegistryValueEditorService;
}

export interface RegistryPluginBackend {
  id: string;
  onRegister(api: RegistryPluginApi): void;
  onUnregister(api: RegistryPluginApi): void;
}

export interface RegistryPluginDescriptor {
  id: string;
  name: string;
  version: string;
  description: string;
  schemes: string[];
}

export interface RegistryPluginDefinition {
  backend: RegistryPluginBackend;
  descriptor: RegistryPluginDescriptor;
}

const COMMAND = {
  newKey: 'reassembly.registry.newKey',
  newValue: 'reassembly.registry.newValue',
  edit: 'reassembly.registry.edit',
  delete: 'reassembly.registry.delete',
} as const;

const TOOLBAR = {
  newKey: 'reassembly.registry.newKey.toolbar',
  newValue: 'reassembly.registry.newValue.toolbar',
  edit: 'reassembly.registry.edit.toolbar',
  delete: 'reassembly.registry.delete.toolbar',
  refresh: 'reassembly.registry.refresh.toolbar',
} as const;

const MENU = {
  newKey: 'reassembly.registry.newKey.menu',
  newValue: 'reassembly.registry.newValue.menu',
  edit: 'reassembly.registry.edit.menu',
  delete: 'reassembly.registry.delete.menu',
} as const;

function isRegistry(path: string): boolean {
  return /^registry:\/\//i.test((path ?? '').trim());
}

function commandContext(invocation?: RegistryCommandInvocation): RegistryMenuContext {
  const raw = invocation?.context ?? {};
  const selectedPaths = Array.isArray(raw['selectedPaths'])
    ? raw['selectedPaths'].filter((value): value is string => typeof value === 'string')
    : [];

  const currentDirectory =
    typeof raw['currentDirectory'] === 'string'
      ? raw['currentDirectory']
      : typeof raw['path'] === 'string'
        ? raw['path']
        : '';

  const targetPath =
    typeof raw['targetPath'] === 'string'
      ? raw['targetPath']
      : selectedPaths[0] ?? currentDirectory;

  return { currentDirectory, targetPath, selectedPaths };
}

async function refreshAfter(
  invocation: RegistryCommandInvocation | undefined,
  action: () => Promise<void>,
): Promise<void> {
  await action();
  const refresh = invocation?.context?.['refresh'];
  if (typeof refresh === 'function') {
    await Promise.resolve(refresh());
  }
}

export function createRegistryPlugin(
  options: RegistryPluginOptions,
): RegistryPluginDefinition {
  const ops = options.operations;

  const createKey = async (context: RegistryMenuContext): Promise<void> => {
    if (!isRegistry(context.currentDirectory)) return;

    const name = await options.keyEditor.open({
      title: 'New Registry Key',
      name: '',
      confirmText: 'Create',
    });
    if (!name) return;

    const path = await ops.joinPaths([context.currentDirectory, name]);
    await ops.createDirectory(path, false);
  };

  const createValue = async (context: RegistryMenuContext): Promise<void> => {
    if (!isRegistry(context.currentDirectory)) return;

    const name = (await ops.prompt({
      title: 'New Registry Value',
      message: 'Enter the value name.',
      confirmText: 'Next',
      historyKey: 'reassembly.registry.newValue.name',
    }))?.trim();
    if (!name) return;

    const path = await ops.joinPaths([context.currentDirectory, name]);
    const value = await options.valueEditor.edit({
      path,
      name,
      type: 'REG_SZ',
      data: '',
    });
    if (!value) return;

    await ops.writeRegistryValue(path, value.type, value.data);
  };

  const editEntry = async (context: RegistryMenuContext): Promise<void> => {
    if (
      !isRegistry(context.targetPath) ||
      context.targetPath === context.currentDirectory
    ) {
      return;
    }

    const info = await ops.getFileInfo(context.targetPath);
    const kind = String(info.registryKind ?? '').toLowerCase();

    if (kind === 'key') {
      const name = await options.keyEditor.open({
        title: `Edit Registry Key — ${info.name}`,
        name: info.name,
        confirmText: 'Rename',
      });
      if (!name || name === info.name) return;

      const separator = context.targetPath.lastIndexOf('/');
      if (separator < 0) return;
      const parent = context.targetPath.slice(0, separator);
      const destination = await ops.joinPaths([parent, name]);
      await ops.moveFile(context.targetPath, destination);
      return;
    }

    if (kind !== 'value') return;

    const current = await ops.readFile(context.targetPath, 'utf8');
    const edit = await options.valueEditor.edit({
      path: context.targetPath,
      name: info.name,
      type: info.registryValueType || 'REG_SZ',
      data: current.content ?? '',
    });
    if (!edit) return;

    await ops.writeRegistryValue(
      context.targetPath,
      edit.type,
      edit.data,
    );
  };

  const deleteEntry = async (context: RegistryMenuContext): Promise<void> => {
    if (
      !isRegistry(context.targetPath) ||
      context.targetPath === context.currentDirectory
    ) {
      return;
    }

    const confirmed = await ops.confirm({
      title: 'Delete Registry Entry',
      message: `Delete ${context.targetPath}?`,
      confirmText: 'Delete',
      danger: true,
    });
    if (!confirmed) return;

    await ops.deletePath(context.targetPath, true, false);
  };

  const source = new RegistryPanelSource(
    options.panelSource,
    async (context, entry) => {
      await editEntry({
        currentDirectory: context.path,
        targetPath: entry.path,
        selectedPaths: [entry.path],
      });
      return true;
    },
  );

  const backend: RegistryPluginBackend = {
    id: 'reassembly.registry',
    onRegister: api => {
      api.registerPanelSource(source);

      api.registerCommand({
        id: COMMAND.newKey,
        title: 'Registry: New Key',
        category: 'Registry',
        execute: invocation => refreshAfter(
          invocation,
          () => createKey(commandContext(invocation)),
        ),
        isEnabled: invocation =>
          isRegistry(commandContext(invocation).currentDirectory),
      });

      api.registerCommand({
        id: COMMAND.newValue,
        title: 'Registry: New Value',
        category: 'Registry',
        execute: invocation => refreshAfter(
          invocation,
          () => createValue(commandContext(invocation)),
        ),
        isEnabled: invocation =>
          isRegistry(commandContext(invocation).currentDirectory),
      });

      api.registerCommand({
        id: COMMAND.edit,
        title: 'Registry: Edit',
        category: 'Registry',
        execute: invocation => refreshAfter(
          invocation,
          () => editEntry(commandContext(invocation)),
        ),
        isEnabled: invocation => {
          const context = commandContext(invocation);
          return isRegistry(context.targetPath) &&
            context.targetPath !== context.currentDirectory;
        },
      });

      api.registerCommand({
        id: COMMAND.delete,
        title: 'Registry: Delete Entry',
        category: 'Registry',
        execute: invocation => refreshAfter(
          invocation,
          () => deleteEntry(commandContext(invocation)),
        ),
        isEnabled: invocation => {
          const context = commandContext(invocation);
          return isRegistry(context.targetPath) &&
            context.targetPath !== context.currentDirectory;
        },
      });

      api.registerToolbarAction({
        id: TOOLBAR.newKey,
        commandId: COMMAND.newKey,
        label: 'New key',
        score: context => isRegistry(context.currentDirectory) ? 100 : 0,
      });

      api.registerToolbarAction({
        id: TOOLBAR.newValue,
        commandId: COMMAND.newValue,
        label: 'New value',
        score: context => isRegistry(context.currentDirectory) ? 99 : 0,
      });

      api.registerToolbarAction({
        id: TOOLBAR.edit,
        commandId: COMMAND.edit,
        label: 'Edit',
        score: context =>
          isRegistry(context.currentDirectory) && context.selectedPaths.length
            ? 98
            : 0,
      });

      api.registerToolbarAction({
        id: TOOLBAR.delete,
        commandId: COMMAND.delete,
        label: 'Delete',
        score: context =>
          isRegistry(context.currentDirectory) && context.selectedPaths.length
            ? 97
            : 0,
      });

      api.registerToolbarAction({
        id: TOOLBAR.refresh,
        commandId: 'pane.nav.refresh',
        label: 'Refresh',
        score: context => isRegistry(context.currentDirectory) ? 96 : 0,
      });

      api.registerContextMenuItem({
        id: MENU.newKey,
        commandId: COMMAND.newKey,
        label: 'New Key',
        score: context =>
          isRegistry(context.currentDirectory) &&
          context.targetPath === context.currentDirectory
            ? 95
            : 0,
      });

      api.registerContextMenuItem({
        id: MENU.newValue,
        commandId: COMMAND.newValue,
        label: 'New Value',
        score: context =>
          isRegistry(context.currentDirectory) &&
          context.targetPath === context.currentDirectory
            ? 94
            : 0,
      });

      api.registerContextMenuItem({
        id: MENU.edit,
        commandId: COMMAND.edit,
        label: 'Edit',
        score: context =>
          isRegistry(context.targetPath) &&
          context.targetPath !== context.currentDirectory
            ? 90
            : 0,
      });

      api.registerContextMenuItem({
        id: MENU.delete,
        commandId: COMMAND.delete,
        label: 'Delete',
        score: context =>
          isRegistry(context.targetPath) &&
          context.targetPath !== context.currentDirectory
            ? 85
            : 0,
      });
    },
    onUnregister: api => {
      api.unregisterContextMenuItem(MENU.delete);
      api.unregisterContextMenuItem(MENU.edit);
      api.unregisterContextMenuItem(MENU.newValue);
      api.unregisterContextMenuItem(MENU.newKey);

      api.unregisterToolbarAction(TOOLBAR.refresh);
      api.unregisterToolbarAction(TOOLBAR.delete);
      api.unregisterToolbarAction(TOOLBAR.edit);
      api.unregisterToolbarAction(TOOLBAR.newValue);
      api.unregisterToolbarAction(TOOLBAR.newKey);

      api.unregisterCommand(COMMAND.delete);
      api.unregisterCommand(COMMAND.edit);
      api.unregisterCommand(COMMAND.newValue);
      api.unregisterCommand(COMMAND.newKey);

      api.unregisterPanelSource(source.id);
    },
  };

  return {
    backend,
    descriptor: {
      id: 'reassembly.registry',
      name: 'Windows Registry',
      version: '1.2.0',
      description: 'Registry panel source, presentation and Registry actions.',
      schemes: ['registry'],
    },
  };
}