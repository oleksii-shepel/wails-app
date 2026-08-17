import { deletionRecoveryPath, isSameOrDescendantPath, remapDescendantPath } from './resource-mutation.service';

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
