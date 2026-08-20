import { WindowStateService } from './window-state.service';

describe('WindowStateService', () => {
  function createService(): WindowStateService {
    return new WindowStateService({ runOutsideAngular: (fn: () => void) => fn() } as any);
  }

  it('uses the backend work-area normalizer when available', async () => {
    const service = createService() as any;
    const originalGo = (window as any).go;
    (window as any).go = {
      app: {
        App: {
          NormalizeWindowStateForSave: jasmine.createSpy('NormalizeWindowStateForSave').and.resolveTo({
            width: 1600,
            height: 900,
            x: 0,
            y: 0,
            maximised: false,
            hasPosition: true,
          }),
        },
      },
    };

    try {
      const snapshot = await service.normalizeWindowStateForSave({
        width: 2200,
        height: 1400,
        x: 1800,
        y: 900,
        maximised: false,
        hasPosition: true,
      });

      expect(snapshot.width).toBe(1600);
      expect(snapshot.height).toBe(900);
      expect(snapshot.x).toBe(0);
      expect(snapshot.y).toBe(0);
      expect((window as any).go.app.App.NormalizeWindowStateForSave).toHaveBeenCalled();
    } finally {
      (window as any).go = originalGo;
    }
  });

  it('falls back to the original state when the backend helper is unavailable', async () => {
    const service = createService() as any;
    const originalGo = (window as any).go;
    (window as any).go = undefined;

    try {
      const original = {
        width: 1800,
        height: 1000,
        x: 400,
        y: 300,
        maximised: false,
        hasPosition: false,
      };
      const snapshot = await service.normalizeWindowStateForSave(original);

      expect(snapshot).toEqual(original);
    } finally {
      (window as any).go = originalGo;
    }
  });
});
