import { RegistryKeyEditorService } from './registry-key-editor.service';
import { RegistryValueEditorService } from './registry-value-editor.service';
import { RegistryPanelSource, type RegistryPanelSourceHost } from './registry-panel-source';
import type { Type } from '@angular/core';
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
export interface RegistryPanelPresentationContribution {
    readonly id: string;
    score(context: {
        readonly sourceKind: string;
        readonly viewMode: string;
    }): number;
    load(): Promise<Type<unknown>> | Type<unknown>;
}
export interface RegistryPluginApi {
    registerPanelSource(source: RegistryPanelSource): void;
    unregisterPanelSource(id: string): void;
    registerPanelPresentation(presentation: RegistryPanelPresentationContribution): void;
    unregisterPanelPresentation(id: string): void;
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
    readFile(path: string, encoding: string): Promise<{
        content?: string;
    }>;
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
export declare function createRegistryPlugin(options: RegistryPluginOptions): RegistryPluginDefinition;
