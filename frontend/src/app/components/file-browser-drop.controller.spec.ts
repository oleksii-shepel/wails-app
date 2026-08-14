import { describe, expect, it, vi } from 'vitest';

import { FileBrowserDropController, type FileBrowserDropHost, type FileBrowserDropPane } from './file-browser-drop.controller';
import type { SourceCapabilities } from '../services/source-capabilities';

const mutable: SourceCapabilities = { read: true, create: true, delete: true, rename: true, trash: true };
const readOnly: SourceCapabilities = { read: true, create: false, delete: false, rename: false, trash: false };

function pane(capabilities: SourceCapabilities): FileBrowserDropPane {
  return {
    getDestinationDir: () => 'filesystem://target',
    isInArchive: () => false,
    sourceCapabilities: capabilities,
    browserCommands: {
      navigation: {
        navigateUp: vi.fn(),
        refresh: vi.fn(),
        openSelectedItem: vi.fn(),
      },
    },
  };
}

function host(target: FileBrowserDropPane): FileBrowserDropHost {
  return {
    getActivePane: () => target,
    getLeftPane: () => target,
    getRightPane: () => target,
    getPanesContainer: () => undefined,
    getLeftPaneWidth: () => 50,
    apiService: { batchCopy: vi.fn().mockResolvedValue({ failed: 0 }) } as unknown as FileBrowserDropHost['apiService'],
    confirmDialog: { alert: vi.fn() } as unknown as FileBrowserDropHost['confirmDialog'],
    buildDestinationPath: vi.fn(async (source: string, destination: string) => `${destination}/${source.split('/').pop()}`),
  };
}

describe('FileBrowserDropController destination capabilities', () => {
  it('copies native drops into writable destinations', async () => {
    const targetPane = pane(mutable);
    const targetHost = host(targetPane);

    await new FileBrowserDropController().handleNativeFileDrop(targetHost, 0, 0, ['C:/source/file.txt']);

    expect(targetHost.apiService.batchCopy).toHaveBeenCalledTimes(1);
    expect(targetPane.browserCommands.navigation.refresh).toHaveBeenCalledTimes(1);
  });

  it('rejects native drops into read-only destinations', async () => {
    const targetHost = host(pane(readOnly));

    await new FileBrowserDropController().handleNativeFileDrop(targetHost, 0, 0, ['C:/source/file.txt']);

    expect(targetHost.apiService.batchCopy).not.toHaveBeenCalled();
    expect(targetHost.confirmDialog.alert).toHaveBeenCalledWith('Copy Not Available\nThe destination is read-only.');
  });
});
