import { KeybindingStorageService } from './keybinding-storage.service';

describe('KeybindingStorageService', () => {
  it('keeps default command palette and pin bindings visible when runtime snapshot is incomplete', () => {
    const service = new KeybindingStorageService();

    const merged = service.mergeWithDefaults([]);
    const ids = new Set(merged.map(binding => binding.id));

    expect(ids.has('pane.nav.openCommandPalette.ctrlShiftP')).toBe(true);
    expect(ids.has('pane.tabs.pin.ctrlAltP')).toBe(true);
  });
});
