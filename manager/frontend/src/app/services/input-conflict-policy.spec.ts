import { compareStableBindingIdentity, whenClausesCanOverlap } from './input-conflict-policy';

describe('input conflict policy', () => {
  it('recognizes complementary conditions as mutually exclusive', () => {
    expect(whenClausesCanOverlap('viewerSurface', '!viewerSurface')).toBeFalse();
    expect(whenClausesCanOverlap('!typing && !viewerSurface', 'viewerSurface')).toBeFalse();
  });

  it('keeps unrelated conditions conservatively overlapping', () => {
    expect(whenClausesCanOverlap('!typing', 'viewerSurface')).toBeTrue();
    expect(whenClausesCanOverlap(undefined, '!typing')).toBeTrue();
  });

  it('orders exact ties by stable identity', () => {
    expect(compareStableBindingIdentity(
      { id: 'a.binding', commandId: 'z.command' },
      { id: 'b.binding', commandId: 'a.command' },
    )).toBeLessThan(0);
  });
});