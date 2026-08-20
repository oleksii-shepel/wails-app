import { RegistryPaneComponent } from './registry-pane.component';
import { RegistryFileInfo, RegistryFileType } from './registry-model';

function entry(
  path: string,
  overrides: Partial<RegistryFileInfo> = {},
): RegistryFileInfo {
  return {
    name: overrides.name ?? path.split('/').pop() ?? '',
    path,
    type: overrides.type ?? RegistryFileType.DIRECTORY,
    size: overrides.size ?? 0,
    created: 0,
    modified: 0,
    accessed: 0,
    permissions: '',
    isHidden: false,
    ...overrides,
  };
}

describe('RegistryPaneComponent', () => {
  let component: RegistryPaneComponent;

  beforeEach(() => {
    component = new RegistryPaneComponent();
  });

  it('treats top-level registry paths as hives', () => {
    expect(component.entryKind(entry('registry://HKEY_LOCAL_MACHINE'))).toBe('hive');
    expect(component.kindLabel(entry('registry://HKEY_LOCAL_MACHINE'))).toBe('Hive');
    expect(component.typeLabel(entry('registry://HKEY_LOCAL_MACHINE'))).toBe('Root hive');
  });

  it('uses a specialized root view at registry://', () => {
    component.path = 'registry://';
    component.entries = [
      entry('registry://HKEY_LOCAL_MACHINE', { registryKind: 'Key' }),
      entry('registry://HKEY_CURRENT_USER', { registryKind: 'Key' }),
    ];

    expect(component.isRootView).toBeTrue();
    expect(component.keyEntries.length).toBe(2);
    expect(component.valueEntries.length).toBe(0);
  });

  it('separates subkeys from values for key views', () => {
    component.path = 'registry://HKEY_CURRENT_USER/Software';
    component.entries = [
      entry('__parent__', { name: '..', registryKind: '' }),
      entry('registry://HKEY_CURRENT_USER/Software/TestKey', {
        registryKind: 'Key',
        type: RegistryFileType.DIRECTORY,
      }),
      entry('registry://HKEY_CURRENT_USER/Software/TestValue', {
        registryKind: 'Value',
        registryValueType: 'REG_SZ',
        type: RegistryFileType.FILE,
      }),
    ];

    expect(component.isRootView).toBeFalse();
    expect(component.parentEntry?.path).toBe('__parent__');
    expect(component.keyEntries.map(item => item.name)).toEqual(['TestKey']);
    expect(component.valueEntries.map(item => item.name)).toEqual(['TestValue']);
  });

  it('preserves value semantics and type labels', () => {
    const value = entry('registry://HKEY_CURRENT_USER/Software/Test/ValueName', {
      type: RegistryFileType.FILE,
      registryKind: 'Value',
      registryValueType: 'REG_DWORD',
      registryPreview: '1 (0x00000001)',
      size: 4,
    });

    expect(component.entryKind(value)).toBe('value');
    expect(component.kindLabel(value)).toBe('Value');
    expect(component.typeLabel(value)).toBe('REG_DWORD');
    expect(component.dataLabel(value)).toBe('1 (0x00000001)');
  });

  it('prefers the cursor value for the focused value inspector', () => {
    const first = entry('registry://HKEY_CURRENT_USER/Software/TestValueA', {
      type: RegistryFileType.FILE,
      registryKind: 'Value',
      registryValueType: 'REG_SZ',
    });
    const second = entry('registry://HKEY_CURRENT_USER/Software/TestValueB', {
      type: RegistryFileType.FILE,
      registryKind: 'Value',
      registryValueType: 'REG_DWORD',
    });

    component.entries = [first, second];
    component.cursorPath = second.path;
    component.selectedPaths = new Set([first.path]);

    expect(component.focusedValueEntry?.path).toBe(second.path);
  });

  it('falls back to a selected value when there is no value cursor', () => {
    const key = entry('registry://HKEY_CURRENT_USER/Software/TestKey', {
      type: RegistryFileType.DIRECTORY,
      registryKind: 'Key',
    });
    const value = entry('registry://HKEY_CURRENT_USER/Software/TestValue', {
      type: RegistryFileType.FILE,
      registryKind: 'Value',
      registryValueType: 'REG_SZ',
    });

    component.entries = [key, value];
    component.cursorPath = key.path;
    component.selectedPaths = new Set([value.path]);

    expect(component.focusedValueEntry?.path).toBe(value.path);
  });

  it('shows empty data for blank values', () => {
    const value = entry('registry://HKEY_CURRENT_USER/Software/Test/(default)', {
      type: RegistryFileType.FILE,
      registryKind: 'Value',
      registryValueType: 'REG_SZ',
      registryPreview: '',
      size: 0,
    });

    expect(component.dataLabel(value)).toBe('(empty)');
    expect(component.metaLabel(value)).toContain('REG_SZ value');
  });
});
