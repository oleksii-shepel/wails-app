import { ReassemblyPaneIntelligenceService } from './reassembly-pane-intelligence.service';

describe('ReassemblyPaneIntelligenceService', () => {
  let service: ReassemblyPaneIntelligenceService;

  beforeEach(() => {
    localStorage.clear();
    service = new ReassemblyPaneIntelligenceService();
  });

  it('persists modes independently per workspace', () => {
    service.saveWorkspace('alpha', {
      linkedNavigation: true,
      mirrorPresentation: true,
      mirrorFilter: true,
      syncScroll: false,
      followSelection: true,
      roots: { left: 'C:/src', right: 'D:/backup' },
    });

    service.saveWorkspace('beta', {
      linkedNavigation: false,
      mirrorPresentation: false,
      mirrorFilter: false,
      syncScroll: true,
      followSelection: false,
      roots: null,
    });

    expect(service.loadWorkspace('alpha')).toEqual(jasmine.objectContaining({
      linkedNavigation: true,
      mirrorPresentation: true,
      mirrorFilter: true,
      followSelection: true,
      roots: { left: 'C:/src', right: 'D:/backup' },
    }));

    expect(service.loadWorkspace('beta')).toEqual(jasmine.objectContaining({
      linkedNavigation: false,
      syncScroll: true,
      roots: null,
    }));
  });

  it('never restores filter mirroring without presentation mirroring', () => {
    service.saveWorkspace('alpha', {
      linkedNavigation: false,
      mirrorPresentation: false,
      mirrorFilter: true,
      syncScroll: false,
      followSelection: false,
      roots: null,
    });

    expect(service.loadWorkspace('alpha').mirrorFilter).toBeFalse();
  });

  it('creates, updates, lists and deletes named pane-pair presets', () => {
    const first = service.savePreset({
      name: 'Source ↔ Backup',
      roots: { left: 'C:/src', right: 'D:/backup' },
      modes: {
        linkedNavigation: true,
        mirrorPresentation: true,
        mirrorFilter: false,
        syncScroll: true,
        followSelection: true,
      },
    });

    const updated = service.savePreset({
      id: first.id,
      name: 'Source ↔ Archive',
      roots: { left: 'C:/src', right: 'E:/archive' },
      modes: first.modes,
    });

    expect(updated.id).toBe(first.id);
    expect(service.listPresets().map(item => item.name))
      .toEqual(['Source ↔ Archive']);

    service.deletePreset(first.id);
    expect(service.listPresets()).toEqual([]);
  });
});
