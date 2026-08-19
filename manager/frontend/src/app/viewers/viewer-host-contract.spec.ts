import type { ViewerHostTabState } from './viewer-host-contract';
import { ViewerHostSurfaceComponent } from './viewer-host-surface.component';

describe('Viewer host tab contract', () => {
  it('covers the pane bindings consumed by the viewer host', () => {
    const tab = {} as ViewerHostTabState;
    const host = {} as ViewerHostSurfaceComponent;

    host.targetPath = tab.targetPath;
    host.fileList = tab.fileList;
    host.archiveContext = tab.archiveContext;
    host.viewerId = tab.viewerId;
    host.autoFollow = tab.autoFollow;
    host.isPinned = tab.isPinned;
    host.isClosable = tab.isClosable;

    expect(host).toBeDefined();
  });
});