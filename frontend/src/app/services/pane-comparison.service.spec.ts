import { FileInfo, FileType } from '../shared/protocol';
import { PaneComparisonService } from './pane-comparison.service';

function file(name: string, side: 'left' | 'right', overrides: Partial<FileInfo> = {}): FileInfo {
  return {
    name,
    path: `/${side}/${name}`,
    type: FileType.FILE,
    size: 10,
    created: 1,
    modified: 100,
    accessed: 1,
    permissions: '',
    isHidden: false,
    ...overrides,
  };
}

describe('PaneComparisonService', () => {
  const service = new PaneComparisonService();

  it('classifies missing, newer and size-different files by name', () => {
    const result = service.compare(
      [file('same.txt', 'left'), file('left.txt', 'left'), file('newer.txt', 'left', { modified: 200 }), file('size.txt', 'left', { size: 20 })],
      [file('same.txt', 'right'), file('right.txt', 'right'), file('newer.txt', 'right', { modified: 100 }), file('size.txt', 'right', { size: 10 })],
    );

    expect(result.summary).toMatchObject({ same: 1, onlyLeft: 1, onlyRight: 1, leftNewer: 1, sizeDifferent: 1, different: 4 });
  });

  it('treats timestamp differences within the tolerance as equal', () => {
    const result = service.compare([file('a.txt', 'left', { modified: 100 })], [file('a.txt', 'right', { modified: 101 })]);
    expect(result.items[0].status).toBe('same');
  });

  it('does not recursively compare directories', () => {
    const result = service.compare(
      [file('folder', 'left', { type: FileType.DIRECTORY, modified: 200 })],
      [file('folder', 'right', { type: FileType.DIRECTORY, modified: 100 })],
    );
    expect(result.items[0].status).toBe('same');
  });

  it('returns safe missing-only copy sets', () => {
    const result = service.compare([file('left.txt', 'left')], [file('right.txt', 'right')]);
    expect(service.missingOnRightPaths(result)).toEqual(['/left/left.txt']);
    expect(service.missingOnLeftPaths(result)).toEqual(['/right/right.txt']);
  });

  it('supports explicit size-only and date-only policies', () => {
    const sizeOnly = service.compare(
      [file('a.txt', 'left', { size: 10, modified: 200 })],
      [file('a.txt', 'right', { size: 10, modified: 100 })],
      { policy: 'size' },
    );
    expect(sizeOnly.items[0].status).toBe('same');

    const dateOnly = service.compare(
      [file('a.txt', 'left', { size: 20, modified: 100 })],
      [file('a.txt', 'right', { size: 10, modified: 100 })],
      { policy: 'date' },
    );
    expect(dateOnly.items[0].status).toBe('same');
  });

  it('verifies equal-size files by SHA-256 when hash policy is selected', async () => {
    const base = service.compare([file('a.txt', 'left')], [file('a.txt', 'right')], { policy: 'hash' });
    const api = {
      hashFile: async (path: string) => path.includes('/left/') ? 'aaa' : 'bbb',
      readFileRange: async () => ({ path: '', content: '', encoding: 'base64', size: 0 }),
    };
    const verified = await service.verify(api as any, base, 'hash');
    expect(verified.items[0].status).toBe('hash-different');
    expect(verified.summary.hashDifferent).toBe(1);
  });

  it('compares content in bounded ranges instead of loading complete files', async () => {
    const size = 300 * 1024;
    const base = service.compare(
      [file('a.bin', 'left', { size })],
      [file('a.bin', 'right', { size })],
      { policy: 'content' },
    );
    const calls: Array<[string, number, number]> = [];
    const api = {
      hashFile: async () => '',
      readFileRange: async (path: string, offset: number, length: number) => {
        calls.push([path, offset, length]);
        return { path, content: offset === 0 ? 'same' : (path.includes('/left/') ? 'left' : 'right'), encoding: 'base64', size };
      },
    };
    const verified = await service.verify(api as any, base, 'content');
    expect(verified.items[0].status).toBe('content-different');
    expect(calls.some(([, offset]) => offset > 0)).toBe(true);
  });

});