import { readFileSync } from 'node:fs';

const appBindings = readFileSync(
  new URL('../../../../internal/app/bindings_file_basics.go', import.meta.url),
  'utf8',
);
const batch = readFileSync(
  new URL('../../../../internal/batch/service.go', import.meta.url),
  'utf8',
);
const windowsTrash = readFileSync(
  new URL('../../../../internal/platform/trash_windows.go', import.meta.url),
  'utf8',
);
const unixTrash = readFileSync(
  new URL('../../../../internal/platform/trash_unix.go', import.meta.url),
  'utf8',
);

describe('Recycle Bin transfer conflict resolution', () => {
  it('routes destination collisions through the shared batch conflict coordinator', () => {
    expect(batch).toContain('func (s *Service) CheckConflict(');
    expect(appBindings).toContain('a.State.Batch.CheckConflict(');
    expect(appBindings).toContain('var blanket *contracts.ConflictResolution');
  });

  it('supports Replace and Keep both through exact destination paths', () => {
    expect(appBindings).toContain('os.RemoveAll(finalDestination)');
    expect(appBindings).toContain('platform.CopyTrashEntryToPath(id, finalDestination)');
    expect(appBindings).toContain('platform.RestoreTrashEntryToPath(id, finalDestination)');
    expect(windowsTrash).toContain('func CopyTrashEntryToPath(');
    expect(unixTrash).toContain('func CopyTrashEntryToPath(');
  });

  it('does not turn Skip or Cancel into a raw restore error', () => {
    expect(appBindings).toContain('if skip {');
    expect(appBindings).toContain('if strings.Contains(strings.ToLower(conflictErr.Error()), "cancel")');
  });
});
