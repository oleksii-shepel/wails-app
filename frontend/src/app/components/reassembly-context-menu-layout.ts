export interface ContextMenuPlacementInput {
  anchorX: number;
  anchorY: number;
  menuWidth: number;
  menuHeight: number;
  viewportWidth: number;
  viewportHeight: number;
  margin?: number;
  gap?: number;
}

export interface ContextMenuPoint {
  x: number;
  y: number;
}

export function contextMenuColumnCount(
  naturalHeight: number,
  availableHeight: number,
  viewportWidth: number,
  preferredColumnWidth = 286,
): number {
  const required = Math.max(1, Math.ceil(naturalHeight / Math.max(1, availableHeight)));
  const horizontal = Math.max(1, Math.floor(viewportWidth / preferredColumnWidth));
  return Math.min(required, horizontal);
}

export function placeContextMenu(input: ContextMenuPlacementInput): ContextMenuPoint {
  const margin = input.margin ?? 8;
  const gap = input.gap ?? 4;

  let x = input.anchorX + gap;
  let y = input.anchorY + gap;

  if (x + input.menuWidth > input.viewportWidth - margin) {
    x = input.anchorX - input.menuWidth - gap;
  }
  if (y + input.menuHeight > input.viewportHeight - margin) {
    y = input.anchorY - input.menuHeight - gap;
  }

  return {
    x: Math.max(margin, Math.min(x, input.viewportWidth - input.menuWidth - margin)),
    y: Math.max(margin, Math.min(y, input.viewportHeight - input.menuHeight - margin)),
  };
}
