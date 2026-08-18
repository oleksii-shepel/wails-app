import { ViewerHostSurfaceComponent } from './viewer-host-surface.component';

describe('ViewerHostSurfaceComponent chrome', () => {
  function host(): ViewerHostSurfaceComponent {
    return new ViewerHostSurfaceComponent(
      {} as any,
      {} as any,
      { markForCheck() {}, detectChanges() {} } as any,
    );
  }

  it('derives one concise identity from the active file', () => {
    const value = host();
    value.targetPath = 'filesystem://c/Pictures/photo.jpg';
    value.viewerId = 'image';

    expect(value.displayName).toBe('photo.jpg');
    expect(value.displayKind).toBe('image');
  });

  it('invalidates asynchronous resolve work on destroy', () => {
    const value = host();
    const before = (value as any).resolveSeq;
    value.ngOnDestroy();
    expect((value as any).resolveSeq).toBe(before + 1);
  });

  it('only exposes generic actions when the mounted viewer supports them', () => {
    const value = host();
    Object.defineProperty(value, 'mountedViewer', {
      get: () => ({ zoomIn() {} }),
    });

    expect(value.supports('zoomIn')).toBeTrue();
    expect(value.supports('showNext')).toBeFalse();
  });
});
