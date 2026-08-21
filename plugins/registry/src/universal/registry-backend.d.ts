import { type RegistryFileInfo } from '../registry-model';
import type { RegistryPanelSourceHost } from '../registry-panel-source';
import type { RegistryPluginOperations } from '../registry.plugin';
export interface RegistryBackendTransport {
    call<T = unknown>(method: string, params?: Readonly<Record<string, unknown>>): Promise<T>;
}
export interface RegistryDialogHost {
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
interface RegistryListingResult {
    path: string;
    entries: RegistryFileInfo[];
}
export interface RegistryBackendHello {
    protocol: number;
    plugin: string;
    target: string;
}
interface RegistryReadResult {
    content?: string;
}
export declare class RegistryBackendError extends Error {
    readonly method?: string | undefined;
    readonly causeValue?: unknown | undefined;
    constructor(message: string, method?: string | undefined, causeValue?: unknown | undefined);
}
export declare function normalizeRegistryPath(path: string): string;
export declare function joinRegistryPath(parts: readonly string[]): string;
export declare class RegistryBackendClient {
    private readonly transport;
    private helloResult?;
    constructor(transport: RegistryBackendTransport);
    initialize(): Promise<RegistryBackendHello>;
    private call;
    list(path: string): Promise<RegistryListingResult>;
    read(path: string): Promise<RegistryReadResult>;
    stat(path: string): Promise<RegistryFileInfo>;
    createKey(path: string): Promise<unknown>;
    write(path: string, type: string, data: string): Promise<unknown>;
    delete(path: string): Promise<unknown>;
    rename(path: string, newName: string): Promise<unknown>;
}
export declare function createRegistryPanelSourceHost(backend: RegistryBackendClient): RegistryPanelSourceHost;
export declare function createRegistryOperations(backend: RegistryBackendClient, dialogs: RegistryDialogHost): RegistryPluginOperations;
export declare const REGISTRY_HIVES: readonly ["HKEY_CURRENT_USER", "HKEY_LOCAL_MACHINE", "HKEY_CLASSES_ROOT", "HKEY_USERS", "HKEY_CURRENT_CONFIG"];
export type RegistryHive = typeof REGISTRY_HIVES[number];
export declare function registryHivePath(hive: RegistryHive): string;
export declare function suggestRegistryChildren(backend: RegistryBackendClient, path: string, prefix?: string): Promise<RegistryFileInfo[]>;
export {};
