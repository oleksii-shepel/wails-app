import { FileType, type FileInfo } from '../shared/protocol';
import { PaneComparisonService } from './pane-comparison.service';

function file(name: string, path: string, size = 1, modified = 1): FileInfo {
  return {
    name,
    path,
    type: FileType.FILE,
    size,
    modified,
    created: modified,
    accessed: modified,
    permissions: '',
    isHidden: false,
    isSymlink: false,
  };
}

describe('PaneComparisonService status maps', () => {
  it('indexes comparison status by the concrete path on each side', () => {
    const service = new PaneComparisonService();
    const result = service.compare(
      [
        file('same.txt', 'L/same.txt', 1, 1),
        file('left.txt', 'L/left.txt'),
        file('newer.txt', 'L/newer.txt', 1, 20),
      ],
      [
        file('same.txt', 'R/same.txt', 1, 1),
        file('right.txt', 'R/right.txt'),
        file('newer.txt', 'R/newer.txt', 1, 1),
      ],
    );

    const maps = service.statusMaps(result);
    expect(maps.left.get('L/left.txt')).toBe('only-left');
    expect(maps.right.get('R/right.txt')).toBe('only-right');
    expect(maps.left.get('L/newer.txt')).toBe('left-newer');
    expect(maps.right.get('R/newer.txt')).toBe('left-newer');
  });
});