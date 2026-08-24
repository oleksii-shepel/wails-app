import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('pane Details column resize architecture', () => {
  const resize = read('pane-details-resize-handle.directive.ts');
  const details = read('pane-details.component.ts');
  const directory = read('directory-pane.component.ts');
  const registry = read('registry-pane.component.ts');

  it('has exactly one mouse drag implementation for Details column resizing', () => {
    expect(resize).toContain("window.addEventListener('mousemove'");
    expect(resize).toContain("window.addEventListener('mouseup'");
    expect(resize).toContain('--details-column-');

    for (const consumer of [details, directory, registry]) {
      expect(consumer).not.toContain('onColumnResizeStart');
      expect(consumer).not.toContain('onColumnResizeMove');
      expect(consumer).not.toContain('cleanupColumnResizeListeners');
    }
  });

  it('routes all current Details surfaces through the shared resize handle', () => {
    expect(details).toContain('PaneDetailsResizeHandleDirective');
    expect(details).toContain('[paneDetailsResizeHandle]="col.key"');
    expect(directory).toContain('PaneDetailsResizeHandleDirective');
    expect(directory).toContain("[paneDetailsResizeHandle]=\"'name'\"");
    expect(directory).toContain("[paneDetailsResizeHandle]=\"'bin-name'\"");
    expect(registry).toContain('PaneDetailsResizeHandleDirective');
    expect(registry).toContain("[paneDetailsResizeHandle]=\"'value'\"");
  });

  it('uses the same inherited width variables for header and rows', () => {
    expect(details).toContain('--details-column-');
    expect(directory).toContain('var(--details-column-name-width');
    expect(registry).toContain('var(--details-column-name-width');
  });
});
