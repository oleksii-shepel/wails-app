import { describe, expect, it } from 'vitest';
import { buildBackgroundFileContextMenu, buildBuiltinFileContextMenu } from './file-context-menu.model';
import type { ActionAvailability } from '../services/action-availability';

const enabled: ActionAvailability = {
  createFolder: true,
  deleteSelected: true,
  copySelected: true,
  cutSelected: true,
  paste: true,
  rename: true,
  copyToOther: true,
  moveToOther: true,
};

describe('buildBuiltinFileContextMenu', () => {
  it('keeps read actions available while disabling unavailable mutations', () => {
    const items = buildBuiltinFileContextMenu({
      availability: { ...enabled, cutSelected: false, deleteSelected: false, rename: false, moveToOther: false },
      selectedCount: 1,
      isDirectory: false,
    });
    const byId = new Map(items.map(item => [item.id, item]));

    expect(byId.get('open')?.enabled).toBe(true);
    expect(byId.get('copy')?.enabled).toBe(true);
    expect(byId.get('cut')?.enabled).toBe(false);
    expect(byId.get('delete')?.enabled).toBe(false);
    expect(byId.get('rename')?.enabled).toBe(false);
    expect(byId.get('moveToOther')?.enabled).toBe(false);
  });

  it('offers directory navigation without file-only preview/external actions', () => {
    const items = buildBuiltinFileContextMenu({ availability: enabled, selectedCount: 1, isDirectory: true });
    const byId = new Map(items.map(item => [item.id, item]));

    expect(byId.get('open')?.enabled).toBe(true);
    expect(byId.get('openInNewTab')?.enabled).toBe(true);
    expect(byId.get('quickView')?.enabled).toBe(false);
    expect(byId.get('openWithExternal')?.enabled).toBe(false);
  });

  it('requires a single selection for rename', () => {
    const items = buildBuiltinFileContextMenu({ availability: enabled, selectedCount: 2, isDirectory: false });
    expect(items.find(item => item.id === 'rename')?.enabled).toBe(false);
  });

  it('builds background actions from create/paste availability', () => {
    const items = buildBackgroundFileContextMenu({ ...enabled, createFolder: false, paste: false });
    const byId = new Map(items.map(item => [item.id, item]));
    expect(byId.get('newFolder')?.enabled).toBe(false);
    expect(byId.get('paste')?.enabled).toBe(false);
    expect(byId.get('refresh')?.enabled).toBe(true);
  });

});
