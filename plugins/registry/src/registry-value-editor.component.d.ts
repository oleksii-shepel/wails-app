import { ChangeDetectorRef, OnDestroy, OnInit } from '@angular/core';
import { RegistryValueEditorService, type RegistryValueEditRequest } from './registry-value-editor.service';
import * as i0 from "@angular/core";
export declare class RegistryValueEditorComponent implements OnInit, OnDestroy {
    private readonly editor;
    private readonly cdr;
    private dataInput?;
    readonly registryTypes: readonly ["REG_SZ", "REG_EXPAND_SZ", "REG_BINARY", "REG_DWORD", "REG_DWORD_BIG_ENDIAN", "REG_MULTI_SZ", "REG_QWORD", "REG_NONE"];
    request: RegistryValueEditRequest | null;
    type: string;
    data: string;
    private readonly destroy$;
    constructor(editor: RegistryValueEditorService, cdr: ChangeDetectorRef);
    ngOnInit(): void;
    ngOnDestroy(): void;
    save(): void;
    cancel(): void;
    onKeyDown(event: KeyboardEvent): void;
    static ɵfac: i0.ɵɵFactoryDeclaration<RegistryValueEditorComponent, never>;
    static ɵcmp: i0.ɵɵComponentDeclaration<RegistryValueEditorComponent, "app-registry-value-editor", never, {}, {}, never, never, true, never>;
}
