import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./directory-list-surface.component.ts', import.meta.url),
  'utf8',
);

describe('Directory list surface scroll region', () => {
  it('keeps the details header outside the row viewport', () => {
    const headerIndex = source.indexOf('class="details-header-viewport"');
    const viewportIndex = source.indexOf('class="list-viewport"');

    expect(headerIndex).toBeGreaterThanOrEqual(0);
    expect(viewportIndex).toBeGreaterThan(headerIndex);
    expect(source).toContain('class="list"');
    expect(source).toContain('data-pane-details-root');
  });

  it('keeps pane-content, scrolling, and background gestures on the row viewport', () => {
    expect(source).toContain('#listViewport');
    expect(source).toContain('class="list-viewport"');
    expect(source).toContain('data-pane-content');
    expect(source).toContain('(scroll)="onViewportScroll($event)"');
    expect(source).toContain('.list-viewport {\n      flex: 1 1 auto;\n      min-height: 0;\n      overflow: auto;');
  });

  it('syncs the fixed details header with horizontal body scrolling', () => {
    expect(source).toContain('#detailsHeader');
    expect(source).toContain('private detailsHeaderRef?: ElementRef<HTMLElement>');
    expect(source).toContain('header.style.transform = viewport.scrollLeft > 0');
  });

  it('reserves the viewport scrollbar gutter in the details header', () => {
    expect(source).toContain('padding-right: calc(10px + var(--details-scrollbar-gutter, 0px));');
    expect(source).toContain('const scrollbarGutter = Math.max(0, viewport.offsetWidth - viewport.clientWidth);');
    expect(source).toContain("header.style.setProperty('--details-scrollbar-gutter', `${scrollbarGutter}px`);");
  });
});
