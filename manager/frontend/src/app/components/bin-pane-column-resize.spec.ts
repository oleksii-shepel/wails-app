import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin Details column resizing', () => {
  it('uses the shared pane Details resize directive', () => {
    expect(source).toContain("PaneDetailsResizeHandleDirective");
    expect(source).toContain('data-pane-details-root');
    expect(source).toContain("[paneDetailsResizeHandle]=\"'bin-name'\"");
    expect(source).toContain("[paneDetailsResizeHandle]=\"'bin-location'\"");
    expect(source).toContain("[paneDetailsResizeHandle]=\"'bin-deleted'\"");
    expect(source).toContain("[paneDetailsResizeHandle]=\"'bin-size'\"");
    expect(source).toContain("[paneDetailsResizeHandle]=\"'bin-revision'\"");
  });

  it('keeps grouped and flat on one six-track width contract', () => {
    expect(source).toContain('var(--details-column-bin-name-width');
    expect(source).toContain('var(--details-column-bin-location-width');
    expect(source).toContain('var(--details-column-bin-deleted-width');
    expect(source).toContain('var(--details-column-bin-size-width');
    expect(source).toContain('var(--details-column-bin-revision-width');
    expect(source).toContain('<span>Location<i [paneDetailsResizeHandle]="\'bin-location\'"');
    expect(source).toContain('<span>Path<i [paneDetailsResizeHandle]="\'bin-location\'"');
  });

  it('does not introduce a Bin-specific resize implementation', () => {
    expect(source).not.toContain('onColumnResizeStart(');
    expect(source).not.toContain("window.addEventListener('mousemove'");
    expect(source).not.toContain("window.addEventListener('mouseup'");
  });
});
