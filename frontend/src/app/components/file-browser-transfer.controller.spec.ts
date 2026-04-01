import { describe, expect, it, vi } from 'vitest';

import { FileBrowserTransferController, type FileBrowserTransferHost, type FileBrowserTransferPane } from './file-browser-transfer.controller';

function createPane(overrides: Partial<FileBrowserTransferPane> = {}): FileBrowserTransferPane {
  return {
    currentPath: 'filesystem://target/current',
    selectedFiles: new Set<string>(),
    archiveStack: [],
    getDestinationDir: () => 'filesystem://target/selected-folder',
    getTreeArchiveSelectionContext: () => null,
    initiatePendingMove: vi.fn(),
    isInArchive: () => false,
    browserCommands: {
      navigation: {
        refresh: vi.fn(),
      },
    },
    ...overrides,
  };
}

function createHost(overrides: Partial<FileBrowserTransferHost> = {}): FileBrowserTransferHost {
  return {
    leftPane: createPane({ currentPath: 'filesystem://source/current' }),
    rightPane: createPane(),
    apiService: {
      batchCopy: vi.fn().mockResolvedValue({ failed: 0 }),
      batchMove: vi.fn().mockResolvedValue({ failed: 0 }),
    } as unknown as FileBrowserTransferHost['apiService'],
    archiveService: {
      copy: vi.fn(),
      copyByContext: vi.fn(),
      moveByContext: vi.fn(),
    } as unknown as FileBrowserTransferHost['archiveService'],
    confirmDialog: {
      alert: vi.fn(),
    } as unknown as FileBrowserTransferHost['confirmDialog'],
    buildDestinationPath: vi.fn(async (sourcePath: string, targetDir: string) => `${targetDir}/${sourcePath.split('/').pop()}`),
    ...overrides,
  };
}

describe('FileBrowserTransferController', () => {
  it('copies into the other pane current path instead of its selected directory', async () => {
    const controller = new FileBrowserTransferController();
    const host = createHost();

    await controller.copy(host, 'left', ['filesystem://source/current/file.txt']);

    expect(host.buildDestinationPath).toHaveBeenCalledWith(
      'filesystem://source/current/file.txt',
      'filesystem://target/current',
    );
    expect((host.apiService.batchCopy as ReturnType<typeof vi.fn>)).toHaveBeenCalledWith([
      {
        source: 'filesystem://source/current/file.txt',
        destination: 'filesystem://target/current/file.txt',
      },
    ], true);
  });

  it('moves into the other pane current path instead of its selected directory', async () => {
    const controller = new FileBrowserTransferController();
    const host = createHost();

    await controller.move(host, 'left', ['filesystem://source/current/file.txt']);

    expect(host.buildDestinationPath).toHaveBeenCalledWith(
      'filesystem://source/current/file.txt',
      'filesystem://target/current',
    );
    expect((host.apiService.batchMove as ReturnType<typeof vi.fn>)).toHaveBeenCalledWith([
      {
        source: 'filesystem://source/current/file.txt',
        destination: 'filesystem://target/current/file.txt',
      },
    ], true);
  });
});