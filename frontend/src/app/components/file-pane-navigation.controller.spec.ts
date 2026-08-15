import { describe, expect, it, vi } from 'vitest';

import { FilePaneNavigationController } from './file-pane-navigation.controller';

function remoteHost(options?: { mounted?: boolean; mountedRoot?: string | null }) {
  const promptForRemoteMount = vi.fn(async () => options?.mountedRoot ?? null);
  return {
    apiService: {
      remoteListConnections: vi.fn(async () => options?.mounted
        ? [{ protocol: 'gdrive', publicPath: 'gdrive://work' }]
        : []),
    },
    promptForRemoteMount,
  } as any;
}

describe('FilePaneNavigationController remote restore policy', () => {
  it('does not prompt for an unavailable cloud mount during non-interactive startup restore', async () => {
    const controller = new FilePaneNavigationController();
    const host = remoteHost();

    const resolved = await (controller as any).resolveRemotePathForLoad(
      host,
      'gdrive://work/Projects',
    );

    expect(resolved).toBeNull();
    expect(host.promptForRemoteMount).not.toHaveBeenCalled();
  });

  it('keeps an already mounted cloud location during startup restore', async () => {
    const controller = new FilePaneNavigationController();
    const host = remoteHost({ mounted: true });

    const resolved = await (controller as any).resolveRemotePathForLoad(
      host,
      'gdrive://work/Projects',
    );

    expect(resolved).toBe('gdrive://work/Projects');
    expect(host.promptForRemoteMount).not.toHaveBeenCalled();
  });

  it('prompts only for explicit interactive navigation and preserves the suffix', async () => {
    const controller = new FilePaneNavigationController();
    const host = remoteHost({ mountedRoot: 'gdrive://work' });

    const resolved = await (controller as any).resolveRemotePathForLoad(
      host,
      'gdrive://work/Projects/2026',
      { interactive: true },
    );

    expect(host.promptForRemoteMount).toHaveBeenCalledWith('gdrive', 'work');
    expect(resolved).toBe('gdrive://work/Projects/2026');
  });

  it('switches to a custom surface without loading it as a directory', async () => {
    const controller = new FilePaneNavigationController();
    const tab = {
      id: 'console-tab',
      title: 'Console',
      path: 'surface://console/session/main',
      location: { location: 'surface://console/session/main', scheme: 'surface', kind: 'surface' },
      surfaceId: 'console',
      isActive: false,
      isPinned: false,
      createdAt: 1,
    } as any;
    const loadPathTransparently = vi.fn(async () => undefined);
    const emitActiveTabChange = vi.fn();
    const host = {
      tabs: [
        { id: 'dir-tab', title: 'Dir', path: '/', surfaceId: 'dir', isActive: true, isPinned: false, createdAt: 1 },
        tab,
      ],
      currentTabId: 'dir-tab',
      currentPath: '/',
      paneId: 'left',
      workspaceService: {
        updateTabPath: vi.fn(),
        switchTab: vi.fn(),
        getTabs: vi.fn(() => [tab]),
        getArchiveState: vi.fn(() => null),
      },
      isInArchive: vi.fn(() => false),
      clearArchiveState: vi.fn(),
      loadPathTransparently,
      emitActiveTabChange,
      activeDirTabId: 'dir-tab',
    } as any;

    await controller.onTabChange(host, tab);

    expect(host.workspaceService.switchTab).toHaveBeenCalledWith('left', 'console-tab');
    expect(emitActiveTabChange).toHaveBeenCalledWith(tab);
    expect(loadPathTransparently).not.toHaveBeenCalled();
    expect(host.activeDirTabId).toBe('dir-tab');
  });

});
