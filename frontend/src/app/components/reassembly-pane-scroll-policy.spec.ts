import { chooseScrollAlignment, normalizeScrollRatio } from './reassembly-pane-scroll-policy';

describe('Reassembly pane scroll policy', () => {
  it('prefers matching resource identity by name', () => {
    expect(chooseScrollAlignment(
      { topName: 'README.md', ratio: 0.8 },
      ['src', 'README.md', 'package.json'],
    )).toEqual({ mode:'name', name:'README.md' });
  });

  it('falls back to proportional scrolling when no peer exists', () => {
    expect(chooseScrollAlignment(
      { topName: 'missing.txt', ratio: 0.62 },
      ['a.txt','b.txt'],
    )).toEqual({ mode:'ratio', ratio:0.62 });
  });

  it('clamps invalid ratios', () => {
    expect(normalizeScrollRatio(-1)).toBe(0);
    expect(normalizeScrollRatio(2)).toBe(1);
    expect(normalizeScrollRatio(Number.NaN)).toBe(0);
  });
});
