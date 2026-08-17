import { normalizeContextMenuItems } from './reassembly-context-menu-policy';

describe('reassembly context menu policy', () => {
  it('removes duplicate command contributions while keeping first placement', () => {
    const items = normalizeContextMenuItems([
      { commandId: 'open', label: 'Open' },
      { separator: true },
      { commandId: 'open', label: 'Open again' },
      { commandId: 'copy', label: 'Copy' },
    ]);

    expect(items).toEqual([
      { commandId: 'open', label: 'Open' },
      { separator: true },
      { commandId: 'copy', label: 'Copy' },
    ]);
  });

  it('removes leading, trailing and repeated separators', () => {
    const items = normalizeContextMenuItems([
      { separator: true },
      { separator: true },
      { commandId: 'open', label: 'Open' },
      { separator: true },
      { separator: true },
    ]);

    expect(items).toEqual([{ commandId: 'open', label: 'Open' }]);
  });

  it('drops malformed items that cannot be invoked', () => {
    const items = normalizeContextMenuItems([
      { label: 'Missing command' },
      { commandId: 'missing-label' },
      { commandId: 'valid', label: 'Valid' },
    ]);

    expect(items).toEqual([{ commandId: 'valid', label: 'Valid' }]);
  });
});
