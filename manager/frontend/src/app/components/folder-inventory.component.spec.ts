import '@angular/compiler';
import { signal } from '@angular/core';
import { FileType } from '../shared/protocol';
import { FolderInventoryComponent } from './folder-inventory.component';

function setup(listDirectory: (path: string) => Promise<{ entries: ReturnType<typeof file>[] }>) {
  const component = Object.create(FolderInventoryComponent.prototype) as FolderInventoryComponent;
  Object.assign(component, {
    api: { listDirectory },
    rootPath: '/original',
    request: 0,
    entries: [],
    visible: [],
    selected: new Set<string>(),
    query: '',
    kind: 'all',
    sort: 'name',
    loading: signal(false),
    error: signal(''),
    notice: signal(''),
  });
  return component;
}

function file(path: string) {
  return { path, name: path.split('/').pop()!, type: FileType.FILE, size: 1, modified: 0 };
}

describe('Folder Inventory refresh', () => {
  it('ignores a late listing after the folder changes', async () => {
    let finishOriginal!: (listing: { entries: ReturnType<typeof file>[] }) => void;
    const inventory = setup(path => path === '/original'
      ? new Promise(resolve => { finishOriginal = resolve; })
      : Promise.resolve({ entries: [file('/new/new.txt')] }));

    const earlier = inventory.load();
    inventory.rootPath = '/new';
    await inventory.load();
    finishOriginal({ entries: [file('/original/old.txt')] });
    await earlier;

    expect(inventory.visible.map(entry => entry.path)).toEqual(['/new/new.txt']);
    expect(inventory.loading()).toBe(false);
  });

  it('retains selection through an error and removes missing paths after retry', async () => {
    let fail = true;
    const inventory = setup(async () => {
      if (fail) throw new Error('Offline');
      return { entries: [file('/original/keep.txt')] };
    });
    inventory.selected.add('/original/keep.txt');
    inventory.selected.add('/original/missing.txt');

    await inventory.load();
    expect(inventory.error()).toBe('Offline');
    expect(inventory.selected.size).toBe(2);

    fail = false;
    await inventory.load();
    expect(inventory.error()).toBe('');
    expect([...inventory.selected]).toEqual(['/original/keep.txt']);
  });
});
