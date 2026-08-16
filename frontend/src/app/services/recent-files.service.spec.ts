import { RecentFilesService } from './recent-files.service';
import { FileType } from '../shared/protocol';

function entry(path: string) {
  const name = path.split('/').pop() || path;
  return { name, path, type: FileType.FILE, size: 1, modified: 1, created: 1, accessed: 1, permissions: '', isHidden: false, isSymlink: false };
}

describe('RecentFilesService', () => {
  beforeEach(() => localStorage.clear());

  it('keeps most recently opened files first and de-duplicates by path', () => {
    const service = new RecentFilesService();
    service.record(entry('/a.txt'));
    service.record(entry('/b.txt'));
    service.record(entry('/a.txt'));

    expect(service.recent().map(file => file.path)).toEqual(['/a.txt', '/b.txt']);
  });
});
