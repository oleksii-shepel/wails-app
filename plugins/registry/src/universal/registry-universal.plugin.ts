import { RegistryKeyEditorService } from '../registry-key-editor.service';
import { RegistryValueEditorService } from '../registry-value-editor.service';
import {
  createRegistryPlugin,
  type RegistryPluginApi,
  type RegistryPluginDefinition,
} from '../registry.plugin';
import {
  RegistryBackendClient,
  createRegistryOperations,
  createRegistryPanelSourceHost,
  type RegistryBackendTransport,
  type RegistryDialogHost,
} from './registry-backend';

/** Generic host contract. No Registry-specific filesystem service is exposed. */
export interface UniversalRegistryHost {
  readonly backend: RegistryBackendTransport;
  readonly dialogs: RegistryDialogHost;
  readonly api: RegistryPluginApi;
  readonly keyEditor: RegistryKeyEditorService;
  readonly valueEditor: RegistryValueEditorService;
}

export interface ActiveUniversalRegistryPlugin {
  readonly definition: RegistryPluginDefinition;
  dispose(): void;
}

export async function activateUniversalRegistryPlugin(
  host: UniversalRegistryHost,
): Promise<ActiveUniversalRegistryPlugin> {
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
