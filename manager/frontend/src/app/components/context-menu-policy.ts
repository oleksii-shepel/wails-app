import type { ContextMenuItem } from './context-menu.component';

/**
 * Removes duplicate commands and normalizes separators after conditional menu
 * construction/plugin contribution. First occurrence wins so built-in placement
 * remains deterministic.
 */
export function normalizeContextMenuItems(
  items: readonly ContextMenuItem[],
): ContextMenuItem[] {
  const result: ContextMenuItem[] = [];
  const commands = new Set<string>();
  let pendingSeparator = false;

  for (const item of items) {
    if (item.separator) {
      if (result.length) pendingSeparator = true;
      continue;
    }

    if (!item.commandId || !item.label) continue;
    if (commands.has(item.commandId)) continue;

    if (pendingSeparator && result.length) {
      result.push({ separator: true });
    }
    pendingSeparator = false;
    commands.add(item.commandId);
    result.push(item);
  }

  return result;
}