import { readFileSync } from 'node:fs';

const edit = readFileSync(new URL('./address-bar-edit.component.ts', import.meta.url), 'utf8');

describe('address bar Windows drive-root completion', () => {
  it('recognizes filesystem://c/ as a drive root', () => {
    expect(edit).toContain('/^filesystem:\\/\\/[a-z]\\/?$/i.test(raw)');
  });

  it('lists the current drive root directly for Ctrl+Space', () => {
    expect(edit).toContain("resolvedSource.source.id === 'filesystem'");
    expect(edit).toContain('this.isFilesystemDriveRoot(dirPath)');
    expect(edit).toContain('await resolvedSource.source.list(sourceContext, false)');
    expect(edit).toContain('onBatch?.(suggestions)');
  });

  it('keeps nested filesystem paths on progressive streaming', () => {
    expect(edit).toContain('startListDirectoryStream');
  });
});
