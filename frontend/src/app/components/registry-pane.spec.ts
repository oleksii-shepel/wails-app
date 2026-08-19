import { RegistryPaneComponent } from './registry-pane.component';

describe('RegistryPaneComponent', () => {
  const component = new RegistryPaneComponent();

  it('uses Registry semantics rather than filesystem semantics', () => {
    expect(component.typeLabel({
      path: '__parent__',
      name: '..',
      type: 1,
      size: 0,
    } as any)).toBe('Key');

    expect(component.typeLabel({
      path: 'registry://HKLM/Test/Enabled',
      name: 'Enabled',
      type: 0,
      size: 4,
      registryKind: 'Value',
      registryValueType: 'REG_DWORD',
    } as any)).toBe('REG_DWORD');
  });

  it('shows value payload inline but not for keys', () => {
    expect(component.payload({
      path: 'registry://HKLM/Test/Name',
      name: 'Name',
      type: 0,
      size: 10,
      registryKind: 'Value',
      registryPreview: 'Commander',
    } as any)).toBe('Commander');

    expect(component.payload({
      path: 'registry://HKLM/Test/Subkey',
      name: 'Subkey',
      type: 1,
      size: 0,
      registryKind: 'Key',
      registryPreview: 'should not render',
    } as any)).toBe('');
  });

  it('uses exactly Name Type Size Data in Details', () => {
    const source = (RegistryPaneComponent as any).ɵcmp?.template?.toString?.() ?? '';
    expect(component.detailsGridTemplate).toContain('minmax(180px, 1fr)');
    expect(component.detailsGridTemplate).toContain('minmax(220px, 1fr)');
    // The component owns four semantic columns; the leading 28px track is icon-only.
    expect(component.detailsGridTemplate.startsWith('28px ')).toBeTrue();
  });
});
