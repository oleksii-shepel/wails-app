import { FileType, type FileInfo } from '../shared/protocol';
import { formatBytes, selectionSizeSuffix } from './selection-summary';

function entry(path: string, size: number, type: FileType = FileType.FILE): FileInfo {
  return {
    name: path.split('/').pop() || path,
    path,
    type,
    size,
    created: 0,
    modified: 0,
    accessed: 0,
    permissions: '',
    isHidden: false,
    isSymlink: false,
  };
}

describe('selection size summary', () => {
  it('returns empty for an empty selection', () => {
    expect(selectionSizeSuffix(new Set(), [entry('/a.txt', 10)])).toBe('');
  });

  it('sums selected file sizes and formats them', () => {
    const entries = [
      entry('/a.txt', 1024),
      entry('/b.bin', 512),
      entry('/c.txt', 2048),
    ];
    const suffix = selectionSizeSuffix(new Set(['/a.txt', '/c.txt']), entries);
    expect(suffix).toBe(' · 3.0 KB');
  });

  it('ignores unselected entries', () => {
    const suffix = selectionSizeSuffix(new Set(['/b.bin']), [entry('/a.txt', 1024), entry('/b.bin', 512)]);
    expect(suffix).toBe(' · 512 B');
  });

  it('returns empty when a directory or symlink is selected', () => {
    const entries = [entry('/a.txt', 10), entry('/docs', 0, FileType.DIRECTORY), entry('/l.txt', 5, FileType.SYMLINK)];
    expect(selectionSizeSuffix(new Set(['/a.txt', '/docs']), entries)).toBe('');
    expect(selectionSizeSuffix(new Set(['/l.txt']), entries)).toBe('');
  });

  it('formats bytes across unit boundaries', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(999)).toBe('999 B');
    expect(formatBytes(1024)).toBe('1.0 KB');
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(1024 * 1024 * 5)).toBe('5.0 MB');
  });
});
