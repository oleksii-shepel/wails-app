import { deletionRecoveryPath, isSameOrDescendantPath, remapDescendantPath, ResourceMutationService } from './resource-mutation.service';

describe('resource mutation path relationships', () => {
  it('recognizes same and nested locations across slash forms', () => {
    expect(isSameOrDescendantPath('C:\\Work\\Project\\src', 'filesystem://c/Work/Project')).toBe(true);
    expect(isSameOrDescendantPath('filesystem://c/Other', 'filesystem://c/Work')).toBe(false);
  });

  it('remaps a nested open location when its ancestor moves', () => {
    expect(remapDescendantPath(
      'filesystem://c/Work/Project/src',
      'filesystem://c/Work/Project',
      'filesystem://c/Archive/Project',
    )).toBe('filesystem://c/Archive/Project/src');
  });

  it('recovers a nested location to the nearest surviving parent after deletion', () => {
    expect(deletionRecoveryPath(
      'filesystem://c/Work/Project/src',
      ['filesystem://c/Work/Project'],
    )).toBe('filesystem://c/Work');
  });
});

describe('ResourceMutationService coalescing', () => {
  it('coalesces a burst of identical bin invalidations into one listener delivery', async () => {
    const service = new ResourceMutationService();
    const received: any[] = [];
    const release = service.subscribe(mutation => received.push(mutation));

    for (let index = 0; index < 25; index += 1) {
      service.publish({
        kind: 'modify',
        items: [{ source: 'bin://' }],
        origin: 'recycle-bin',
      });
    }

    await new Promise(resolve => setTimeout(resolve, 120));
    release();

    expect(received.length).toBe(1);
    expect(received[0].kind).toBe('modify');
    expect(received[0].items.length).toBe(1);
    expect(received[0].items[0].source).toBe('bin://');
  });

  it('deduplicates repeated source/destination pairs inside one coalesced mutation', async () => {
    const service = new ResourceMutationService();
    const received: any[] = [];
    const release = service.subscribe(mutation => received.push(mutation));

    service.publish({ kind: 'modify', items: [{ source: 'bin://' }, { source: 'bin://' }] });
    service.publish({ kind: 'modify', items: [{ source: 'bin://' }] });

    await new Promise(resolve => setTimeout(resolve, 120));
    release();

    expect(received.length).toBe(1);
    expect(received[0].items.length).toBe(1);
  });
});
