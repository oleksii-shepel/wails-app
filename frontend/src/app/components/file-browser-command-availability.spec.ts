import type { ActionAvailability } from '../services/action-availability';
import { BC } from './file-browser-command-registry';
import { isFileBrowserCommandAvailable } from './file-browser-command-availability';

const availability: ActionAvailability = {
  createFolder: false,
  deleteSelected: false,
  copySelected: true,
  cutSelected: false,
  paste: false,
  rename: false,
  copyToOther: true,
  moveToOther: false,
};

describe('isFileBrowserCommandAvailable', () => {
  it('maps capability-sensitive browser commands', () => {
    expect(isFileBrowserCommandAvailable(BC.copy, availability)).toBe(true);
    expect(isFileBrowserCommandAvailable(BC.cut, availability)).toBe(false);
    expect(isFileBrowserCommandAvailable(BC.newFolder, availability)).toBe(false);
    expect(isFileBrowserCommandAvailable(BC.copyToOtherPane, availability)).toBe(true);
    expect(isFileBrowserCommandAvailable(BC.moveToOtherPane, availability)).toBe(false);
  });

  it('leaves unrelated commands available', () => {
    expect(isFileBrowserCommandAvailable(BC.refresh, availability)).toBe(true);
    expect(isFileBrowserCommandAvailable('plugin.example.command', availability)).toBe(true);
  });
});

