import { beforeEach, describe, expect, it, vi } from 'vitest';
import { PinnedLocationService } from './pinned-location.service';
import { RelatedLocationService } from './related-location.service';

function pin(service: PinnedLocationService, location: string, surfaceId?: string) {
  const value = service.create(location, undefined, undefined, Date.now(), surfaceId);
  if (!value) throw new Error('pin creation failed');
  return value;
}

describe('RelatedLocationService', () => {
  let pins: PinnedLocationService;
  let related: RelatedLocationService;

  beforeEach(() => {
    pins = new PinnedLocationService();
    related = new RelatedLocationService(pins);
  });

  it('ranks saved searches by exact, child, then ancestor context', () => {
    const exact = pin(pins, 'search://?path=C%3A%2Fwork%2Fapp&pattern=TODO');
    const child = pin(pins, 'search://?path=C%3A%2Fwork%2Fapp%2Fsrc&pattern=spec');
    const ancestor = pin(pins, 'search://?path=C%3A%2Fwork&pattern=FIXME');
    const unrelated = pin(pins, 'search://?path=D%3A%2Fother&pattern=x');

    expect(related.resolve([ancestor, unrelated, child, exact], 'C:/work/app').map(x => x.id))
      .toEqual([exact.id, child.id, ancestor.id]);
  });

  it('does not include the current pinned surface in Related', () => {
    const current = pin(pins, 'search://?path=C%3A%2Fwork&pattern=TODO', 'search-results');
    expect(related.resolve([current], current.location, 'search-results')).toEqual([]);
  });

  it('lets extensions contribute their own relationship score', () => {
    const git = pin(pins, 'surface://git/repo?root=C%3A%2Fwork', 'git');
    related.register({
      id: 'test.git-related',
      score: (candidate, context) => candidate.surfaceId === 'git' && context.contextLocation === 'C:/work' ? 950 : 0,
    });

    expect(related.resolve([git], 'C:/work')).toEqual([git]);
  });

  it('isolates a failing relationship provider', () => {
    const search = pin(pins, 'search://?path=C%3A%2Fwork&pattern=TODO');
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    related.register({ id: 'broken', score: () => { throw new Error('boom'); } });

    expect(related.resolve([search], 'C:/work')).toEqual([search]);
    expect(spy).toHaveBeenCalled();
    spy.mockRestore();
  });
});
