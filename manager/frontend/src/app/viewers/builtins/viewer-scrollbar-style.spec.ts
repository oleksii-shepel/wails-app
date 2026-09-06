import { readFileSync } from 'node:fs';

describe('built-in viewer scrollbar style', () => {
  const shared = readFileSync(new URL('./builtin-viewer.shared.ts', import.meta.url), 'utf8');
  const image = readFileSync(new URL('./image-viewer.component.ts', import.meta.url), 'utf8');

  it('does not hide scrollbars on viewer overflow regions', () => {
    expect(shared).not.toContain('scrollbar-width: none');
    expect(shared).not.toContain('.viewer-toolbar::-webkit-scrollbar');
    expect(image).not.toContain('scrollbar-width: none');
    expect(image).not.toContain('.carousel-list::-webkit-scrollbar');
  });
});
