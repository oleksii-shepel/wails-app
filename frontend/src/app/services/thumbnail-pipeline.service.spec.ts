import { FileType, type FileInfo } from '../shared/protocol';
import { ThumbnailPipelineService } from './thumbnail-pipeline.service';

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
  it('uses generation cancellation per grid scope', () => {
    const service = new ThumbnailPipelineService({} as any);
    const first = service.beginGeneration('left-grid');
    expect(service.isCurrent('left-grid', first)).toBeTrue();

    const second = service.beginGeneration('left-grid');
    expect(second).toBe(first + 1);
    expect(service.isCurrent('left-grid', first)).toBeFalse();
    expect(service.isCurrent('left-grid', second)).toBeTrue();
  });

  it('only considers thumbnail-capable files', () => {
    const service = new ThumbnailPipelineService({} as any);
    expect(service.canThumbnail(image('/photo.jpg'))).toBeTrue();
    expect(service.canThumbnail(image('/movie.mp4'))).toBeTrue();
    expect(service.canThumbnail(image('/notes.txt'))).toBeFalse();
    expect(service.canThumbnail({
      ...image('/folder'),
      type: FileType.DIRECTORY,
    })).toBeFalse();
  });

  it('has bounded decoded cache limits in the implementation', () => {
    const service = new ThumbnailPipelineService({} as any);
    expect((service as any).maxDecodedEntries).toBe(192);
    expect((service as any).maxDecodedBytes).toBe(96 * 1024 * 1024);
  });
});
