import { ViewerHostSurfaceComponent, type ViewerHostChromeState } from './viewer-host-surface.component';

describe('ViewerHostSurfaceComponent state binding', () => {
  it('derives chrome flags from one viewerState input', () => {
    const host = Object.create(ViewerHostSurfaceComponent.prototype) as ViewerHostSurfaceComponent;
    const state: ViewerHostChromeState = {
      autoFollow: true,
      sourceTabId: 'source-tab',
      sourceDir: 'C:/source',
      isPinned: true,
      isClosable: false,
    };

    host.viewerState = state;

    expect(host.autoFollow).toBeTrue();
    expect(host.sourceAvailable).toBeTrue();
    expect(host.isPinned).toBeTrue();
    expect(host.isClosable).toBeFalse();
  });
});
