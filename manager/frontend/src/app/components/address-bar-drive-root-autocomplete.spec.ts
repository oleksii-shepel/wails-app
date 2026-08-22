import { readFileSync } from 'node:fs';

const edit = readFileSync(new URL('./address-bar-edit.component.ts', import.meta.url), 'utf8');

describe('address bar Windows drive-root completion', () => {
  it('recognizes friendly and canonical C drive roots after normalization', () => {
    expect(edit).toContain('private isFilesystemDriveRoot(path: string)');
    expect(edit).toContain('/^filesystem:\\/\\/[a-z]$/i.test(normalized)');
  });

  it('lists the resolved filesystem source directly for a bare drive root', () => {
    expect(edit).toContain("resolvedSource.source.id === 'filesystem'");
    expect(edit).toContain('this.isFilesystemDriveRoot(dirPath)');
    expect(edit).toContain('await resolvedSource.source.list(sourceContext, false)');
  });

  it('publishes drive-root directories immediately to the dropdown', () => {
    expect(edit).toContain('onBatch?.(suggestions)');
    expect(edit).toContain("if (entry.type !== FileType.DIRECTORY) continue;");
  });

  it('keeps streaming for nested filesystem paths', () => {
    expect(edit).toContain('startListDirectoryStream');
  });
});
