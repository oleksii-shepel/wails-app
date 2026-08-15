import { describe, expect, it } from 'vitest';
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
});
