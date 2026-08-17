export interface ReconciledSelection {
  selected: Set<string>;
  cursor: string | null;
  anchor: string | null;
}

export interface ReconcileVisibleSelectionInput {
  selected: ReadonlySet<string>;
  cursor: string | null;
  anchor: string | null;
  visiblePaths: readonly string[];
}

export function togglePathSelection(
  selected: ReadonlySet<string>,
  path: string,
  anchor: string | null,
): { selected: Set<string>; anchor: string } {
  const next = new Set(selected);
  if (next.has(path)) next.delete(path);
  else next.add(path);

  return {
    selected: next,
    anchor: anchor ?? path,
  };
}

export function selectPathRange(
  visiblePaths: readonly string[],
  anchor: string,
  target: string,
): Set<string> | null {
  const start = visiblePaths.indexOf(anchor);
  const end = visiblePaths.indexOf(target);
  if (start < 0 || end < 0) return null;

  const from = Math.min(start, end);
  const to = Math.max(start, end);
  return new Set(visiblePaths.slice(from, to + 1));
}

export function reconcileVisibleSelection(
  input: ReconcileVisibleSelectionInput,
): ReconciledSelection {
  const visible = input.visiblePaths;
  const visibleSet = new Set(visible);
  const selected = new Set(
    Array.from(input.selected).filter(path => visibleSet.has(path)),
  );

  if (visible.length === 0) {
    return { selected, cursor: null, anchor: null };
  }

  const cursor =
    input.cursor && visibleSet.has(input.cursor)
      ? input.cursor
      : Array.from(selected)[0] ?? visible[0];

  const anchor =
    input.anchor && visibleSet.has(input.anchor)
      ? input.anchor
      : cursor;

  return { selected, cursor, anchor };
}
