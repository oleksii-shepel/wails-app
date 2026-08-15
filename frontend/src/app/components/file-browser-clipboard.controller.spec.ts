import { fn, spyOnFn, stubGlobal, restoreGlobals } from '../testing/jasmine-test-helpers';

import { FileBrowserClipboardController, type FileBrowserClipboardHost, type FileBrowserClipboardPane } from './file-browser-clipboard.controller';
import type { SourceCapabilities } from '../services/source-capabilities';

const mutable: SourceCapabilities = { read: true, create: true, delete: true, rename: true, trash: true };
const readOnly: SourceCapabilities = { read: true, create: false, delete: false, rename: false, trash: false };

function pane(capabilities: SourceCapabilities): FileBrowserClipboardPane {
  return {
    currentPath: 'filesystem://target',
    selectedFiles: new Set<string>(),
    sourceCapabilities: capabilities,
  };
}

function host(target: FileBrowserClipboardPane): FileBrowserClipboardHost {
  return {
    getActivePane: () => target,
    getActivePaneId: () => 'left',
    apiService: {
      batchCopy: fn().mockResolvedValue({ failed: 0 }),
      batchMove: fn().mockResolvedValue({ failed: 0 }),
    } as unknown as FileBrowserClipboardHost['apiService'],
    confirmDialog: { alert: fn() } as unknown as FileBrowserClipboardHost['confirmDialog'],
    buildDestinationPath: fn(async (source: string, destination: string) => `${destination}/${source.split('/').pop()}`),
    runCrossPaneCopy: fn(),
    runCrossPaneMove: fn(),
  };
}

describe('FileBrowserClipboardController paste capabilities', () => {
  it('pastes into a writable destination', async () => {
    const target = host(pane(mutable));
    const clipboard = { operation: 'copy' as const, paths: ['filesystem://source/file.txt'] };

    const result = await new FileBrowserClipboardController().pasteFromClipboard(target, clipboard);

    expect(target.apiService.batchCopy).toHaveBeenCalledTimes(1);
    expect(result).toBe(clipboard);
  });

  it('blocks paste into a read-only destination and preserves a cut clipboard', async () => {
    const target = host(pane(readOnly));
    const clipboard = { operation: 'cut' as const, paths: ['filesystem://source/file.txt'] };

    const result = await new FileBrowserClipboardController().pasteFromClipboard(target, clipboard);

    expect(target.apiService.batchMove).not.toHaveBeenCalled();
    expect(target.confirmDialog.alert).toHaveBeenCalledWith('Move Not Available\nThe destination is read-only.');
    expect(result).toBe(clipboard);
  });
});

