import { FileType, type FileInfo } from '../shared/protocol';
import { ThumbnailPipelineService } from './thumbnail-pipeline.service';
import { WorkSchedulerService } from './work-scheduler.service';

function image(path: string): FileInfo {
  return {
    name: path.split('/').pop() ?? path,
    path,
    type: FileType.FILE,
    size: 4096,
    modified: 1,
    created: 1,
    accessed: 1,
    permissions: '',
    isHidden: false,
    isSymlink: false,
  };
}

describe('ThumbnailPipelineService', () => {
  function service(): ThumbnailPipelineService {
    return new ThumbnailPipelineService(
      {} as any,
      new WorkSchedulerService(),
    );
  }

  it('uses central scheduler generations per grid scope', () => {
    const value = service();
    const first = value.beginGeneration('left-grid');
    expect(value.isCurrent('left-grid', first)).toBeTrue();

    const second = value.beginGeneration('left-grid');
    expect(second).toBe(first + 1);
    expect(value.isCurrent('left-grid', first)).toBeFalse();
    expect(value.isCurrent('left-grid', second)).toBeTrue();
  });

  it('only considers thumbnail-capable files', () => {
    const value = service();
    expect(value.canThumbnail(image('/photo.jpg'))).toBeTrue();
    expect(value.canThumbnail(image('/movie.mp4'))).toBeTrue();
    expect(value.canThumbnail(image('/notes.txt'))).toBeFalse();
  });

  it('keeps decoded-image cache bounded independently of work concurrency', () => {
    const value = service();
    expect((value as any).maxDecodedEntries).toBe(192);
    expect((value as any).maxDecodedBytes).toBe(96 * 1024 * 1024);
    expect((value as any).maxConcurrency).toBeUndefined();
  });
});
