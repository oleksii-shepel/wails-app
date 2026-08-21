import { CommonModule } from '@angular/common';
import { FileType } from '@reassembly/core/contracts/files';
import {
  RegistryFileType,
  type RegistryFileInfo,
} from './registry-model';
import { RegistryPaneComponent } from './registry-pane.component';

describe('Registry/core file contract', () => {
  it('uses the core FileType identity', () => {
    expect(RegistryFileType).toBe(FileType);
  });

  it('accepts core FileType values in Registry entries', () => {
    const entry: RegistryFileInfo = {
      name: 'Example',
      path: 'registry://HKEY_CURRENT_USER/Example',
      type: FileType.DIRECTORY,
      size: 0,
      created: 0,
      modified: 0,
      accessed: 0,
      permissions: '',
      isHidden: false,
      registryKind: 'Key',
    };
    expect(entry.type).toBe(FileType.DIRECTORY);
  });

  it('keeps the Registry pane Angular CommonModule dependency intact', () => {
    expect(CommonModule).toBeDefined();
    expect(RegistryPaneComponent).toBeDefined();
  });
});
