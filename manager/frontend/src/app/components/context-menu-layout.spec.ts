import {
  contextMenuColumnCount,
  placeContextMenu,
} from './context-menu-layout';

describe('reassembly context menu layout', () => {
  it('keeps a menu below and right when it fits', () => {
    expect(placeContextMenu({
      anchorX: 100,
      anchorY: 100,
      menuWidth: 280,
      menuHeight: 300,
      viewportWidth: 1200,
      viewportHeight: 800,
    })).toEqual({ x: 104, y: 104 });
  });

  it('flips left and above near the bottom-right corner', () => {
    expect(placeContextMenu({
      anchorX: 1180,
      anchorY: 780,
      menuWidth: 280,
      menuHeight: 300,
      viewportWidth: 1200,
      viewportHeight: 800,
    })).toEqual({ x: 896, y: 476 });
  });

  it('clamps the complete menu inside the viewport', () => {
    expect(placeContextMenu({
      anchorX: 0,
      anchorY: 0,
      menuWidth: 280,
      menuHeight: 300,
      viewportWidth: 1200,
      viewportHeight: 800,
    })).toEqual({ x: 8, y: 8 });
  });

  it('adds columns instead of requiring vertical scrolling', () => {
    expect(contextMenuColumnCount(900, 600, 1200)).toBe(2);
    expect(contextMenuColumnCount(1500, 600, 1200)).toBe(3);
  });

  it('never requests more columns than fit horizontally', () => {
    expect(contextMenuColumnCount(2000, 500, 700)).toBe(2);
  });
});