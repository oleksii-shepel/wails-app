import { PaneDetailsComponent } from './pane-details.component';
import { FileType, type FileInfo } from '../shared/protocol';

function entry(path: string, size: number, modified = 1): FileInfo {
  return {
    name: path.split('/').pop() || path,
    path,
    type: FileType.FILE,
    size,
    modified,
    created: modified,
    accessed: modified,
    permissions: '',
    isHidden: false,
    isSymlink: false,
  };
}

describe('PaneDetailsComponent performance model', () => {
  it('caches formatted values per immutable entry object', () => {
    const component = new PaneDetailsComponent(
      { runOutsideAngular: (fn: () => void) => fn() } as any,
      { markForCheck() {} } as any,
    );
    const value = entry('/a.txt', 1024, 10);
    const size = (component as any).defaultColumns.find((c: any) => c.field === 'size');

    expect(component.getCellValue(value, size)).toBe('1 KB');
    const cache = (component as any).formattedValueCache.get(value);
    expect(cache.get('size').value).toBe('1 KB');
    expect(component.getCellValue(value, size)).toBe('1 KB');
  });

  it('applies independent visibility and order configuration', () => {
    const component = new PaneDetailsComponent(
      { runOutsideAngular: (fn: () => void) => fn() } as any,
      { markForCheck() {} } as any,
    );
    component.columnConfiguration = {
      columns: [
        { key: 'type', order: 0 },
        { key: 'name', order: 1 },
        { key: 'size', visible: false },
      ],
    };

    (component as any).refreshColumnState();
    expect(component.activeColumns.map(column => column.field)).toEqual([
      'type',
      'name',
      'modified',
    ]);
  });

  it('keeps Details virtualization on the shared VirtualCollection engine', () => {
    const source = PaneDetailsComponent.toString();
    expect(source).toBeTruthy();
    const component = new PaneDetailsComponent(
      { runOutsideAngular: (fn: () => void) => fn() } as any,
      { markForCheck() {} } as any,
    );
    expect((component as any).virtualCollection).toBeDefined();
  });
});
