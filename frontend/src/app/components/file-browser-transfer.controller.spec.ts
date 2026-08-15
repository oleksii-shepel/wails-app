import { describe, expect, it, vi } from 'vitest';

import { FileBrowserTransferController, type FileBrowserTransferHost, type FileBrowserTransferPane } from './file-browser-transfer.controller';

function createPane(overrides: Partial<FileBrowserTransferPane> = {}): FileBrowserTransferPane {
  return {
    currentPath: 'filesystem://target/current',
    selectedFiles: new Set<string>(),
    archiveStack: [],
    sourceCapabilities: { read: true, create: true, delete: true, rename: true, trash: true },
    getDestinationDir: () => 'filesystem://target/selected-folder',
    getTreeArchiveSelectionContext: () => null,
    initiatePendingMove: vi.fn(),
    isInArchive: () => false,
    browserCommands: {
      navigation: {
        navigateUp: vi.fn(),
        refresh: vi.fn(),
        openSelectedItem: vi.fn(),
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
    get confirmBeforeCopy() { return false; },
    get confirmBeforeMove() { return false; },
    buildDestinationPath: vi.fn(async (sourcePath: string, targetDir: string) => `${targetDir}/${sourcePath.split('/').pop()}`),
    ...overrides,
  };
}

describe('FileBrowserTransferController', () => {
  it('copies into the other pane current path instead of its selected directory', async () => {
    const controller = new FileBrowserTransferController();
    const host = createHost();
    const destinationRefresh = host.rightPane?.browserCommands.navigation.refresh as ReturnType<typeof vi.fn>;

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
    expect(destinationRefresh).toHaveBeenCalledTimes(1);
  });

  it('moves into the other pane current path instead of its selected directory', async () => {
    const controller = new FileBrowserTransferController();
    const host = createHost();
    const sourceRefresh = host.leftPane?.browserCommands.navigation.refresh as ReturnType<typeof vi.fn>;
    const destinationRefresh = host.rightPane?.browserCommands.navigation.refresh as ReturnType<typeof vi.fn>;

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
    expect(sourceRefresh).toHaveBeenCalledTimes(1);
    expect(destinationRefresh).toHaveBeenCalledTimes(1);
  });

  it('allows copying from a read-only source into a writable destination', async () => {
    const host = createHost({
      leftPane: createPane({
        currentPath: 's3://bucket',
        sourceCapabilities: { read: true, create: false, delete: false, rename: false, trash: false },
      }),
    });
    await new FileBrowserTransferController().copy(host, 'left', ['s3://bucket/file.txt']);
    expect(host.apiService.batchCopy).toHaveBeenCalledTimes(1);
  });

  it('blocks copying into a read-only destination', async () => {
    const host = createHost({
      rightPane: createPane({
        currentPath: 's3://bucket',
        sourceCapabilities: { read: true, create: false, delete: false, rename: false, trash: false },
      }),
    });
    await new FileBrowserTransferController().copy(host, 'left', ['filesystem://source/current/file.txt']);
    expect(host.apiService.batchCopy).not.toHaveBeenCalled();
    expect(host.confirmDialog.alert).toHaveBeenCalledWith('Copy Not Available\nThe destination is read-only.');
  });

  it('blocks moving from a read-only source', async () => {
    const host = createHost({
      leftPane: createPane({
        currentPath: 's3://bucket',
        sourceCapabilities: { read: true, create: false, delete: false, rename: false, trash: false },
      }),
    });
    await new FileBrowserTransferController().move(host, 'left', ['s3://bucket/file.txt']);
    expect(host.apiService.batchMove).not.toHaveBeenCalled();
    expect(host.confirmDialog.alert).toHaveBeenCalledWith('Move Not Available\nThe source is read-only and cannot be moved. You can copy it instead.');
  });


  it('can skip the generic copy confirmation after an explicit sync-plan approval', async () => {
    const host = createHost({
      confirmBeforeCopy: true,
      confirmDialog: { confirm: vi.fn(), alert: vi.fn() } as unknown as FileBrowserTransferHost['confirmDialog'],
    });
    await new FileBrowserTransferController().copy(host, 'left', ['filesystem://left/file.txt'], { skipConfirmation: true });
    expect(host.confirmDialog.confirm).not.toHaveBeenCalled();
    expect(host.apiService.batchCopy).toHaveBeenCalledTimes(1);
  });

});