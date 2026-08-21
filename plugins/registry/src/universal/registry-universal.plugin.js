import { createRegistryPlugin, } from '../registry.plugin';
import { RegistryBackendClient, createRegistryOperations, createRegistryPanelSourceHost, } from './registry-backend';
export async function activateUniversalRegistryPlugin(host) {
    const backend = new RegistryBackendClient(host.backend);
    await backend.initialize();
    const definition = createRegistryPlugin({
        panelSource: createRegistryPanelSourceHost(backend),
        operations: createRegistryOperations(backend, host.dialogs),
        keyEditor: host.keyEditor,
        valueEditor: host.valueEditor,
    });
    definition.backend.onRegister(host.api);
    return {
        definition,
        dispose: () => definition.backend.onUnregister(host.api),
    };
}
//# sourceMappingURL=registry-universal.plugin.js.map