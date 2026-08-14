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
});
