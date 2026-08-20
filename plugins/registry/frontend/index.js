/**
 * Universal Windows Registry frontend entry.
 *
 * Registry knowledge lives in this package. The host contributes only generic
 * plugin services and the process transport for backend/plugin.exe.
 */
const HIVE_COMMANDS = [
  ['reassembly.registry.open.currentUser', 'Registry: Open HKEY_CURRENT_USER', 'HKEY_CURRENT_USER'],
  ['reassembly.registry.open.localMachine', 'Registry: Open HKEY_LOCAL_MACHINE', 'HKEY_LOCAL_MACHINE'],
  ['reassembly.registry.open.classesRoot', 'Registry: Open HKEY_CLASSES_ROOT', 'HKEY_CLASSES_ROOT'],
  ['reassembly.registry.open.users', 'Registry: Open HKEY_USERS', 'HKEY_USERS'],
  ['reassembly.registry.open.currentConfig', 'Registry: Open HKEY_CURRENT_CONFIG', 'HKEY_CURRENT_CONFIG'],
];

export async function activate(ctx) {
  const module = await ctx.frontend.import('@reassembly/registry-plugin');
  const keyEditor = ctx.frontend.get(module.RegistryKeyEditorService);
  const valueEditor = ctx.frontend.get(module.RegistryValueEditorService);

  ctx.frontend.mount(module.RegistryPluginHostComponent);

  // Activation performs the backend hello/protocol handshake before Registry
  // becomes reachable through the host.
  const active = await module.activateUniversalRegistryPlugin({
    api: ctx.api,
    backend: ctx.backend,
    dialogs: ctx.dialogs,
    keyEditor,
    valueEditor,
  });

  ctx.schemes.register({ scheme: 'registry', sourceId: 'registry' });

  ctx.commands.register({
    id: 'reassembly.registry.open',
    title: 'Registry: Open',
    category: 'Registry',
    execute: () => ctx.navigation.open('registry://'),
  });

  for (const [id, title, hive] of HIVE_COMMANDS) {
    ctx.commands.register({
      id,
      title,
      category: 'Registry',
      execute: () => ctx.navigation.open(`registry://${hive}`),
    });
  }

  return () => active.dispose();
}
