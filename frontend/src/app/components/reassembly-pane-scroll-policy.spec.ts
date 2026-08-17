import { chooseScrollAlignment, normalizeScrollRatio } from './reassembly-pane-scroll-policy';

describe('Reassembly pane scroll policy', () => {
  it('prefers a matching resource identity', () => {
    expect(chooseScrollAlignment(
      { topName:'README.md', ratio:.8 },
      ['src','README.md','package.json'],
    )).toEqual({ mode:'name', name:'README.md' });
  });

  it('falls back to ratio when no peer exists', () => {
    expect(chooseScrollAlignment(
      { topName:'missing.txt', ratio:.62 },
      ['a.txt','b.txt'],
    )).toEqual({ mode:'ratio', ratio:.62 });
  });

  it('clamps bad ratios', () => {
    expect(normalizeScrollRatio(-1)).toBe(0);
    expect(normalizeScrollRatio(2)).toBe(1);
  });
});
