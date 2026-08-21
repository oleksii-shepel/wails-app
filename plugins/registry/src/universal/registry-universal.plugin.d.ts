import { RegistryKeyEditorService } from '../registry-key-editor.service';
import { RegistryValueEditorService } from '../registry-value-editor.service';
import { type RegistryPluginApi, type RegistryPluginDefinition } from '../registry.plugin';
import { type RegistryBackendTransport, type RegistryDialogHost } from './registry-backend';
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
export declare function activateUniversalRegistryPlugin(host: UniversalRegistryHost): Promise<ActiveUniversalRegistryPlugin>;
