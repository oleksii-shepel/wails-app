import { readFileSync } from 'node:fs';

const component = readFileSync(
  new URL('./perf-overlay.component.ts', import.meta.url),
  'utf8',
);

describe('Performance overlay draggable contract', () => {
  it('allows pointer interaction and starts dragging from the overlay', () => {
    expect(component).toContain('pointer-events: auto;');
    expect(component).toContain('(pointerdown)="startDrag($event)"');
    expect(component).toContain('element.setPointerCapture(event.pointerId);');
    expect(component).toContain('event.stopPropagation();');
  });

  it('tracks document pointer events while keeping the overlay in the viewport', () => {
    expect(component).toContain("@HostListener('document:pointermove', ['$event'])");
    expect(component).toContain("@HostListener('document:pointerup', ['$event'])");
    expect(component).toContain("@HostListener('document:pointercancel', ['$event'])");
    expect(component).toContain('window.innerWidth - this.dragWidth - this.viewportMarginPx');
    expect(component).toContain('window.innerHeight - this.dragHeight - this.viewportMarginPx');
  });

  it('lets component state own top and bottom positioning so drag does not stretch the panel', () => {
    expect(component).toContain('[style.top.px]="positionTop"');
    expect(component).toContain('[style.bottom.px]="positionTop === null ? 8 : null"');
    expect(component).not.toContain('bottom: 8px;');
  });

  it('can reset to the default bottom-left position', () => {
    expect(component).toContain('(dblclick)="resetPosition($event)"');
    expect(component).toContain('this.positionLeft = 8;');
    expect(component).toContain('this.positionTop = null;');
  });
});
