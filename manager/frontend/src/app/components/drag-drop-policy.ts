export type DragOperation = 'copy' | 'move';
export type DragDropValidity =
  | 'allowed'
  | 'same-location'
  | 'self-target'
  | 'descendant-target'
  | 'empty';

export interface DragEntryRef {
  path: string;
  type?: unknown;
}

export interface DragDropDecision {
  operation: DragOperation;
  validity: DragDropValidity;
  reason: string;
}

function normalize(value: string): string {
  return (value ?? '')
    .replace(/\\/g, '/')
    .replace(/\/+/g, '/')
    .replace(/\/$/, '')
    .toLocaleLowerCase();
}

export function sameResourcePath(left: string, right: string): boolean {
  return normalize(left) === normalize(right);
}

export function isDescendantResourcePath(candidate: string, ancestor: string): boolean {
  const child = normalize(candidate);
  const parent = normalize(ancestor);
  if (!child || !parent || child === parent) return false;
  return child.startsWith(`${parent}/`);
}

export function dragOperationFromModifiers(
  event: Pick<DragEvent, 'shiftKey' | 'ctrlKey'>,
): DragOperation {
  return event.shiftKey ? 'move' : 'copy';
}

export function collapseNestedDragEntries<T extends DragEntryRef>(
  entries: readonly T[],
): T[] {
  const result: T[] = [];

  for (const entry of entries) {
    if (!entry?.path) continue;
    if (result.some(existing => sameResourcePath(existing.path, entry.path))) continue;
    if (result.some(existing => isDescendantResourcePath(entry.path, existing.path))) continue;

    for (let index = result.length - 1; index >= 0; index -= 1) {
      if (isDescendantResourcePath(result[index].path, entry.path)) {
        result.splice(index, 1);
      }
    }
    result.push(entry);
  }

  return result;
}

export function decideInternalDrop(input: {
  entries: readonly DragEntryRef[];
  sourceDirectory: string;
  targetDirectory: string;
  operation: DragOperation;
}): DragDropDecision {
  const entries = collapseNestedDragEntries(input.entries);

  if (entries.length === 0 || !input.targetDirectory) {
    return { operation: input.operation, validity: 'empty', reason: 'Nothing to drop' };
  }

  if (sameResourcePath(input.sourceDirectory, input.targetDirectory)) {
    return { operation: input.operation, validity: 'same-location', reason: 'Already in this folder' };
  }

  for (const entry of entries) {
    if (sameResourcePath(entry.path, input.targetDirectory)) {
      return { operation: input.operation, validity: 'self-target', reason: 'Cannot drop an item onto itself' };
    }
    if (isDescendantResourcePath(input.targetDirectory, entry.path)) {
      return { operation: input.operation, validity: 'descendant-target', reason: 'Cannot drop a folder inside itself' };
    }
  }

  return {
    operation: input.operation,
    validity: 'allowed',
    reason: input.operation === 'move' ? 'Move here' : 'Copy here',
  };
}