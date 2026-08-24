import { readFileSync } from 'node:fs';
import { join } from 'node:path';

describe('Pane Details visible resize delimiter', () => {
  const source = readFileSync(
    join(__dirname, 'pane-details-resize-handle.directive.ts'),
    'utf8',
  );

  it('renders a persistent one-pixel delimiter inside the shared eight-pixel drag target', () => {
    expect(source).toContain("@HostBinding('style.width') readonly width = '8px'");
    expect(source).toContain("linear-gradient(to right");
    expect(source).toContain("calc(50% - 0.5px)");
    expect(source).toContain("calc(50% + 0.5px)");
  });

  it('strengthens the shared delimiter on hover and while dragging', () => {
    expect(source).toContain("@HostListener('mouseenter')");
    expect(source).toContain("@HostListener('mouseleave')");
    expect(source).toContain("this.delimiterOpacity = '1'");
  });
});
