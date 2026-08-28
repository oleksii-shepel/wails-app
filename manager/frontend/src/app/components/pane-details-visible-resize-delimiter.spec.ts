import { readFileSync } from 'node:fs';
import { join } from 'node:path';

describe('Pane Details visible resize delimiter', () => {
  const directive = readFileSync(
    join(__dirname, 'pane-details-resize-handle.directive.ts'),
    'utf8',
  );
  const component = readFileSync(
    join(__dirname, 'pane-details.component.ts'),
    'utf8',
  );

  it('renders a persistent one-pixel delimiter inside the shared eight-pixel drag target', () => {
    expect(directive).toContain("@HostBinding('style.width') readonly width = '8px'");
    expect(directive).toContain("linear-gradient(to right");
    expect(directive).toContain("calc(50% - 0.5px)");
    expect(directive).toContain("calc(50% + 0.5px)");
  });

  it('resizes the real column tracks live through CSS variables while dragging', () => {
    expect(directive).toContain("this.applyWidth(this.currentWidth);");
    expect(component).toContain("return `${variable}, ${Math.max(1, configured)}px)`;");
    expect(component).toContain("return `${variable}, ${Math.max(1, fallback)}px)`;");
    expect(component).toContain("return `${variable}, minmax(${minWidth}px, ${fr}fr))`;");
  });
});
