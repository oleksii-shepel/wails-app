import { readFileSync } from 'node:fs';

describe('TextViewer custom scrollbar removal', () => {
  const source = readFileSync(new URL('./text-viewer.component.ts', import.meta.url), 'utf8');

  it('contains no dangling custom scrollbar state or assignments', () => {
    expect(source).not.toContain('editorScrollbar');
    expect(source).not.toContain('VscodeScrollbarController');
    expect(source).not.toContain('VscodeScrollbarDirective');
    expect(source).not.toContain('appVscodeScrollbar');
  });
});
