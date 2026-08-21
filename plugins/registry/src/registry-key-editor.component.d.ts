import { ChangeDetectorRef, OnDestroy, OnInit } from '@angular/core';
import { RegistryKeyEditorService, type RegistryKeyEditRequest } from './registry-key-editor.service';
import * as i0 from "@angular/core";
export declare class RegistryKeyEditorComponent implements OnInit, OnDestroy {
    private readonly editor;
    private readonly cdr;
    private nameInput?;
    request: RegistryKeyEditRequest | null;
    name: string;
    private readonly destroy$;
    constructor(editor: RegistryKeyEditorService, cdr: ChangeDetectorRef);
    ngOnInit(): void;
    ngOnDestroy(): void;
    save(): void;
    cancel(): void;
    onDialogKeyDown(event: KeyboardEvent): void;
    static ɵfac: i0.ɵɵFactoryDeclaration<RegistryKeyEditorComponent, never>;
    static ɵcmp: i0.ɵɵComponentDeclaration<RegistryKeyEditorComponent, "app-registry-key-editor", never, {}, {}, never, never, true, never>;
}
