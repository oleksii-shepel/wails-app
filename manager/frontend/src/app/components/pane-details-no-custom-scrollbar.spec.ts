import { readFileSync } from 'node:fs';

describe('Details scrolling ownership', () => {
  const source = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');

  it('does not mount the custom VS Code scrollbar directive', () => {
    expect(source).not.toContain('VscodeScrollbarDirective');
    expect(source).not.toContain('appVscodeScrollbar');
  });

  it('keeps Details body as the native scroll container', () => {
    expect(source).toContain('.details-body');
    expect(source).toContain('overflow-y: auto;');
  });
});
