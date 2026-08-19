import { FileType } from '../shared/protocol';
import {
  isInlineRenameNoop,
  renameSelectionEnd,
} from './rename-policy';

describe('reassembly inline rename policy', () => {
  it('selects a file basename but preserves its extension for quick typing', () => {
    expect(renameSelectionEnd('report.final.pdf', FileType.FILE))
      .toBe('report.final'.length);
  });

  it('selects the complete directory name', () => {
    expect(renameSelectionEnd('Project.v2', FileType.DIRECTORY))
      .toBe('Project.v2'.length);
  });

  it('selects complete names for extensionless and dot-prefixed files', () => {
    expect(renameSelectionEnd('README', FileType.FILE)).toBe(6);
    expect(renameSelectionEnd('.gitignore', FileType.FILE)).toBe(10);
  });

  it('treats a trimmed unchanged name as a no-op', () => {
    expect(isInlineRenameNoop('notes.txt', ' notes.txt ')).toBeTrue();
  });
});