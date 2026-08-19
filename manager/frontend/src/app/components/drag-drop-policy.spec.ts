import {
  collapseNestedDragEntries,
  decideInternalDrop,
  dragOperationFromModifiers,
  isDescendantResourcePath,
} from './drag-drop-policy';

describe('reassembly drag/drop policy', () => {
  it('uses copy by default and Shift for move', () => {
    expect(dragOperationFromModifiers({ shiftKey: false, ctrlKey: false } as DragEvent)).toBe('copy');
    expect(dragOperationFromModifiers({ shiftKey: true, ctrlKey: false } as DragEvent)).toBe('move');
    expect(dragOperationFromModifiers({ shiftKey: false, ctrlKey: true } as DragEvent)).toBe('copy');
  });

  it('recognizes normalized descendants', () => {
    expect(isDescendantResourcePath('C:\\Work\\Project\\src', 'C:/Work/Project')).toBeTrue();
    expect(isDescendantResourcePath('C:/Work/Project2', 'C:/Work/Project')).toBeFalse();
  });

  it('collapses descendants when their selected parent is transferred', () => {
    const entries = collapseNestedDragEntries([
      { path: 'C:/Work/Project' },
      { path: 'C:/Work/Project/src/a.ts' },
      { path: 'C:/Other/readme.txt' },
    ]);
    expect(entries.map(item => item.path)).toEqual([
      'C:/Work/Project',
      'C:/Other/readme.txt',
    ]);
  });

  it('collapses children encountered before their selected parent', () => {
    const entries = collapseNestedDragEntries([
      { path: 'C:/Work/Project/src/a.ts' },
      { path: 'C:/Work/Project' },
    ]);
    expect(entries.map(item => item.path)).toEqual(['C:/Work/Project']);
  });

  it('rejects same underlying source/destination folder across panes', () => {
    expect(decideInternalDrop({
      entries: [{ path: 'C:/Work/a.txt' }],
      sourceDirectory: 'C:/Work',
      targetDirectory: 'c:\\work\\',
      operation: 'copy',
    }).validity).toBe('same-location');
  });

  it('rejects self-target and descendant-target drops', () => {
    expect(decideInternalDrop({
      entries: [{ path: 'C:/Work/Project' }],
      sourceDirectory: 'C:/Work',
      targetDirectory: 'C:/Work/Project',
      operation: 'move',
    }).validity).toBe('self-target');

    expect(decideInternalDrop({
      entries: [{ path: 'C:/Work/Project' }],
      sourceDirectory: 'C:/Work',
      targetDirectory: 'C:/Work/Project/src',
      operation: 'copy',
    }).validity).toBe('descendant-target');
  });
});