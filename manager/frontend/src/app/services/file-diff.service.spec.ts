import type { ApiService } from './api.service';
import { FileDiffService } from './file-diff.service';

describe('FileDiffService', () => {
  const makeApiStub = (overrides: Partial<ApiService> = {}): ApiService =>
    ({
      getFileInfo: jasmine.createSpy('getFileInfo').and.resolveTo({
        name: 'a.txt',
        path: '/tmp/a.txt',
        size: 12,
      }),
      probeFile: jasmine.createSpy('probeFile').and.resolveTo({
        mimeType: 'text/plain',
        isText: true,
        isImage: false,
        isBinary: false,
      }),
      readFile: jasmine.createSpy('readFile').and.resolveTo({
        path: '/tmp/a.txt',
        content: 'hello\nworld\n',
        encoding: 'utf8',
        size: 12,
        truncated: false,
      }),
      ...overrides,
    }) as unknown as ApiService;

  it('detects identical files cleanly', () => {
    const svc = new FileDiffService(makeApiStub());
    const diff = svc.computeLineDiff('alpha\nbeta\ngamma\n', 'alpha\nbeta\ngamma\n');
    expect(diff.isIdentical).toBeTrue();
    expect(diff.hunks.length).toBe(0);
    expect(diff.stats.unchanged).toBe(3);
  });

  it('computes modified, added, and removed rows and hunk metadata', () => {
    const svc = new FileDiffService(makeApiStub());
    const left = 'line1\nline2\nline3\nline4\n';
    const right = 'line1\nline2-edited\nline3\nline4\nline5\n';
    const diff = svc.computeLineDiff(left, right);

    expect(diff.isIdentical).toBeFalse();
    expect(diff.stats.modified).toBe(1);
    expect(diff.stats.added).toBe(1);
    expect(diff.stats.removed).toBe(0);
    expect(diff.hunks.length).toBe(2);
    expect(diff.rows.find(r => r.kind === 'modify')?.leftText).toBe('line2');
    expect(diff.rows.find(r => r.kind === 'modify')?.rightText).toBe('line2-edited');
    expect(diff.rows.find(r => r.kind === 'add')?.rightText).toBe('line5');
  });

  it('honors ignoreWhitespace and ignoreCase options', () => {
    const svc = new FileDiffService(makeApiStub());
    const diff = svc.computeLineDiff('  Hello   World\n', 'hello world\n', {
      ignoreWhitespace: true,
      ignoreCase: true,
    });
    expect(diff.isIdentical).toBeTrue();
  });

  it('formats unified patches with context headers', () => {
    const svc = new FileDiffService(makeApiStub());
    const diff = svc.computeLineDiff('a\nb\nc\nd\n', 'a\nB\nc\nd\n');
    const patch = svc.formatUnifiedPatch('/a.txt', '/b.txt', diff, 1);

    expect(patch).toContain('--- /a.txt');
    expect(patch).toContain('+++ /b.txt');
    expect(patch).toContain('@@ -1,3 +1,3 @@');
    expect(patch).toContain('-b');
    expect(patch).toContain('+B');
  });

  it('rejects files larger than 2 MB or containing NUL bytes', async () => {
    const largeSvc = new FileDiffService(
      makeApiStub({
        getFileInfo: jasmine.createSpy('getFileInfo').and.resolveTo({
          name: 'big.log',
          path: '/tmp/big.log',
          size: 3 * 1024 * 1024,
        }),
      }),
    );
    const largeResult = await largeSvc.loadFileForDiff('/tmp/big.log');
    expect(largeResult.tooLarge).toBeTrue();
    expect(largeResult.error).toContain('exceeds');

    const nulSvc = new FileDiffService(
      makeApiStub({
        readFile: jasmine.createSpy('readFile').and.resolveTo({
          path: '/tmp/bin.dat',
          content: 'abc\u0000def',
          encoding: 'utf8',
          size: 7,
          truncated: false,
        }),
      }),
    );
    const nulResult = await nulSvc.loadFileForDiff('/tmp/bin.dat');
    expect(nulResult.isBinary).toBeTrue();
    expect(nulResult.error).toContain('Binary');
  });
});
