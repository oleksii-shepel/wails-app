import { ChangeDetectionStrategy, Component } from '@angular/core';
import { RegistryKeyEditorComponent } from './registry-key-editor.component';
import { RegistryValueEditorComponent } from './registry-value-editor.component';

/**
 * Single host-mounted overlay outlet for Registry-owned dialogs.
 * Manager does not own the individual editor components.
 */
@Component({
  selector: 'reassembly-registry-plugin-host',
  standalone: true,
  imports: [RegistryKeyEditorComponent, RegistryValueEditorComponent],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <app-registry-value-editor></app-registry-value-editor>
    <app-registry-key-editor></app-registry-key-editor>
  `,
})
export class RegistryPluginHostComponent {}